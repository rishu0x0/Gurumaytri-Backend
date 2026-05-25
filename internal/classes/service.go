package classes

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound        = errors.New("class_not_found")
	ErrDuplicateName   = errors.New("duplicate_class_name")
	ErrConfirmRequired = errors.New("confirm_required")
)

type Class struct {
	ID        string  `json:"id"`
	TeacherID string  `json:"teacher_id"`
	Name      string  `json:"name"`
	Section   *string `json:"section,omitempty"`
	Subject   *string `json:"subject,omitempty"`
	CreatedAt string  `json:"created_at"`
}

type DeleteWarning struct {
	StudentCount int    `json:"student_count"`
	Message      string `json:"message"`
}

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (s *Service) List(ctx context.Context, teacherID string) ([]Class, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, teacher_id, name, section, subject, created_at::text
		 FROM classes
		 WHERE teacher_id = $1
		 ORDER BY name`,
		teacherID,
	)
	if err != nil {
		return nil, fmt.Errorf("query classes: %w", err)
	}
	defer rows.Close()

	var out []Class
	for rows.Next() {
		var c Class
		if err := rows.Scan(&c.ID, &c.TeacherID, &c.Name, &c.Section, &c.Subject, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan class: %w", err)
		}
		out = append(out, c)
	}
	if out == nil {
		out = []Class{}
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, teacherID, name, section, subject string) (*Class, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("class name is required")
	}

	var c Class
	err := s.db.QueryRow(ctx,
		`INSERT INTO classes (teacher_id, name, section, subject)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, teacher_id, name, section, subject, created_at::text`,
		teacherID,
		name,
		nullableString(section),
		nullableString(subject),
	).Scan(&c.ID, &c.TeacherID, &c.Name, &c.Section, &c.Subject, &c.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrDuplicateName
		}
		return nil, fmt.Errorf("create class: %w", err)
	}
	return &c, nil
}

func (s *Service) Update(ctx context.Context, teacherID, classID, name string) (*Class, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("class name is required")
	}

	var c Class
	err := s.db.QueryRow(ctx,
		`UPDATE classes SET name = $1
		 WHERE id = $2 AND teacher_id = $3
		 RETURNING id, teacher_id, name, section, subject, created_at::text`,
		name, classID, teacherID,
	).Scan(&c.ID, &c.TeacherID, &c.Name, &c.Section, &c.Subject, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrDuplicateName
		}
		return nil, fmt.Errorf("update class: %w", err)
	}
	return &c, nil
}

// Delete removes a class and all its data. confirm must be true.
// If confirm is false, returns ErrConfirmRequired and a warning with student count.
func (s *Service) Delete(ctx context.Context, teacherID, classID string, confirm bool) (*DeleteWarning, error) {
	// First verify ownership
	var exists bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM classes WHERE id = $1 AND teacher_id = $2)`,
		classID, teacherID,
	).Scan(&exists)
	if err != nil || !exists {
		return nil, ErrNotFound
	}

	// Count students
	var studentCount int
	_ = s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM students WHERE class_id = $1`,
		classID,
	).Scan(&studentCount)

	if !confirm {
		return &DeleteWarning{
			StudentCount: studentCount,
			Message: fmt.Sprintf(
				"Deleting this class will permanently remove %d student(s) and all attendance records. Pass confirm=true to proceed.",
				studentCount,
			),
		}, ErrConfirmRequired
	}

	_, err = s.db.Exec(ctx,
		`DELETE FROM classes WHERE id = $1 AND teacher_id = $2`,
		classID, teacherID,
	)
	if err != nil {
		return nil, fmt.Errorf("delete class: %w", err)
	}
	return nil, nil
}

func nullableString(s string) interface{} {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}
