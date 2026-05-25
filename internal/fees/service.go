package fees

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrClassNotFound     = errors.New("class_not_found")
	ErrStructureNotFound = errors.New("fee_structure_not_found")
)

type FeeStructure struct {
	ID           string  `json:"id"`
	ClassID      string  `json:"class_id"`
	Title        string  `json:"title"`
	Amount       float64 `json:"amount"`
	DueDate      *string `json:"due_date,omitempty"`
	AcademicYear *string `json:"academic_year,omitempty"`
	CreatedAt    string  `json:"created_at"`
	// Aggregates populated by List
	TotalStudents int     `json:"total_students"`
	PaidCount     int     `json:"paid_count"`
	PartialCount  int     `json:"partial_count"`
	TotalCollected float64 `json:"total_collected"`
}

type FeeRecord struct {
	StudentID   string  `json:"student_id"`
	FullName    string  `json:"full_name"`
	RollNumber  int     `json:"roll_number"`
	AmountPaid  float64 `json:"amount_paid"`
	PaidAt      *string `json:"paid_at,omitempty"`
	PaymentMode *string `json:"payment_mode,omitempty"`
	Notes       *string `json:"notes,omitempty"`
	Status      string  `json:"status"` // paid | partial | pending
}

type PaymentInput struct {
	StudentID   string  `json:"student_id"`
	AmountPaid  float64 `json:"amount_paid"`
	PaidAt      string  `json:"paid_at"`
	PaymentMode string  `json:"payment_mode"`
	Notes       string  `json:"notes"`
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
		`SELECT EXISTS(SELECT 1 FROM classes WHERE id=$1 AND teacher_id=$2)`,
		classID, teacherID,
	).Scan(&exists)
	if err != nil || !exists {
		return ErrClassNotFound
	}
	return nil
}

func (s *Service) ListStructures(ctx context.Context, classID, teacherID string) ([]FeeStructure, error) {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}

	rows, err := s.db.Query(ctx, `
		SELECT
			fs.id, fs.class_id, fs.title, fs.amount::float8,
			fs.due_date::text, fs.academic_year, fs.created_at::text,
			COUNT(DISTINCT s.id) AS total_students,
			COUNT(DISTINCT CASE WHEN fr.amount_paid >= fs.amount THEN fr.student_id END) AS paid_count,
			COUNT(DISTINCT CASE WHEN fr.amount_paid > 0 AND fr.amount_paid < fs.amount THEN fr.student_id END) AS partial_count,
			COALESCE(SUM(fr.amount_paid), 0)::float8 AS total_collected
		FROM fee_structures fs
		LEFT JOIN students s ON s.class_id = fs.class_id AND s.is_active = true
		LEFT JOIN fee_records fr ON fr.fee_structure_id = fs.id AND fr.student_id = s.id
		WHERE fs.class_id=$1
		GROUP BY fs.id
		ORDER BY fs.created_at DESC`,
		classID,
	)
	if err != nil {
		return nil, fmt.Errorf("query fee structures: %w", err)
	}
	defer rows.Close()

	var out []FeeStructure
	for rows.Next() {
		var f FeeStructure
		if err := rows.Scan(
			&f.ID, &f.ClassID, &f.Title, &f.Amount,
			&f.DueDate, &f.AcademicYear, &f.CreatedAt,
			&f.TotalStudents, &f.PaidCount, &f.PartialCount, &f.TotalCollected,
		); err != nil {
			return nil, fmt.Errorf("scan fee structure: %w", err)
		}
		out = append(out, f)
	}
	if out == nil {
		out = []FeeStructure{}
	}
	return out, nil
}

func (s *Service) CreateStructure(ctx context.Context, classID, teacherID, title string, amount float64, dueDate, academicYear string) (*FeeStructure, error) {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	var f FeeStructure
	err := s.db.QueryRow(ctx, `
		INSERT INTO fee_structures (class_id, teacher_id, title, amount, due_date, academic_year)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, class_id, title, amount::float8, due_date::text, academic_year, created_at::text`,
		classID, teacherID, title, amount, nullableString(dueDate), nullableString(academicYear),
	).Scan(&f.ID, &f.ClassID, &f.Title, &f.Amount, &f.DueDate, &f.AcademicYear, &f.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create fee structure: %w", err)
	}
	return &f, nil
}

