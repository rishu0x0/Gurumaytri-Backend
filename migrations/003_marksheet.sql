-- Marksheet: subjects, exams, marks
-- Run this in Supabase SQL Editor

CREATE TABLE IF NOT EXISTS subjects (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    class_id      UUID        NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    teacher_id    TEXT        NOT NULL,
    name          TEXT        NOT NULL,
    max_marks     INTEGER     NOT NULL DEFAULT 100,
    display_order INTEGER     NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(class_id, name)
);

CREATE TABLE IF NOT EXISTS exams (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    class_id      UUID        NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    teacher_id    TEXT        NOT NULL,
    name          TEXT        NOT NULL,
    exam_date     DATE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One row per (exam × student × subject)
CREATE TABLE IF NOT EXISTS marks (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    exam_id       UUID        NOT NULL REFERENCES exams(id) ON DELETE CASCADE,
    student_id    UUID        NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    subject_id    UUID        NOT NULL REFERENCES subjects(id) ON DELETE CASCADE,
    obtained      NUMERIC(6,2),
    is_absent     BOOLEAN     NOT NULL DEFAULT FALSE,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(exam_id, student_id, subject_id)
);

CREATE INDEX IF NOT EXISTS idx_subjects_class_id ON subjects(class_id);
CREATE INDEX IF NOT EXISTS idx_exams_class_id    ON exams(class_id);
CREATE INDEX IF NOT EXISTS idx_marks_exam_id     ON marks(exam_id);
CREATE INDEX IF NOT EXISTS idx_marks_student_id  ON marks(student_id);
