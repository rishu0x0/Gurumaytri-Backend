-- API request log table
-- Run this in Supabase SQL Editor

CREATE TABLE IF NOT EXISTS api_logs (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id   TEXT,
    device_id    TEXT,
    teacher_id   TEXT,                    -- nullable for unauthenticated routes
    method       TEXT        NOT NULL,
    endpoint     TEXT        NOT NULL,    -- Chi route pattern e.g. /classes/{classID}/students
    action       TEXT,                    -- human-readable description of what the API does
    status_code  INTEGER     NOT NULL,
    status       TEXT        NOT NULL,    -- 'success' (< 400) | 'failed' (>= 400)
    message      TEXT,                    -- extracted from response body
    ip_address   TEXT,
    user_agent   TEXT,
    duration_ms  BIGINT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_logs_created_at  ON api_logs (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_api_logs_device_id   ON api_logs (device_id);
CREATE INDEX IF NOT EXISTS idx_api_logs_teacher_id  ON api_logs (teacher_id);
CREATE INDEX IF NOT EXISTS idx_api_logs_status      ON api_logs (status);
CREATE INDEX IF NOT EXISTS idx_api_logs_endpoint    ON api_logs (endpoint);
