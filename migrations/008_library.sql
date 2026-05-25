-- Phase: Library — Books & Resources

CREATE TABLE books (
  id               UUID         DEFAULT gen_random_uuid() PRIMARY KEY,
  teacher_id       TEXT         NOT NULL,
  title            TEXT         NOT NULL,
  author           TEXT,
  isbn             TEXT,
  category         TEXT,
  total_copies     INT          NOT NULL DEFAULT 1 CHECK (total_copies >= 1),
  available_copies INT          NOT NULL DEFAULT 1 CHECK (available_copies >= 0),
  created_at       TIMESTAMPTZ  DEFAULT NOW()
);

CREATE TABLE book_issues (
  id           UUID         DEFAULT gen_random_uuid() PRIMARY KEY,
  book_id      UUID         NOT NULL REFERENCES books(id) ON DELETE CASCADE,
  teacher_id   TEXT         NOT NULL,
  student_name TEXT         NOT NULL,
  class_name   TEXT,
  issued_at    DATE         NOT NULL DEFAULT CURRENT_DATE,
  due_date     DATE,
  returned_at  DATE,
  notes        TEXT,
  created_at   TIMESTAMPTZ  DEFAULT NOW()
);

CREATE INDEX idx_books_teacher   ON books(teacher_id);
CREATE INDEX idx_issues_book     ON book_issues(book_id);
CREATE INDEX idx_issues_teacher  ON book_issues(teacher_id);
CREATE INDEX idx_issues_returned ON book_issues(teacher_id, returned_at);
