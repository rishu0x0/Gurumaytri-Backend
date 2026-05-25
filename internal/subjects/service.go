package subjects

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrClassNotFound   = errors.New("class_not_found")
	ErrSubjectNotFound = errors.New("subject_not_found")
	ErrDuplicateName   = errors.New("duplicate_subject_name")
)

type Subject struct {
	ID           string `json:"id"`
	ClassID      string `json:"class_id"`
	Name         string `json:"name"`
	MaxMarks     int    `json:"max_marks"`
	DisplayOrder int    `json:"display_order"`
	CreatedAt    string `json:"created_at"`
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

func (s *Service) List(ctx context.Context, classID, teacherID string) ([]Subject, error) {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}

	rows, err := s.db.Query(ctx,
		`SELECT id, class_id, name, max_marks, display_order, created_at::text
		 FROM subjects
		 WHERE class_id = $1
		 ORDER BY display_order, name`,
		classID,
	)
	if err != nil {
		return nil, fmt.Errorf("query subjects: %w", err)
	}
	defer rows.Close()

	var out []Subject
	for rows.Next() {
		var sub Subject
		if err := rows.Scan(&sub.ID, &sub.ClassID, &sub.Name, &sub.MaxMarks, &sub.DisplayOrder, &sub.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan subject: %w", err)
		}
		out = append(out, sub)
	}
	if out == nil {
		out = []Subject{}
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, classID, teacherID, name string, maxMarks, displayOrder int) (*Subject, error) {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("subject name is required")
	}
	if maxMarks <= 0 {
		maxMarks = 100
	}

	var sub Subject
	err := s.db.QueryRow(ctx,
		`INSERT INTO subjects (class_id, teacher_id, name, max_marks, display_order)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, class_id, name, max_marks, display_order, created_at::text`,
		classID, teacherID, name, maxMarks, displayOrder,
	).Scan(&sub.ID, &sub.ClassID, &sub.Name, &sub.MaxMarks, &sub.DisplayOrder, &sub.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrDuplicateName
		}
		return nil, fmt.Errorf("create subject: %w", err)
	}
	return &sub, nil
}

func (s *Service) Update(ctx context.Context, classID, teacherID, subjectID, name string, maxMarks int) (*Subject, error) {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("subject name is required")
	}
	if maxMarks <= 0 {
		maxMarks = 100
	}

	var sub Subject
	err := s.db.QueryRow(ctx,
		`UPDATE subjects SET name=$1, max_marks=$2
		 WHERE id=$3 AND class_id=$4
		 RETURNING id, class_id, name, max_marks, display_order, created_at::text`,
		name, maxMarks, subjectID, classID,
	).Scan(&sub.ID, &sub.ClassID, &sub.Name, &sub.MaxMarks, &sub.DisplayOrder, &sub.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSubjectNotFound
		}
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrDuplicateName
		}
		return nil, fmt.Errorf("update subject: %w", err)
	}
	return &sub, nil
}

func (s *Service) Delete(ctx context.Context, classID, teacherID, subjectID string) error {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return err
	}
	tag, err := s.db.Exec(ctx,
		`DELETE FROM subjects WHERE id=$1 AND class_id=$2`,
		subjectID, classID,
	)
	if err != nil {
		return fmt.Errorf("delete subject: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSubjectNotFound
	}
	return nil
}
