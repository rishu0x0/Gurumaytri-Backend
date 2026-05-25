-- Phase: Timetable — Class Schedule Planner

CREATE TABLE timetable_slots (
  id            UUID         DEFAULT gen_random_uuid() PRIMARY KEY,
  class_id      UUID         NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  teacher_id    TEXT         NOT NULL,
  day_of_week   SMALLINT     NOT NULL CHECK (day_of_week BETWEEN 0 AND 5), -- 0=Mon…5=Sat
  period_number SMALLINT     NOT NULL CHECK (period_number BETWEEN 1 AND 12),
  subject_name  TEXT         NOT NULL,
  start_time    TIME,
  end_time      TIME,
  room_number   TEXT,
  created_at    TIMESTAMPTZ  DEFAULT NOW(),
  UNIQUE (class_id, day_of_week, period_number)
);

CREATE INDEX idx_timetable_class ON timetable_slots(class_id);
