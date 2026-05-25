package reports

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// ReportRow is the DB view returned to clients.
type ReportRow struct {
	ID          string
	ClassID     string
	Month       string
	Format      string
	FileName    *string
	Status      string
	ErrorMsg    *string
	StoragePath string
	SignedURL   *string
	CreatedAt   string
}

// ReportService orchestrates: fetch data → generate → upload → save DB row.
type ReportService struct {
	pool    *pgxpool.Pool
	storage *StorageClient
}

func NewService(pool *pgxpool.Pool, storage *StorageClient) *ReportService {
	return &ReportService{pool: pool, storage: storage}
}

// CreateReportRow inserts a pending report row and returns its ID.
func (s *ReportService) CreateReportRow(ctx context.Context, teacherID, classID, month, format string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO reports (teacher_id, class_id, month, format)
		VALUES ($1, $2, $3, $4)
		RETURNING id`,
		teacherID, classID, month, format,
	).Scan(&id)
	return id, err
}

// GenerateReport generates a PDF or Excel report for a class+month, uploads to Storage, and updates the DB row.
func (s *ReportService) GenerateReport(ctx context.Context, reportID, classID, teacherID, month, format string) error {
	if err := s.updateStatus(ctx, reportID, "processing", ""); err != nil {
		return err
	}

	data, err := s.fetchReportData(ctx, classID, month)
	if err != nil {
		_ = s.updateStatus(ctx, reportID, "error", err.Error())
		return err
	}

	var fileBytes []byte
	var contentType, ext string
	switch format {
	case "pdf":
		fileBytes, err = GeneratePDF(data)
		contentType = "application/pdf"
		ext = "pdf"
	case "excel":
		fileBytes, err = GenerateExcel(data)
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		ext = "xlsx"
	default:
		err = fmt.Errorf("unknown format: %s", format)
	}
	if err != nil {
		_ = s.updateStatus(ctx, reportID, "error", err.Error())
		return err
	}

	safeName := unsafeChars.ReplaceAllString(strings.ReplaceAll(data.ClassName, " ", "_"), "")
	fileName := fmt.Sprintf("%s_Attendance_%s.%s", safeName, month, ext)
	storagePath := fmt.Sprintf("%s/%s/%s", classID, month, fileName)

	if err := s.storage.Upload(storagePath, fileBytes, contentType); err != nil {
		_ = s.updateStatus(ctx, reportID, "error", err.Error())
		return err
	}

	signedURL, expiresAt, err := s.storage.SignedURL(storagePath, 7*24*3600)
	if err != nil {
		_ = s.updateStatus(ctx, reportID, "error", err.Error())
		return err
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE reports
		SET status='ready', storage_path=$1, file_name=$2,
		    signed_url=$3, signed_url_expires_at=$4
		WHERE id=$5`,
		storagePath, fileName, signedURL, expiresAt, reportID)
	return err
}

