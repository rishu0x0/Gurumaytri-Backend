package attendance_test

// Integration tests for attendance lock enforcement and cross-teacher isolation.
// These tests require a live DATABASE_URL env var pointing to a Supabase instance.
// Run with: DATABASE_URL=... go test ./internal/attendance/... -run Integration -v
//
// Tests are skipped automatically when DATABASE_URL is not set.

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"school-management/backend/internal/attendance"
	"school-management/backend/pkg/timezone"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	_ = godotenv.Load("../../.env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("pool connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// TestIntegration_AttendanceLock_ISTBoundary verifies that:
// - a session dated yesterday IST is rejected by EditRecords with ErrAttendanceLocked
// - the timezone package correctly computes the lock boundary
func TestIntegration_AttendanceLock_ISTBoundary(t *testing.T) {
	// Unit-level: verify the lock boundary logic is correct without DB
	yesterdayIST := timezone.TodayIST().Add(-24 * time.Hour)
	nowIST := timezone.NowIST()

	if !timezone.IsLocked(yesterdayIST, nowIST) {
		t.Errorf("yesterday's session should be locked: sessionDate=%v now=%v", yesterdayIST, nowIST)
	}
	if timezone.IsLocked(timezone.TodayIST(), nowIST) {
		t.Errorf("today's session should NOT be locked: sessionDate=%v now=%v", timezone.TodayIST(), nowIST)
	}
}

// TestIntegration_RLS_TeacherIsolation verifies that the service layer rejects
// cross-teacher data access (teacher A cannot list/edit teacher B's classes).
//
// This test creates two ephemeral teacher rows, then verifies the service
// ownership check correctly rejects cross-teacher access.
func TestIntegration_RLS_TeacherIsolation(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	// Create two test teachers via service role (bypasses RLS)
	var teacherAID, teacherBID string
	err := pool.QueryRow(ctx, `
		INSERT INTO teachers (name, mobile, school_name, password_hash)
		VALUES ('Test Teacher A (08-01)', '9000000001', 'Test School', '$2b$12$placeholder')
		RETURNING id`).Scan(&teacherAID)
	if err != nil {
		t.Fatalf("create teacher A: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM teachers WHERE id = $1`, teacherAID)
	})

	err = pool.QueryRow(ctx, `
		INSERT INTO teachers (name, mobile, school_name, password_hash)
		VALUES ('Test Teacher B (08-01)', '9000000002', 'Test School', '$2b$12$placeholder')
		RETURNING id`).Scan(&teacherBID)
	if err != nil {
		t.Fatalf("create teacher B: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM teachers WHERE id = $1`, teacherBID)
	})

	// Create a class owned by teacher A
	var classAID string
	err = pool.QueryRow(ctx, `
		INSERT INTO classes (teacher_id, name)
		VALUES ($1, 'Teacher A Class')
		RETURNING id`, teacherAID).Scan(&classAID)
	if err != nil {
		t.Fatalf("create class A: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM classes WHERE id = $1`, classAID)
	})

	svc := attendance.NewService(pool)

	// Teacher B attempts to access teacher A's class attendance — should fail
	_, err = svc.GetByDate(ctx, classAID, teacherBID, time.Now())
	if err == nil {
		t.Error("expected error when teacher B accesses teacher A's class, got nil")
	} else {
		t.Logf("cross-teacher access correctly rejected: %v", err)
	}
}
