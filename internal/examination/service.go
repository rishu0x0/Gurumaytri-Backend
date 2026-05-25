package examination

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
	ErrEventNotFound = errors.New("exam_event_not_found")
	ErrSlotNotFound  = errors.New("exam_slot_not_found")
	ErrDuplicateSlot = errors.New("duplicate_subject_slot")
)

type ExamEvent struct {
	ID           string `json:"id"`
	ClassID      string `json:"class_id"`
	Title        string `json:"title"`
	AcademicYear *string `json:"academic_year,omitempty"`
	CreatedAt    string `json:"created_at"`
	SlotCount    int    `json:"slot_count"`
}

type ExamSlot struct {
	ID          string  `json:"id"`
	EventID     string  `json:"event_id"`
	SubjectName string  `json:"subject_name"`
	ExamDate    string  `json:"exam_date"`
	StartTime   *string `json:"start_time,omitempty"`
	EndTime     *string `json:"end_time,omitempty"`
	RoomNumber  *string `json:"room_number,omitempty"`
	MaxMarks    int     `json:"max_marks"`
	CreatedAt   string  `json:"created_at"`
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

// ── Events ────────────────────────────────────────────────────────────────────

func (s *Service) ListEvents(ctx context.Context, classID, teacherID string) ([]ExamEvent, error) {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT ee.id, ee.class_id, ee.title, ee.academic_year, ee.created_at::text,
		       COUNT(es.id) AS slot_count
		FROM exam_events ee
		LEFT JOIN exam_slots es ON es.event_id = ee.id
		WHERE ee.class_id=$1
		GROUP BY ee.id
		ORDER BY ee.created_at DESC`,
		classID,
	)
	if err != nil {
		return nil, fmt.Errorf("query exam events: %w", err)
	}
	defer rows.Close()

	var out []ExamEvent
	for rows.Next() {
		var e ExamEvent
		if err := rows.Scan(&e.ID, &e.ClassID, &e.Title, &e.AcademicYear, &e.CreatedAt, &e.SlotCount); err != nil {
			return nil, fmt.Errorf("scan exam event: %w", err)
		}
		out = append(out, e)
	}
	if out == nil {
		out = []ExamEvent{}
	}
	return out, nil
}

func (s *Service) CreateEvent(ctx context.Context, classID, teacherID, title, academicYear string) (*ExamEvent, error) {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}

	var e ExamEvent
	err := s.db.QueryRow(ctx, `
		INSERT INTO exam_events (class_id, teacher_id, title, academic_year)
		VALUES ($1, $2, $3, $4)
		RETURNING id, class_id, title, academic_year, created_at::text`,
		classID, teacherID, title, nullableString(academicYear),
	).Scan(&e.ID, &e.ClassID, &e.Title, &e.AcademicYear, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create exam event: %w", err)
	}
	return &e, nil
}

func (s *Service) DeleteEvent(ctx context.Context, classID, teacherID, eventID string) error {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return err
	}
	tag, err := s.db.Exec(ctx,
		`DELETE FROM exam_events WHERE id=$1 AND class_id=$2`,
		eventID, classID,
	)
	if err != nil {
		return fmt.Errorf("delete exam event: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrEventNotFound
	}
	return nil
}

// ── Slots ──────────────────────────────────────────────────────────────────────

func (s *Service) ListSlots(ctx context.Context, classID, teacherID, eventID string) ([]ExamSlot, error) {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}
	var exists bool
	_ = s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM exam_events WHERE id=$1 AND class_id=$2)`,
		eventID, classID,
	).Scan(&exists)
	if !exists {
		return nil, ErrEventNotFound
	}

	rows, err := s.db.Query(ctx, `
		SELECT id, event_id, subject_name, exam_date::text,
		       start_time::text, end_time::text, room_number, max_marks, created_at::text
		FROM exam_slots WHERE event_id=$1
		ORDER BY exam_date, start_time`,
		eventID,
	)
	if err != nil {
		return nil, fmt.Errorf("query exam slots: %w", err)
	}
	defer rows.Close()

	var out []ExamSlot
	for rows.Next() {
		var sl ExamSlot
		if err := rows.Scan(
			&sl.ID, &sl.EventID, &sl.SubjectName, &sl.ExamDate,
			&sl.StartTime, &sl.EndTime, &sl.RoomNumber, &sl.MaxMarks, &sl.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan exam slot: %w", err)
		}
		out = append(out, sl)
	}
	if out == nil {
		out = []ExamSlot{}
	}
	return out, nil
}

