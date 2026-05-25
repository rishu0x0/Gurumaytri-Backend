package homework

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrClassNotFound      = errors.New("class_not_found")
	ErrAssignmentNotFound = errors.New("assignment_not_found")
)

type Assignment struct {
	ID          string  `json:"id"`
	ClassID     string  `json:"class_id"`
	TeacherID   string  `json:"teacher_id"`
	Subject     string  `json:"subject"`
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
	DueDate     string  `json:"due_date"`
	CreatedAt   string  `json:"created_at"`
}

type Input struct {
	Subject     string
	Title       string
	Description string
	DueDate     string
}

type Service struct{ db *pgxpool.Pool }

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

func (s *Service) verifyOwnership(ctx context.Context, classID, teacherID string) error {
	var exists bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM classes WHERE id=$1 AND teacher_id=$2)`,
		classID, teacherID,
	).Scan(&exists)
	if err != nil || !exists {
		return ErrClassNotFound
	}
	return nil
}

func (s *Service) List(ctx context.Context, classID, teacherID string) ([]Assignment, error) {
	if err := s.verifyOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, class_id, teacher_id, subject, title, description,
		       due_date::text, created_at::text
		FROM assignments WHERE class_id=$1
		ORDER BY due_date DESC, created_at DESC`,
		classID,
	)
	if err != nil {
		return nil, fmt.Errorf("list assignments: %w", err)
	}
	defer rows.Close()

	var out []Assignment
	for rows.Next() {
		var a Assignment
		if err := rows.Scan(&a.ID, &a.ClassID, &a.TeacherID, &a.Subject,
			&a.Title, &a.Description, &a.DueDate, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if out == nil {
		out = []Assignment{}
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, classID, teacherID string, in Input) (*Assignment, error) {
	if err := s.verifyOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}
	subject := strings.TrimSpace(in.Subject)
	title := strings.TrimSpace(in.Title)
	if subject == "" || title == "" || in.DueDate == "" {
		return nil, fmt.Errorf("subject, title, and due_date are required")
	}
	var a Assignment
	err := s.db.QueryRow(ctx, `
		INSERT INTO assignments (class_id, teacher_id, subject, title, description, due_date)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, class_id, teacher_id, subject, title, description,
		          due_date::text, created_at::text`,
		classID, teacherID, subject, title,
		nullableString(in.Description), in.DueDate,
	).Scan(&a.ID, &a.ClassID, &a.TeacherID, &a.Subject, &a.Title,
		&a.Description, &a.DueDate, &a.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create assignment: %w", err)
	}
	return &a, nil
}

func (s *Service) Update(ctx context.Context, classID, teacherID, id string, in Input) (*Assignment, error) {
	if err := s.verifyOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}
	subject := strings.TrimSpace(in.Subject)
	title := strings.TrimSpace(in.Title)
	if subject == "" || title == "" || in.DueDate == "" {
		return nil, fmt.Errorf("subject, title, and due_date are required")
	}
	var a Assignment
	err := s.db.QueryRow(ctx, `
		UPDATE assignments SET subject=$1, title=$2, description=$3, due_date=$4
		WHERE id=$5 AND class_id=$6
		RETURNING id, class_id, teacher_id, subject, title, description,
		          due_date::text, created_at::text`,
		subject, title, nullableString(in.Description), in.DueDate, id, classID,
	).Scan(&a.ID, &a.ClassID, &a.TeacherID, &a.Subject, &a.Title,
		&a.Description, &a.DueDate, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAssignmentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update assignment: %w", err)
	}
	return &a, nil
}

func (s *Service) Delete(ctx context.Context, classID, teacherID, id string) error {
	if err := s.verifyOwnership(ctx, classID, teacherID); err != nil {
		return err
	}
	tag, err := s.db.Exec(ctx,
		`DELETE FROM assignments WHERE id=$1 AND class_id=$2`, id, classID)
	if err != nil {
		return fmt.Errorf("delete assignment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrAssignmentNotFound
	}
	return nil
}

func nullableString(s string) interface{} {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}
