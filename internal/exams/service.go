package exams

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrClassNotFound = errors.New("class_not_found")
	ErrExamNotFound  = errors.New("exam_not_found")
)

type Exam struct {
	ID        string  `json:"id"`
	ClassID   string  `json:"class_id"`
	Name      string  `json:"name"`
	ExamDate  *string `json:"exam_date,omitempty"`
	CreatedAt string  `json:"created_at"`
}

type Subject struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	MaxMarks int    `json:"max_marks"`
}

type Student struct {
	ID         string `json:"id"`
	FullName   string `json:"full_name"`
	RollNumber int    `json:"roll_number"`
}

type MarkEntry struct {
	StudentID string   `json:"student_id"`
	SubjectID string   `json:"subject_id"`
	Obtained  *float64 `json:"obtained"`
	IsAbsent  bool     `json:"is_absent"`
}

type MarksGrid struct {
	Subjects []Subject   `json:"subjects"`
	Students []Student   `json:"students"`
	Marks    []MarkEntry `json:"marks"`
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

func (s *Service) List(ctx context.Context, classID, teacherID string) ([]Exam, error) {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx,
		`SELECT id, class_id, name, exam_date::text, created_at::text
		 FROM exams WHERE class_id = $1
		 ORDER BY created_at DESC`,
		classID,
	)
	if err != nil {
		return nil, fmt.Errorf("query exams: %w", err)
	}
	defer rows.Close()

	var out []Exam
	for rows.Next() {
		var e Exam
		if err := rows.Scan(&e.ID, &e.ClassID, &e.Name, &e.ExamDate, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan exam: %w", err)
		}
		out = append(out, e)
	}
	if out == nil {
		out = []Exam{}
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, classID, teacherID, name, examDate string) (*Exam, error) {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("exam name is required")
	}

	var e Exam
	err := s.db.QueryRow(ctx,
		`INSERT INTO exams (class_id, teacher_id, name, exam_date)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, class_id, name, exam_date::text, created_at::text`,
		classID, teacherID, name, nullableString(examDate),
	).Scan(&e.ID, &e.ClassID, &e.Name, &e.ExamDate, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create exam: %w", err)
	}
	return &e, nil
}

func (s *Service) Delete(ctx context.Context, classID, teacherID, examID string) error {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return err
	}
	tag, err := s.db.Exec(ctx,
		`DELETE FROM exams WHERE id=$1 AND class_id=$2`,
		examID, classID,
	)
	if err != nil {
		return fmt.Errorf("delete exam: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrExamNotFound
	}
	return nil
}

func (s *Service) GetMarks(ctx context.Context, classID, teacherID, examID string) (*MarksGrid, error) {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}

	var exists bool
	_ = s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM exams WHERE id=$1 AND class_id=$2)`,
		examID, classID,
	).Scan(&exists)
	if !exists {
		return nil, ErrExamNotFound
	}

	// Subjects for this class
	subRows, err := s.db.Query(ctx,
		`SELECT id, name, max_marks FROM subjects WHERE class_id=$1 ORDER BY display_order, name`,
		classID,
	)
	if err != nil {
		return nil, fmt.Errorf("query subjects: %w", err)
	}
	defer subRows.Close()
	var subjects []Subject
	for subRows.Next() {
		var sub Subject
		if err := subRows.Scan(&sub.ID, &sub.Name, &sub.MaxMarks); err != nil {
			return nil, err
		}
		subjects = append(subjects, sub)
	}
	subRows.Close()

	// Active students
	stuRows, err := s.db.Query(ctx,
		`SELECT id, full_name, roll_number FROM students
		 WHERE class_id=$1 AND is_active=true ORDER BY roll_number`,
		classID,
	)
	if err != nil {
		return nil, fmt.Errorf("query students: %w", err)
	}
	defer stuRows.Close()
	var students []Student
	for stuRows.Next() {
		var st Student
		if err := stuRows.Scan(&st.ID, &st.FullName, &st.RollNumber); err != nil {
			return nil, err
		}
		students = append(students, st)
	}
	stuRows.Close()

	// Existing marks
	markRows, err := s.db.Query(ctx,
		`SELECT student_id, subject_id, obtained, is_absent FROM marks WHERE exam_id=$1`,
		examID,
	)
	if err != nil {
		return nil, fmt.Errorf("query marks: %w", err)
	}
	defer markRows.Close()
	var marks []MarkEntry
	for markRows.Next() {
		var m MarkEntry
		if err := markRows.Scan(&m.StudentID, &m.SubjectID, &m.Obtained, &m.IsAbsent); err != nil {
			return nil, err
		}
		marks = append(marks, m)
	}
	if marks == nil {
		marks = []MarkEntry{}
	}

	if subjects == nil {
		subjects = []Subject{}
	}
	if students == nil {
		students = []Student{}
	}

	return &MarksGrid{
		Subjects: subjects,
		Students: students,
		Marks:    marks,
	}, nil
}

// UpsertMarks bulk-inserts or updates mark entries for an exam.
func (s *Service) UpsertMarks(ctx context.Context, classID, teacherID, examID string, entries []MarkEntry) error {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return err
	}

	var exists bool
	_ = s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM exams WHERE id=$1 AND class_id=$2)`,
		examID, classID,
	).Scan(&exists)
	if !exists {
		return ErrExamNotFound
	}

	for _, e := range entries {
		_, err := s.db.Exec(ctx, `
			INSERT INTO marks (exam_id, student_id, subject_id, obtained, is_absent, updated_at)
			VALUES ($1, $2, $3, $4, $5, NOW())
			ON CONFLICT (exam_id, student_id, subject_id)
			DO UPDATE SET obtained=$4, is_absent=$5, updated_at=NOW()`,
			examID, e.StudentID, e.SubjectID, e.Obtained, e.IsAbsent,
		)
		if err != nil {
			return fmt.Errorf("upsert mark student=%s subject=%s: %w", e.StudentID, e.SubjectID, err)
		}
	}
	return nil
}

// GetMarksheetData assembles everything needed to generate the marksheet PDF.
func (s *Service) GetMarksheetData(ctx context.Context, classID, teacherID, examID string) (*MarksheetData, error) {
	grid, err := s.GetMarks(ctx, classID, teacherID, examID)
	if err != nil {
		return nil, err
	}

	var className, examName string
	var examDate *string
	err = s.db.QueryRow(ctx,
		`SELECT c.name, e.name, e.exam_date::text
		 FROM exams e JOIN classes c ON c.id = e.class_id
		 WHERE e.id=$1`,
		examID,
	).Scan(&className, &examName, &examDate)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrExamNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("fetch exam meta: %w", err)
	}

	return &MarksheetData{
		ClassName: className,
		ExamName:  examName,
		ExamDate:  examDate,
		Grid:      *grid,
	}, nil
}

type MarksheetData struct {
	ClassName string
	ExamName  string
	ExamDate  *string
	Grid      MarksGrid
}

func nullableString(s string) interface{} {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}
