package library

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrBookNotFound    = errors.New("book_not_found")
	ErrIssueNotFound   = errors.New("issue_not_found")
	ErrNoCopiesLeft    = errors.New("no_copies_available")
)

type Book struct {
	ID              string  `json:"id"`
	TeacherID       string  `json:"teacher_id"`
	Title           string  `json:"title"`
	Author          *string `json:"author,omitempty"`
	ISBN            *string `json:"isbn,omitempty"`
	Category        *string `json:"category,omitempty"`
	TotalCopies     int     `json:"total_copies"`
	AvailableCopies int     `json:"available_copies"`
	CreatedAt       string  `json:"created_at"`
}

type BookIssue struct {
	ID          string  `json:"id"`
	BookID      string  `json:"book_id"`
	BookTitle   string  `json:"book_title"`
	TeacherID   string  `json:"teacher_id"`
	StudentName string  `json:"student_name"`
	ClassName   *string `json:"class_name,omitempty"`
	IssuedAt    string  `json:"issued_at"`
	DueDate     *string `json:"due_date,omitempty"`
	ReturnedAt  *string `json:"returned_at,omitempty"`
	Notes       *string `json:"notes,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service { return &Service{db: db} }

// ── Books ─────────────────────────────────────────────────────────────────────

func (s *Service) ListBooks(ctx context.Context, teacherID string) ([]Book, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, teacher_id, title, author, isbn, category,
		       total_copies, available_copies, created_at::text
		FROM books WHERE teacher_id=$1 ORDER BY title`,
		teacherID,
	)
	if err != nil {
		return nil, fmt.Errorf("list books: %w", err)
	}
	defer rows.Close()

	var out []Book
	for rows.Next() {
		var b Book
		if err := rows.Scan(&b.ID, &b.TeacherID, &b.Title, &b.Author, &b.ISBN,
			&b.Category, &b.TotalCopies, &b.AvailableCopies, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	if out == nil {
		out = []Book{}
	}
	return out, nil
}

type BookInput struct {
	Title        string
	Author       string
	ISBN         string
	Category     string
	TotalCopies  int
}

func (s *Service) CreateBook(ctx context.Context, teacherID string, in BookInput) (*Book, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	copies := in.TotalCopies
	if copies < 1 {
		copies = 1
	}
	var b Book
	err := s.db.QueryRow(ctx, `
		INSERT INTO books (teacher_id, title, author, isbn, category, total_copies, available_copies)
		VALUES ($1,$2,$3,$4,$5,$6,$6)
		RETURNING id, teacher_id, title, author, isbn, category,
		          total_copies, available_copies, created_at::text`,
		teacherID, title,
		nullableString(in.Author), nullableString(in.ISBN), nullableString(in.Category),
		copies,
	).Scan(&b.ID, &b.TeacherID, &b.Title, &b.Author, &b.ISBN,
		&b.Category, &b.TotalCopies, &b.AvailableCopies, &b.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create book: %w", err)
	}
	return &b, nil
}

func (s *Service) UpdateBook(ctx context.Context, teacherID, bookID string, in BookInput) (*Book, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	copies := in.TotalCopies
	if copies < 1 {
		copies = 1
	}
	var b Book
	err := s.db.QueryRow(ctx, `
		UPDATE books SET title=$1, author=$2, isbn=$3, category=$4, total_copies=$5,
		       available_copies = GREATEST(0, available_copies + ($5 - total_copies))
		WHERE id=$6 AND teacher_id=$7
		RETURNING id, teacher_id, title, author, isbn, category,
		          total_copies, available_copies, created_at::text`,
		title, nullableString(in.Author), nullableString(in.ISBN),
		nullableString(in.Category), copies, bookID, teacherID,
	).Scan(&b.ID, &b.TeacherID, &b.Title, &b.Author, &b.ISBN,
		&b.Category, &b.TotalCopies, &b.AvailableCopies, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrBookNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update book: %w", err)
	}
	return &b, nil
}

func (s *Service) DeleteBook(ctx context.Context, teacherID, bookID string) error {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM books WHERE id=$1 AND teacher_id=$2`, bookID, teacherID)
	if err != nil {
		return fmt.Errorf("delete book: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrBookNotFound
	}
	return nil
}

// ── Issues ────────────────────────────────────────────────────────────────────

func (s *Service) ListIssues(ctx context.Context, teacherID string, activeOnly bool) ([]BookIssue, error) {
	query := `
		SELECT bi.id, bi.book_id, b.title, bi.teacher_id, bi.student_name,
		       bi.class_name, bi.issued_at::text, bi.due_date::text,
		       bi.returned_at::text, bi.notes, bi.created_at::text
		FROM book_issues bi JOIN books b ON b.id=bi.book_id
		WHERE bi.teacher_id=$1`
	if activeOnly {
		query += ` AND bi.returned_at IS NULL`
	}
	query += ` ORDER BY bi.issued_at DESC`

	rows, err := s.db.Query(ctx, query, teacherID)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}
	defer rows.Close()

	var out []BookIssue
	for rows.Next() {
		var i BookIssue
		if err := rows.Scan(&i.ID, &i.BookID, &i.BookTitle, &i.TeacherID,
			&i.StudentName, &i.ClassName, &i.IssuedAt, &i.DueDate,
			&i.ReturnedAt, &i.Notes, &i.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	if out == nil {
		out = []BookIssue{}
	}
	return out, nil
}

func (s *Service) IssueBook(ctx context.Context, teacherID, bookID, studentName, className, dueDate, notes string) (*BookIssue, error) {
	studentName = strings.TrimSpace(studentName)
	if studentName == "" {
		return nil, fmt.Errorf("student_name is required")
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Decrement available_copies; fail if already 0
	tag, err := tx.Exec(ctx, `
		UPDATE books SET available_copies = available_copies - 1
		WHERE id=$1 AND teacher_id=$2 AND available_copies > 0`,
		bookID, teacherID,
	)
	if err != nil {
		return nil, fmt.Errorf("reserve copy: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Either book not found or no copies left
		var exists bool
		_ = tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM books WHERE id=$1 AND teacher_id=$2)`,
			bookID, teacherID,
		).Scan(&exists)
		if !exists {
			return nil, ErrBookNotFound
		}
		return nil, ErrNoCopiesLeft
	}

	var issue BookIssue
	err = tx.QueryRow(ctx, `
		INSERT INTO book_issues (book_id, teacher_id, student_name, class_name, due_date, notes)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, book_id, $7::text, teacher_id, student_name,
		          class_name, issued_at::text, due_date::text,
		          returned_at::text, notes, created_at::text`,
		bookID, teacherID, studentName,
		nullableString(className), nullableString(dueDate), nullableString(notes),
		bookID, // placeholder for book_title — we join below
	).Scan(&issue.ID, &issue.BookID, &issue.BookTitle, &issue.TeacherID,
		&issue.StudentName, &issue.ClassName, &issue.IssuedAt, &issue.DueDate,
		&issue.ReturnedAt, &issue.Notes, &issue.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create issue: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	// Fetch book title for response
	_ = s.db.QueryRow(ctx, `SELECT title FROM books WHERE id=$1`, bookID).Scan(&issue.BookTitle)
	return &issue, nil
}

func (s *Service) ReturnBook(ctx context.Context, teacherID, issueID string) (*BookIssue, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var bookID string
	var issue BookIssue
	err = tx.QueryRow(ctx, `
		UPDATE book_issues SET returned_at=CURRENT_DATE
		WHERE id=$1 AND teacher_id=$2 AND returned_at IS NULL
		RETURNING id, book_id, teacher_id, student_name,
		          class_name, issued_at::text, due_date::text,
		          returned_at::text, notes, created_at::text`,
		issueID, teacherID,
	).Scan(&issue.ID, &bookID, &issue.TeacherID,
		&issue.StudentName, &issue.ClassName, &issue.IssuedAt, &issue.DueDate,
		&issue.ReturnedAt, &issue.Notes, &issue.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrIssueNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("mark returned: %w", err)
	}

	issue.BookID = bookID
	_, err = tx.Exec(ctx,
		`UPDATE books SET available_copies = available_copies + 1 WHERE id=$1`, bookID)
	if err != nil {
		return nil, fmt.Errorf("release copy: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	_ = s.db.QueryRow(ctx, `SELECT title FROM books WHERE id=$1`, bookID).Scan(&issue.BookTitle)
	return &issue, nil
}

func nullableString(s string) interface{} {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}
