package students

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
	ErrStudentNotFound = errors.New("student_not_found")
	ErrDuplicateRoll   = errors.New("duplicate_roll_number")
)

// Student mirrors the students table columns exactly.
// NOTE: the students table has no updated_at column.
type Student struct {
	ID         string  `json:"id"`
	ClassID    string  `json:"class_id"`
	RollNumber int     `json:"roll_number"`
	FullName   string  `json:"full_name"`
	PhotoURL   *string `json:"photo_url,omitempty"`
	IsActive   bool    `json:"is_active"`
	CreatedAt  string  `json:"created_at"`
}

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// verifyClassOwnership confirms the class exists and belongs to the teacher.
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

func (s *Service) List(ctx context.Context, classID, teacherID string) ([]Student, error) {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}

	rows, err := s.db.Query(ctx,
		`SELECT id, class_id, roll_number, full_name, photo_url, is_active, created_at::text
		 FROM students
		 WHERE class_id = $1
		 ORDER BY roll_number ASC`,
		classID,
	)
	if err != nil {
		return nil, fmt.Errorf("query students: %w", err)
	}
	defer rows.Close()

	var out []Student
	for rows.Next() {
		var st Student
		if err := rows.Scan(&st.ID, &st.ClassID, &st.RollNumber, &st.FullName, &st.PhotoURL, &st.IsActive, &st.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan student: %w", err)
		}
		out = append(out, st)
	}
	if out == nil {
		out = []Student{}
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, classID, teacherID, fullName string, rollNumber *int, photoURL string) (*Student, error) {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}

	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return nil, fmt.Errorf("full_name is required")
	}

	// Auto-assign roll number if not provided.
	if rollNumber == nil {
		var maxRoll int
		_ = s.db.QueryRow(ctx,
			`SELECT COALESCE(MAX(roll_number), 0) FROM students WHERE class_id = $1`,
			classID,
		).Scan(&maxRoll)
		next := maxRoll + 1
		rollNumber = &next
	}

	var st Student
	err := s.db.QueryRow(ctx,
		`INSERT INTO students (class_id, roll_number, full_name, photo_url)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, class_id, roll_number, full_name, photo_url, is_active, created_at::text`,
		classID, *rollNumber, fullName, nullableString(photoURL),
	).Scan(&st.ID, &st.ClassID, &st.RollNumber, &st.FullName, &st.PhotoURL, &st.IsActive, &st.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrDuplicateRoll
		}
		return nil, fmt.Errorf("create student: %w", err)
	}
	return &st, nil
}

// Update patches one or more student fields.
// IMPORTANT: students table has NO updated_at column — do not include it in SET clauses.
func (s *Service) Update(ctx context.Context, classID, teacherID, studentID string, fullName *string, rollNumber *int, photoURL *string) (*Student, error) {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}

	// Build dynamic SET clause — only columns provided by the caller.
	// students table columns: id, class_id, roll_number, full_name, photo_url, is_active, created_at
	// There is NO updated_at column; do not add it.
	setClauses := []string{}
	args := []interface{}{}
	argIdx := 1

	if fullName != nil {
		name := strings.TrimSpace(*fullName)
		if name == "" {
			return nil, fmt.Errorf("full_name cannot be empty")
		}
		setClauses = append(setClauses, fmt.Sprintf("full_name = $%d", argIdx))
		args = append(args, name)
		argIdx++
	}
	if rollNumber != nil {
		setClauses = append(setClauses, fmt.Sprintf("roll_number = $%d", argIdx))
		args = append(args, *rollNumber)
		argIdx++
	}
	if photoURL != nil {
		setClauses = append(setClauses, fmt.Sprintf("photo_url = $%d", argIdx))
		args = append(args, nullableString(*photoURL))
		argIdx++
	}

	if len(setClauses) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	args = append(args, studentID, classID)
	query := fmt.Sprintf(
		`UPDATE students SET %s WHERE id = $%d AND class_id = $%d
		 RETURNING id, class_id, roll_number, full_name, photo_url, is_active, created_at::text`,
		strings.Join(setClauses, ", "), argIdx, argIdx+1,
	)

	var st Student
	err := s.db.QueryRow(ctx, query, args...).
		Scan(&st.ID, &st.ClassID, &st.RollNumber, &st.FullName, &st.PhotoURL, &st.IsActive, &st.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrStudentNotFound
		}
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrDuplicateRoll
		}
		return nil, fmt.Errorf("update student: %w", err)
	}
	return &st, nil
}

// SoftRemove sets is_active=false so attendance history is preserved.
func (s *Service) SoftRemove(ctx context.Context, classID, teacherID, studentID string) error {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return err
	}

	tag, err := s.db.Exec(ctx,
		`UPDATE students SET is_active = false WHERE id = $1 AND class_id = $2`,
		studentID, classID,
	)
	if err != nil {
		return fmt.Errorf("soft remove student: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrStudentNotFound
	}
	return nil
}

// Seed creates N dummy students with sequential roll numbers starting after the current max.
// Uses ON CONFLICT DO NOTHING so re-seeding the same class never errors.
func (s *Service) Seed(ctx context.Context, classID, teacherID string, count int) (int, error) {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return 0, err
	}
	if count <= 0 {
		count = 30
	}

	var maxRoll int
	_ = s.db.QueryRow(ctx,
		`SELECT COALESCE(MAX(roll_number), 0) FROM students WHERE class_id = $1`,
		classID,
	).Scan(&maxRoll)

	created := 0
	for i := 1; i <= count; i++ {
		roll := maxRoll + i
		name := fmt.Sprintf("Student %d", roll)
		tag, err := s.db.Exec(ctx,
			`INSERT INTO students (class_id, roll_number, full_name)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (class_id, roll_number) DO NOTHING`,
			classID, roll, name,
		)
		if err == nil && tag.RowsAffected() > 0 {
			created++
		}
	}
	return created, nil
}

// nullableString returns nil for blank strings, enabling NULL storage in TEXT columns.
func nullableString(s string) interface{} {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}
