-- Examination: exam events (schedule headers) and slots (per-subject schedule)
-- Run this in Supabase SQL Editor

CREATE TABLE IF NOT EXISTS exam_events (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    class_id      UUID        NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    teacher_id    TEXT        NOT NULL,
    title         TEXT        NOT NULL,   -- e.g. "Mid-Term 2025"
    academic_year TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One row per (event × subject day)
CREATE TABLE IF NOT EXISTS exam_slots (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id      UUID        NOT NULL REFERENCES exam_events(id) ON DELETE CASCADE,
    subject_name  TEXT        NOT NULL,
    exam_date     DATE        NOT NULL,
    start_time    TIME,
    end_time      TIME,
    room_number   TEXT,
    max_marks     INTEGER     NOT NULL DEFAULT 100,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(event_id, subject_name)
);

CREATE INDEX IF NOT EXISTS idx_exam_events_class_id ON exam_events(class_id);
CREATE INDEX IF NOT EXISTS idx_exam_slots_event_id  ON exam_slots(event_id);
