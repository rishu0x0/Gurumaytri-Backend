-- Fees management: fee structures and student payment records
-- Run this in Supabase SQL Editor

CREATE TABLE IF NOT EXISTS fee_structures (
    id              UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    class_id        UUID          NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
    teacher_id      TEXT          NOT NULL,
    title           TEXT          NOT NULL,
    amount          NUMERIC(10,2) NOT NULL,
    due_date        DATE,
    academic_year   TEXT,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- One row per (fee_structure × student); upserted as teacher records payments
CREATE TABLE IF NOT EXISTS fee_records (
    id               UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    fee_structure_id UUID          NOT NULL REFERENCES fee_structures(id) ON DELETE CASCADE,
    student_id       UUID          NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    amount_paid      NUMERIC(10,2) NOT NULL DEFAULT 0,
    paid_at          DATE,
    payment_mode     TEXT,         -- cash | cheque | online | upi
    notes            TEXT,
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    UNIQUE(fee_structure_id, student_id)
);

CREATE INDEX IF NOT EXISTS idx_fee_structures_class_id   ON fee_structures(class_id);
CREATE INDEX IF NOT EXISTS idx_fee_records_structure_id  ON fee_records(fee_structure_id);
CREATE INDEX IF NOT EXISTS idx_fee_records_student_id    ON fee_records(student_id);