func (s *Service) UpdateStructure(ctx context.Context, classID, teacherID, structureID, title string, amount float64, dueDate, academicYear string) (*FeeStructure, error) {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	var f FeeStructure
	err := s.db.QueryRow(ctx, `
		UPDATE fee_structures SET title=$1, amount=$2, due_date=$3, academic_year=$4
		WHERE id=$5 AND class_id=$6
		RETURNING id, class_id, title, amount::float8, due_date::text, academic_year, created_at::text`,
		title, amount, nullableString(dueDate), nullableString(academicYear), structureID, classID,
	).Scan(&f.ID, &f.ClassID, &f.Title, &f.Amount, &f.DueDate, &f.AcademicYear, &f.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrStructureNotFound
		}
		return nil, fmt.Errorf("update fee structure: %w", err)
	}
	return &f, nil
}

func (s *Service) DeleteStructure(ctx context.Context, classID, teacherID, structureID string) error {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return err
	}
	tag, err := s.db.Exec(ctx,
		`DELETE FROM fee_structures WHERE id=$1 AND class_id=$2`,
		structureID, classID,
	)
	if err != nil {
		return fmt.Errorf("delete fee structure: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrStructureNotFound
	}
	return nil
}

// GetPayments returns all active students with their payment status for a fee structure.
func (s *Service) GetPayments(ctx context.Context, classID, teacherID, structureID string) ([]FeeRecord, float64, error) {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return nil, 0, err
	}

	var structureAmount float64
	err := s.db.QueryRow(ctx,
		`SELECT amount::float8 FROM fee_structures WHERE id=$1 AND class_id=$2`,
		structureID, classID,
	).Scan(&structureAmount)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, ErrStructureNotFound
	}
	if err != nil {
		return nil, 0, fmt.Errorf("fetch structure: %w", err)
	}

	rows, err := s.db.Query(ctx, `
		SELECT
			s.id, s.full_name, s.roll_number,
			COALESCE(fr.amount_paid, 0)::float8,
			fr.paid_at::text,
			fr.payment_mode,
			fr.notes
		FROM students s
		LEFT JOIN fee_records fr ON fr.fee_structure_id=$1 AND fr.student_id=s.id
		WHERE s.class_id=$2 AND s.is_active=true
		ORDER BY s.roll_number`,
		structureID, classID,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("query payments: %w", err)
	}
	defer rows.Close()

	var out []FeeRecord
	for rows.Next() {
		var r FeeRecord
		if err := rows.Scan(
			&r.StudentID, &r.FullName, &r.RollNumber,
			&r.AmountPaid, &r.PaidAt, &r.PaymentMode, &r.Notes,
		); err != nil {
			return nil, 0, fmt.Errorf("scan fee record: %w", err)
		}
		switch {
		case r.AmountPaid >= structureAmount:
			r.Status = "paid"
		case r.AmountPaid > 0:
			r.Status = "partial"
		default:
			r.Status = "pending"
		}
		out = append(out, r)
	}
	if out == nil {
		out = []FeeRecord{}
	}
	return out, structureAmount, nil
}

// RecordPayment upserts a single student's payment record.
func (s *Service) RecordPayment(ctx context.Context, classID, teacherID, structureID string, p PaymentInput) error {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return err
	}

	var exists bool
	_ = s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM fee_structures WHERE id=$1 AND class_id=$2)`,
		structureID, classID,
	).Scan(&exists)
	if !exists {
		return ErrStructureNotFound
	}

	_, err := s.db.Exec(ctx, `
		INSERT INTO fee_records (fee_structure_id, student_id, amount_paid, paid_at, payment_mode, notes, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (fee_structure_id, student_id)
		DO UPDATE SET
			amount_paid=$3, paid_at=$4, payment_mode=$5, notes=$6, updated_at=NOW()`,
		structureID, p.StudentID, p.AmountPaid,
		nullableString(p.PaidAt), nullableString(p.PaymentMode), nullableString(p.Notes),
	)
	return err
}

func nullableString(s string) interface{} {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}