func (s *Service) CreateSlot(ctx context.Context, classID, teacherID, eventID, subjectName, examDate, startTime, endTime, roomNumber string, maxMarks int) (*ExamSlot, error) {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}
	subjectName = strings.TrimSpace(subjectName)
	if subjectName == "" || examDate == "" {
		return nil, fmt.Errorf("subject_name and exam_date are required")
	}
	if maxMarks <= 0 {
		maxMarks = 100
	}

	var sl ExamSlot
	err := s.db.QueryRow(ctx, `
		INSERT INTO exam_slots (event_id, subject_name, exam_date, start_time, end_time, room_number, max_marks)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, event_id, subject_name, exam_date::text,
		          start_time::text, end_time::text, room_number, max_marks, created_at::text`,
		eventID, subjectName, examDate,
		nullableString(startTime), nullableString(endTime),
		nullableString(roomNumber), maxMarks,
	).Scan(
		&sl.ID, &sl.EventID, &sl.SubjectName, &sl.ExamDate,
		&sl.StartTime, &sl.EndTime, &sl.RoomNumber, &sl.MaxMarks, &sl.CreatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrDuplicateSlot
		}
		return nil, fmt.Errorf("create exam slot: %w", err)
	}
	return &sl, nil
}

func (s *Service) UpdateSlot(ctx context.Context, classID, teacherID, eventID, slotID, subjectName, examDate, startTime, endTime, roomNumber string, maxMarks int) (*ExamSlot, error) {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}
	subjectName = strings.TrimSpace(subjectName)
	if subjectName == "" || examDate == "" {
		return nil, fmt.Errorf("subject_name and exam_date are required")
	}
	if maxMarks <= 0 {
		maxMarks = 100
	}

	var sl ExamSlot
	err := s.db.QueryRow(ctx, `
		UPDATE exam_slots SET subject_name=$1, exam_date=$2, start_time=$3,
		       end_time=$4, room_number=$5, max_marks=$6
		WHERE id=$7 AND event_id=$8
		RETURNING id, event_id, subject_name, exam_date::text,
		          start_time::text, end_time::text, room_number, max_marks, created_at::text`,
		subjectName, examDate,
		nullableString(startTime), nullableString(endTime),
		nullableString(roomNumber), maxMarks, slotID, eventID,
	).Scan(
		&sl.ID, &sl.EventID, &sl.SubjectName, &sl.ExamDate,
		&sl.StartTime, &sl.EndTime, &sl.RoomNumber, &sl.MaxMarks, &sl.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSlotNotFound
		}
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return nil, ErrDuplicateSlot
		}
		return nil, fmt.Errorf("update exam slot: %w", err)
	}
	return &sl, nil
}

func (s *Service) DeleteSlot(ctx context.Context, classID, teacherID, eventID, slotID string) error {
	if err := s.verifyClassOwnership(ctx, classID, teacherID); err != nil {
		return err
	}
	tag, err := s.db.Exec(ctx,
		`DELETE FROM exam_slots WHERE id=$1 AND event_id=$2`,
		slotID, eventID,
	)
	if err != nil {
		return fmt.Errorf("delete exam slot: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSlotNotFound
	}
	return nil
}

// GetTimetableData assembles data needed for the PDF timetable.
func (s *Service) GetTimetableData(ctx context.Context, classID, teacherID, eventID string) (*TimetableData, error) {
	slots, err := s.ListSlots(ctx, classID, teacherID, eventID)
	if err != nil {
		return nil, err
	}

	var className, eventTitle string
	var academicYear *string
	err = s.db.QueryRow(ctx, `
		SELECT c.name, ee.title, ee.academic_year
		FROM exam_events ee JOIN classes c ON c.id=ee.class_id
		WHERE ee.id=$1`, eventID,
	).Scan(&className, &eventTitle, &academicYear)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrEventNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("fetch event meta: %w", err)
	}

	// Fetch active student count for hall ticket metadata
	var studentCount int
	_ = s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM students WHERE class_id=$1 AND is_active=true`, classID,
	).Scan(&studentCount)

	return &TimetableData{
		ClassName:    className,
		EventTitle:   eventTitle,
		AcademicYear: academicYear,
		Slots:        slots,
		StudentCount: studentCount,
	}, nil
}

type TimetableData struct {
	ClassName    string
	EventTitle   string
	AcademicYear *string
	Slots        []ExamSlot
	StudentCount int
}

func nullableString(s string) interface{} {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}
