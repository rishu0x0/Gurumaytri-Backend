-- Phase: Admission — Student Enrollment

CREATE TABLE admission_inquiries (
  id              UUID         DEFAULT gen_random_uuid() PRIMARY KEY,
  teacher_id      TEXT         NOT NULL,
  student_name    TEXT         NOT NULL,
  date_of_birth   DATE,
  parent_name     TEXT,
  contact_number  TEXT,
  email           TEXT,
  previous_school TEXT,
  class_applied   TEXT,
  academic_year   TEXT,
  status          TEXT         NOT NULL DEFAULT 'pending'
                               CHECK (status IN ('pending', 'approved', 'rejected')),
  notes           TEXT,
  created_at      TIMESTAMPTZ  DEFAULT NOW(),
  updated_at      TIMESTAMPTZ  DEFAULT NOW()
);

CREATE INDEX idx_admission_teacher ON admission_inquiries(teacher_id);
CREATE INDEX idx_admission_status  ON admission_inquiries(teacher_id, status);
