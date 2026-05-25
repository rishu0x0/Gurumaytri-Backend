package admission

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("inquiry_not_found")

type Inquiry struct {
	ID             string  `json:"id"`
	TeacherID      string  `json:"teacher_id"`
	StudentName    string  `json:"student_name"`
	DateOfBirth    *string `json:"date_of_birth,omitempty"`
	ParentName     *string `json:"parent_name,omitempty"`
	ContactNumber  *string `json:"contact_number,omitempty"`
	Email          *string `json:"email,omitempty"`
	PreviousSchool *string `json:"previous_school,omitempty"`
	ClassApplied   *string `json:"class_applied,omitempty"`
	AcademicYear   *string `json:"academic_year,omitempty"`
	Status         string  `json:"status"`
	Notes          *string `json:"notes,omitempty"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

type CreateInput struct {
	StudentName    string
	DateOfBirth    string
	ParentName     string
	ContactNumber  string
	Email          string
	PreviousSchool string
	ClassApplied   string
	AcademicYear   string
	Notes          string
}

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

const scanCols = `id, teacher_id, student_name, date_of_birth::text,
	parent_name, contact_number, email, previous_school,
	class_applied, academic_year, status, notes, created_at::text, updated_at::text`

func scanInquiry(row pgx.Row) (Inquiry, error) {
	var q Inquiry
	err := row.Scan(
		&q.ID, &q.TeacherID, &q.StudentName, &q.DateOfBirth,
		&q.ParentName, &q.ContactNumber, &q.Email, &q.PreviousSchool,
		&q.ClassApplied, &q.AcademicYear, &q.Status, &q.Notes,
		&q.CreatedAt, &q.UpdatedAt,
	)
	return q, err
}

func (s *Service) List(ctx context.Context, teacherID, status string) ([]Inquiry, error) {
	query := `SELECT ` + scanCols + ` FROM admission_inquiries WHERE teacher_id=$1`
	args := []any{teacherID}
	if status != "" && status != "all" {
		query += ` AND status=$2`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list inquiries: %w", err)
	}
	defer rows.Close()

	var out []Inquiry
	for rows.Next() {
		q, err := scanInquiry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	if out == nil {
		out = []Inquiry{}
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, teacherID string, in CreateInput) (*Inquiry, error) {
	name := strings.TrimSpace(in.StudentName)
	if name == "" {
		return nil, fmt.Errorf("student_name is required")
	}
	var q Inquiry
	row := s.db.QueryRow(ctx, `
		INSERT INTO admission_inquiries
		  (teacher_id, student_name, date_of_birth, parent_name, contact_number,
		   email, previous_school, class_applied, academic_year, notes)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING `+scanCols,
		teacherID, name,
		nullableString(in.DateOfBirth), nullableString(in.ParentName),
		nullableString(in.ContactNumber), nullableString(in.Email),
		nullableString(in.PreviousSchool), nullableString(in.ClassApplied),
		nullableString(in.AcademicYear), nullableString(in.Notes),
	)
	result, err := scanInquiry(row)
	if err != nil {
		return nil, fmt.Errorf("create inquiry: %w", err)
	}
	q = result
	return &q, nil
}

func (s *Service) Update(ctx context.Context, teacherID, id string, in CreateInput) (*Inquiry, error) {
	name := strings.TrimSpace(in.StudentName)
	if name == "" {
		return nil, fmt.Errorf("student_name is required")
	}
	row := s.db.QueryRow(ctx, `
		UPDATE admission_inquiries SET
		  student_name=$1, date_of_birth=$2, parent_name=$3, contact_number=$4,
		  email=$5, previous_school=$6, class_applied=$7, academic_year=$8,
		  notes=$9, updated_at=NOW()
		WHERE id=$10 AND teacher_id=$11
		RETURNING `+scanCols,
		name,
		nullableString(in.DateOfBirth), nullableString(in.ParentName),
		nullableString(in.ContactNumber), nullableString(in.Email),
		nullableString(in.PreviousSchool), nullableString(in.ClassApplied),
		nullableString(in.AcademicYear), nullableString(in.Notes),
		id, teacherID,
	)
	q, err := scanInquiry(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update inquiry: %w", err)
	}
	return &q, nil
}

func (s *Service) UpdateStatus(ctx context.Context, teacherID, id, status string) (*Inquiry, error) {
	if status != "pending" && status != "approved" && status != "rejected" {
		return nil, fmt.Errorf("invalid status: %s", status)
	}
	row := s.db.QueryRow(ctx, `
		UPDATE admission_inquiries SET status=$1, updated_at=NOW()
		WHERE id=$2 AND teacher_id=$3
		RETURNING `+scanCols,
		status, id, teacherID,
	)
	q, err := scanInquiry(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update status: %w", err)
	}
	return &q, nil
}

func (s *Service) Delete(ctx context.Context, teacherID, id string) error {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM admission_inquiries WHERE id=$1 AND teacher_id=$2`,
		id, teacherID,
	)
	if err != nil {
		return fmt.Errorf("delete inquiry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
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
