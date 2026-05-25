-- Phase: Homework — Assignment Tracker

CREATE TABLE assignments (
  id          UUID         DEFAULT gen_random_uuid() PRIMARY KEY,
  class_id    UUID         NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  teacher_id  TEXT         NOT NULL,
  subject     TEXT         NOT NULL,
  title       TEXT         NOT NULL,
  description TEXT,
  due_date    DATE         NOT NULL,
  created_at  TIMESTAMPTZ  DEFAULT NOW()
);

CREATE INDEX idx_assignments_class ON assignments(class_id);
CREATE INDEX idx_assignments_due   ON assignments(class_id, due_date);
