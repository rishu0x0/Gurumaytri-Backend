package stats

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"school-management/backend/pkg/timezone"
)

var ErrClassNotFound = errors.New("class_not_found")

type TodayResult struct {
	ClassID      string `json:"class_id"`
	Date         string `json:"date"`
	PresentCount int    `json:"present_count"`
	AbsentCount  int    `json:"absent_count"`
	LeaveCount   int    `json:"leave_count"`
	TotalCount   int    `json:"total_count"`
	Submitted    bool   `json:"submitted"`
}

type StudentStat struct {
	StudentID         string  `json:"student_id"`
	FullName          string  `json:"full_name"`
	RollNumber        int     `json:"roll_number"`
	AttendancePercent float64 `json:"attendance_percentage"`
}

type MonthlyResult struct {
	ClassID           string        `json:"class_id"`
	Month             string        `json:"month"`
	DaysRecorded      int           `json:"days_recorded"`
	AveragePercentage float64       `json:"average_percentage"`
	BelowThreshold    []StudentStat `json:"below_threshold"`
}

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (s *Service) verifyClassOwnership(ctx context.Context, classID, teacherID string) error {
	var exists bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM classes WHERE id = $1 AND teacher_id = $2)`,
		classID, teacherID,
	).Scan(&exists)
	if err != nil || !exists {
		return ErrClassNotFound
	}
	return nil
}

// TodaySummary returns attendance counts for today's IST date.
// If no session exists for today, returns all zeros with submitted=false.
func (s *Service) TodaySummary(ctx context.Context, classID, teacherID string) (*TodayResult, error) {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}

	todayIST := timezone.TodayIST()
	dateStr := todayIST.Format("2006-01-02")

	result := &TodayResult{
		ClassID: classID,
		Date:    dateStr,
	}

	// Check if session exists for today
	var sessionID string
	err := s.db.QueryRow(ctx,
		`SELECT id FROM attendance_sessions WHERE class_id = $1 AND date = $2`,
		classID, dateStr,
	).Scan(&sessionID)
	if err != nil {
		// No session today — return zeros with submitted=false
		return result, nil
	}
	result.Submitted = true

	// Count by status
	rows, err := s.db.Query(ctx,
		`SELECT status, COUNT(*) FROM attendance_records WHERE session_id = $1 GROUP BY status`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("query today counts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan count: %w", err)
		}
		switch status {
		case "present":
			result.PresentCount = count
		case "absent":
			result.AbsentCount = count
		case "leave":
			result.LeaveCount = count
		}
	}
	result.TotalCount = result.PresentCount + result.AbsentCount + result.LeaveCount
	return result, nil
}

// MonthlySummary returns days recorded, average attendance %, and students below 75%
// for the given month (format: YYYY-MM).
func (s *Service) MonthlySummary(ctx context.Context, classID, teacherID, month string) (*MonthlyResult, error) {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}

	// Parse month to get date range
	startDate, err := time.Parse("2006-01", month)
	if err != nil {
		return nil, fmt.Errorf("invalid month format: %w", err)
	}
	// End date: first day of next month
	endDate := startDate.AddDate(0, 1, 0)

	result := &MonthlyResult{
		ClassID:        classID,
		Month:          month,
		BelowThreshold: []StudentStat{},
	}

	// Count distinct session dates for this month
	err = s.db.QueryRow(ctx,
		`SELECT COUNT(DISTINCT date) FROM attendance_sessions
		 WHERE class_id = $1 AND date >= $2 AND date < $3`,
		classID, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"),
	).Scan(&result.DaysRecorded)
	if err != nil || result.DaysRecorded == 0 {
		return result, nil
	}

	// Per-student attendance percentage for the month.
	// Removed students (is_active=false) show as "(Removed)" in full_name.
	// LEFT JOIN ensures students with zero attendance records are still included.
	rows, err := s.db.Query(ctx,
		`SELECT
		   st.id,
		   CASE WHEN st.is_active THEN st.full_name ELSE '(Removed)' END AS display_name,
		   st.roll_number,
		   COUNT(ar.id) FILTER (WHERE ar.status = 'present') AS present_days
		 FROM students st
		 LEFT JOIN attendance_records ar ON ar.student_id = st.id
		   AND ar.session_id IN (
		     SELECT id FROM attendance_sessions
		     WHERE class_id = $1 AND date >= $2 AND date < $3
		   )
		 WHERE st.class_id = $1
		 GROUP BY st.id, st.full_name, st.roll_number, st.is_active
		 ORDER BY st.roll_number`,
		classID, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"),
	)
	if err != nil {
		return nil, fmt.Errorf("query monthly stats: %w", err)
	}
	defer rows.Close()

	var totalPercent float64
	var studentCount int

	for rows.Next() {
		var st StudentStat
		var presentDays int
		if err := rows.Scan(&st.StudentID, &st.FullName, &st.RollNumber, &presentDays); err != nil {
			return nil, fmt.Errorf("scan student stat: %w", err)
		}

		// Attendance % = present days / total session days recorded * 100
		st.AttendancePercent = float64(presentDays) / float64(result.DaysRecorded) * 100

		totalPercent += st.AttendancePercent
		studentCount++

		if st.AttendancePercent < 75.0 {
			result.BelowThreshold = append(result.BelowThreshold, st)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	if studentCount > 0 {
		result.AveragePercentage = totalPercent / float64(studentCount)
	}

	return result, nil
}
