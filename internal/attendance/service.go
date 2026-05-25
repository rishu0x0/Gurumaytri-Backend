package attendance

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"school-management/backend/pkg/timezone"
)

var (
	ErrClassNotFound    = errors.New("class_not_found")
	ErrSessionNotFound  = errors.New("session_not_found")
	ErrDuplicateSession = errors.New("duplicate_session")
	ErrAttendanceLocked = errors.New("attendance_locked")
	ErrInvalidStatus    = errors.New("invalid_status")
)

var validStatuses = map[string]bool{"present": true, "absent": true, "leave": true}

type Record struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	StudentID string `json:"student_id"`
	Status    string `json:"status"`
	MarkedAt  string `json:"marked_at"`
}

type Session struct {
	ID          string   `json:"id"`
	ClassID     string   `json:"class_id"`
	TeacherID   string   `json:"teacher_id"`
	Date        string   `json:"date"`
	SubmittedAt *string  `json:"submitted_at,omitempty"`
	IsLocked    bool     `json:"is_locked"`
	Records     []Record `json:"records"`
}

type BatchRecord struct {
	StudentID string `json:"student_id"`
	Status    string `json:"status"`
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

// SubmitBatch saves an attendance session and all records in a single transaction.
// Returns ErrDuplicateSession if attendance already exists for this class+date.
func (s *Service) SubmitBatch(ctx context.Context, classID, teacherID string, date time.Time, records []BatchRecord) (*Session, error) {
	log.Printf("[attendance] SubmitBatch start classID=%s teacherID=%s date=%s records=%d", classID, teacherID, date.Format("2006-01-02"), len(records))

	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		log.Printf("[attendance] SubmitBatch verifyOwnership failed: %v", err)
		return nil, err
	}

	for _, r := range records {
		if !validStatuses[r.Status] {
			log.Printf("[attendance] SubmitBatch invalid status %q for student %s", r.Status, r.StudentID)
			return nil, ErrInvalidStatus
		}
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		log.Printf("[attendance] SubmitBatch begin tx failed: %v", err)
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var sessionID string
	dateStr := date.Format("2006-01-02")
	err = tx.QueryRow(ctx,
		`INSERT INTO attendance_sessions (class_id, teacher_id, date, submitted_at)
		 VALUES ($1, $2, $3, NOW())
		 RETURNING id`,
		classID, teacherID, dateStr,
	).Scan(&sessionID)
	if err != nil {
		log.Printf("[attendance] SubmitBatch insert session failed: %v", err)
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrDuplicateSession
		}
		return nil, fmt.Errorf("create session: %w", err)
	}
	log.Printf("[attendance] SubmitBatch session inserted id=%s", sessionID)

	for _, r := range records {
		_, err = tx.Exec(ctx,
			`INSERT INTO attendance_records (session_id, student_id, status)
			 VALUES ($1, $2, $3)`,
			sessionID, r.StudentID, r.Status,
		)
		if err != nil {
			log.Printf("[attendance] SubmitBatch insert record failed student=%s status=%s: %v", r.StudentID, r.Status, err)
			return nil, fmt.Errorf("insert record for student %s: %w", r.StudentID, err)
		}
	}

	if err = tx.Commit(ctx); err != nil {
		log.Printf("[attendance] SubmitBatch commit failed: %v", err)
		return nil, fmt.Errorf("commit transaction: %w", err)
	}
	log.Printf("[attendance] SubmitBatch committed, fetching session")

	return s.GetByDate(ctx, classID, teacherID, date)
}

// GetByDate returns the attendance session and all records for a given date.
// Returns nil, ErrSessionNotFound when no session exists — handler returns {session: null} gracefully.
func (s *Service) GetByDate(ctx context.Context, classID, teacherID string, date time.Time) (*Session, error) {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}

	dateStr := date.Format("2006-01-02")
	log.Printf("[attendance] GetByDate classID=%s date=%s", classID, dateStr)

	var session Session
	err := s.db.QueryRow(ctx,
		`SELECT id, class_id, teacher_id, date::text, submitted_at::text, is_locked
		 FROM attendance_sessions
		 WHERE class_id = $1 AND date = $2`,
		classID, dateStr,
	).Scan(&session.ID, &session.ClassID, &session.TeacherID, &session.Date, &session.SubmittedAt, &session.IsLocked)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("[attendance] GetByDate no session found")
			return nil, ErrSessionNotFound
		}
		log.Printf("[attendance] GetByDate scan session failed: %v", err)
		return nil, fmt.Errorf("query session: %w", err)
	}
	log.Printf("[attendance] GetByDate session found id=%s", session.ID)

	rows, err := s.db.Query(ctx,
		`SELECT id, session_id, student_id, status, marked_at::text
		 FROM attendance_records
		 WHERE session_id = $1
		 ORDER BY student_id`,
		session.ID,
	)
	if err != nil {
		log.Printf("[attendance] GetByDate query records failed: %v", err)
		return nil, fmt.Errorf("query records: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var rec Record
		if err := rows.Scan(&rec.ID, &rec.SessionID, &rec.StudentID, &rec.Status, &rec.MarkedAt); err != nil {
			log.Printf("[attendance] GetByDate scan record failed: %v", err)
			return nil, fmt.Errorf("scan record: %w", err)
		}
		session.Records = append(session.Records, rec)
	}
	if rows.Err() != nil {
		log.Printf("[attendance] GetByDate rows error: %v", rows.Err())
		return nil, fmt.Errorf("iterate records: %w", rows.Err())
	}
	if session.Records == nil {
		session.Records = []Record{}
	}
	log.Printf("[attendance] GetByDate returning %d records", len(session.Records))
	return &session, nil
}

// EditRecords updates status for one or more records in a session.
// Returns ErrAttendanceLocked if past midnight IST on the session date.
func (s *Service) EditRecords(ctx context.Context, classID, teacherID, sessionID string, updates []BatchRecord) (*Session, error) {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}

	var sessionClassID, sessionDateStr string
	err := s.db.QueryRow(ctx,
		`SELECT class_id, date::text FROM attendance_sessions WHERE id = $1`,
		sessionID,
	).Scan(&sessionClassID, &sessionDateStr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("query session: %w", err)
	}
	if sessionClassID != classID {
		return nil, ErrSessionNotFound
	}

	sessionDate, err := time.Parse("2006-01-02", sessionDateStr)
	if err != nil {
		return nil, fmt.Errorf("parse session date: %w", err)
	}

	if timezone.IsLocked(sessionDate, time.Now()) {
		return nil, ErrAttendanceLocked
	}

	for _, u := range updates {
		if !validStatuses[u.Status] {
			return nil, ErrInvalidStatus
		}
		_, err = s.db.Exec(ctx,
			`UPDATE attendance_records
			 SET status = $1, updated_at = NOW()
			 WHERE session_id = $2 AND student_id = $3`,
			u.Status, sessionID, u.StudentID,
		)
		if err != nil {
			return nil, fmt.Errorf("update record for student %s: %w", u.StudentID, err)
		}
	}

	return s.GetByDate(ctx, classID, teacherID, sessionDate)
}
