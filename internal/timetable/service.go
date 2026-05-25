package timetable

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
	ErrSlotNotFound  = errors.New("timetable_slot_not_found")
)

type Slot struct {
	ID           string  `json:"id"`
	ClassID      string  `json:"class_id"`
	DayOfWeek    int     `json:"day_of_week"`
	PeriodNumber int     `json:"period_number"`
	SubjectName  string  `json:"subject_name"`
	StartTime    *string `json:"start_time,omitempty"`
	EndTime      *string `json:"end_time,omitempty"`
	RoomNumber   *string `json:"room_number,omitempty"`
	CreatedAt    string  `json:"created_at"`
}

type TimetableData struct {
	ClassName string
	Slots     []Slot
}

type Service struct {
	db *pgxpool.Pool
}

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

func (s *Service) List(ctx context.Context, classID, teacherID string) ([]Slot, error) {
	if err := s.verifyOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, class_id, day_of_week, period_number, subject_name,
		       start_time::text, end_time::text, room_number, created_at::text
		FROM timetable_slots
		WHERE class_id=$1
		ORDER BY day_of_week, period_number`,
		classID,
	)
	if err != nil {
		return nil, fmt.Errorf("query timetable: %w", err)
	}
	defer rows.Close()

	var out []Slot
	for rows.Next() {
		var sl Slot
		if err := rows.Scan(&sl.ID, &sl.ClassID, &sl.DayOfWeek, &sl.PeriodNumber,
			&sl.SubjectName, &sl.StartTime, &sl.EndTime, &sl.RoomNumber, &sl.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan slot: %w", err)
		}
		out = append(out, sl)
	}
	if out == nil {
		out = []Slot{}
	}
	return out, nil
}

type UpsertInput struct {
	DayOfWeek    int
	PeriodNumber int
	SubjectName  string
	StartTime    string
	EndTime      string
	RoomNumber   string
}

func (s *Service) Upsert(ctx context.Context, classID, teacherID string, in UpsertInput) (*Slot, error) {
	if err := s.verifyOwnership(ctx, classID, teacherID); err != nil {
		return nil, err
	}
	subject := strings.TrimSpace(in.SubjectName)
	if subject == "" {
		return nil, fmt.Errorf("subject_name is required")
	}

	var sl Slot
	err := s.db.QueryRow(ctx, `
		INSERT INTO timetable_slots
		  (class_id, teacher_id, day_of_week, period_number, subject_name, start_time, end_time, room_number)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (class_id, day_of_week, period_number) DO UPDATE
		  SET subject_name=$5, start_time=$6, end_time=$7, room_number=$8
		RETURNING id, class_id, day_of_week, period_number, subject_name,
		          start_time::text, end_time::text, room_number, created_at::text`,
		classID, teacherID, in.DayOfWeek, in.PeriodNumber, subject,
		nullableString(in.StartTime), nullableString(in.EndTime), nullableString(in.RoomNumber),
	).Scan(&sl.ID, &sl.ClassID, &sl.DayOfWeek, &sl.PeriodNumber, &sl.SubjectName,
		&sl.StartTime, &sl.EndTime, &sl.RoomNumber, &sl.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert slot: %w", err)
	}
	return &sl, nil
}

func (s *Service) Delete(ctx context.Context, classID, teacherID, slotID string) error {
	if err := s.verifyOwnership(ctx, classID, teacherID); err != nil {
		return err
	}
	tag, err := s.db.Exec(ctx,
		`DELETE FROM timetable_slots WHERE id=$1 AND class_id=$2`,
		slotID, classID,
	)
	if err != nil {
		return fmt.Errorf("delete slot: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrSlotNotFound
	}
	return nil
}

func (s *Service) GetTimetableData(ctx context.Context, classID, teacherID string) (*TimetableData, error) {
	var className string
	err := s.db.QueryRow(ctx,
		`SELECT name FROM classes WHERE id=$1 AND teacher_id=$2`,
		classID, teacherID,
	).Scan(&className)
	if errors.Is(err, pgx.ErrNoRows) || err != nil {
		return nil, ErrClassNotFound
	}

	slots, err := s.List(ctx, classID, teacherID)
	if err != nil {
		return nil, err
	}
	return &TimetableData{ClassName: className, Slots: slots}, nil
}

func nullableString(s string) interface{} {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}