// ListReports returns all non-expired reports for a teacher, refreshing signed URLs as needed.
func (s *ReportService) ListReports(ctx context.Context, teacherID string) ([]ReportRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, class_id, month, format, file_name, status, error_msg,
		       COALESCE(storage_path, ''), signed_url, signed_url_expires_at, created_at::text
		FROM reports
		WHERE teacher_id=$1 AND expires_at > NOW()
		ORDER BY created_at DESC
		LIMIT 100`,
		teacherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []ReportRow
	for rows.Next() {
		var r ReportRow
		var signedURLExpires *time.Time
		if err := rows.Scan(
			&r.ID, &r.ClassID, &r.Month, &r.Format, &r.FileName,
			&r.Status, &r.ErrorMsg, &r.StoragePath, &r.SignedURL,
			&signedURLExpires, &r.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan report row: %w", err)
		}
		// Refresh if expired, missing, or URL is malformed (missing /storage/v1/)
		urlBad := r.SignedURL == nil || !strings.Contains(*r.SignedURL, "/storage/v1/")
		expBad := signedURLExpires == nil || time.Now().After(*signedURLExpires)
		if r.Status == "ready" && r.StoragePath != "" && (urlBad || expBad) {
			url, exp, err := s.storage.SignedURL(r.StoragePath, 7*24*3600)
			if err == nil {
				r.SignedURL = &url
				_, _ = s.pool.Exec(ctx, `UPDATE reports SET signed_url=$1, signed_url_expires_at=$2 WHERE id=$3`,
					url, exp, r.ID)
			}
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// DeleteExpired removes expired report rows and their storage objects.
func (s *ReportService) DeleteExpired(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `
		DELETE FROM reports WHERE expires_at < NOW()
		RETURNING storage_path`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var path *string
		_ = rows.Scan(&path)
		if path != nil && *path != "" {
			_ = s.storage.deleteObject(*path)
		}
	}
	return rows.Err()
}

func (s *ReportService) updateStatus(ctx context.Context, reportID, status, errMsg string) error {
	_, err := s.pool.Exec(ctx, `UPDATE reports SET status=$1, error_msg=NULLIF($2,'') WHERE id=$3`,
		status, errMsg, reportID)
	return err
}

// fetchReportData queries attendance records for classID/month and builds ReportData.
func (s *ReportService) fetchReportData(ctx context.Context, classID, month string) (ReportData, error) {
	var className string
	err := s.pool.QueryRow(ctx, `SELECT name FROM classes WHERE id=$1`, classID).Scan(&className)
	if err != nil {
		return ReportData{}, fmt.Errorf("fetch class: %w", err)
	}

	studentRows, err := s.pool.Query(ctx, `
		SELECT id, full_name, roll_number FROM students
		WHERE class_id=$1
		ORDER BY roll_number`, classID)
	if err != nil {
		return ReportData{}, fmt.Errorf("fetch students: %w", err)
	}

	type stuMeta struct {
		id   string
		name string
		roll int
	}
	var stuMetas []stuMeta
	stuIndex := map[string]int{}
	for studentRows.Next() {
		var m stuMeta
		if err := studentRows.Scan(&m.id, &m.name, &m.roll); err != nil {
			studentRows.Close()
			return ReportData{}, err
		}
		stuIndex[m.id] = len(stuMetas)
		stuMetas = append(stuMetas, m)
	}
	studentRows.Close()

	recRows, err := s.pool.Query(ctx, `
		SELECT ar.student_id, ar.status, DATE_PART('day', s.date::date)::int AS day
		FROM attendance_records ar
		JOIN attendance_sessions s ON s.id = ar.session_id
		WHERE s.class_id=$1 AND TO_CHAR(s.date, 'YYYY-MM') = $2`,
		classID, month)
	if err != nil {
		return ReportData{}, fmt.Errorf("fetch records: %w", err)
	}
	defer recRows.Close()

	daySet := map[int]struct{}{}
	studentRecords := make([]map[int]string, len(stuMetas))
	for i := range studentRecords {
		studentRecords[i] = map[int]string{}
	}

	for recRows.Next() {
		var studentID, status string
		var day int
		if err := recRows.Scan(&studentID, &status, &day); err != nil {
			return ReportData{}, err
		}
		daySet[day] = struct{}{}
		if idx, ok := stuIndex[studentID]; ok {
			studentRecords[idx][day] = AttendanceLabel(status)
		}
	}

	days := make([]int, 0, len(daySet))
	for d := range daySet {
		days = append(days, d)
	}
	sortInts(days)

	students := make([]StudentRow, len(stuMetas))
	for i, meta := range stuMetas {
		recs := studentRecords[i]
		var present, absent, leave int
		for _, label := range recs {
			switch label {
			case "P":
				present++
			case "A":
				absent++
			case "L":
				leave++
			}
		}
		students[i] = StudentRow{
			Name:       meta.name,
			RollNumber: meta.roll,
			Records:    recs,
			Present:    present,
			Absent:     absent,
			Leave:      leave,
			Total:      present + absent + leave,
		}
	}

	return ReportData{
		ClassName:   className,
		Month:       month,
		GeneratedAt: time.Now(),
		Days:        days,
		Students:    students,
	}, nil
}

func sortInts(s []int) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
