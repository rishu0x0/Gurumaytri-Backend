package timezone_test

import (
	"testing"
	"time"

	"school-management/backend/pkg/timezone"
)

func TestIsLocked_SameDayIST_NotLocked(t *testing.T) {
	// Session date: 2026-05-19, now: 23:59:59 IST — should NOT be locked
	sessionDate := time.Date(2026, 5, 19, 0, 0, 0, 0, timezone.IST)
	now := time.Date(2026, 5, 19, 23, 59, 59, 0, timezone.IST)
	if timezone.IsLocked(sessionDate, now) {
		t.Error("expected NOT locked at 23:59:59 IST same day")
	}
}

func TestIsLocked_AfterMidnightIST_Locked(t *testing.T) {
	// Session date: 2026-05-19, now: 00:00:01 IST next day — should be LOCKED
	sessionDate := time.Date(2026, 5, 19, 0, 0, 0, 0, timezone.IST)
	now := time.Date(2026, 5, 20, 0, 0, 1, 0, timezone.IST)
	if !timezone.IsLocked(sessionDate, now) {
		t.Error("expected LOCKED at 00:00:01 IST next day")
	}
}

func TestIsLocked_ExactMidnightIST_NotLocked(t *testing.T) {
	// Exactly midnight IST is the boundary itself — IsLocked uses After (strict >)
	// so the first nanosecond AFTER midnight triggers the lock, not midnight itself.
	sessionDate := time.Date(2026, 5, 19, 0, 0, 0, 0, timezone.IST)
	now := time.Date(2026, 5, 20, 0, 0, 0, 0, timezone.IST)
	if timezone.IsLocked(sessionDate, now) {
		t.Error("expected NOT locked at exactly midnight IST (boundary is exclusive)")
	}
}

func TestIsLocked_OneNsAfterMidnightIST_Locked(t *testing.T) {
	// One nanosecond past midnight IST → locked
	sessionDate := time.Date(2026, 5, 19, 0, 0, 0, 0, timezone.IST)
	now := time.Date(2026, 5, 20, 0, 0, 0, 1, timezone.IST) // 1 nanosecond past midnight
	if !timezone.IsLocked(sessionDate, now) {
		t.Error("expected LOCKED at 1ns past midnight IST")
	}
}

func TestIsLocked_UTCMidnight_NotLockedIST(t *testing.T) {
	// UTC midnight (00:00 UTC) = 05:30 IST — still within the same IST day
	// Session date: 2026-05-19 IST
	sessionDate := time.Date(2026, 5, 19, 0, 0, 0, 0, timezone.IST)
	// 2026-05-19 00:00:00 UTC = 2026-05-19 05:30:00 IST
	nowUTC := time.Date(2026, 5, 19, 0, 0, 0, 0, time.UTC)
	if timezone.IsLocked(sessionDate, nowUTC) {
		t.Error("expected NOT locked at UTC midnight (IST 05:30 same day)")
	}
}

func TestIsLocked_UTCNextDay_BeforeISTMidnight_NotLocked(t *testing.T) {
	// 2026-05-19 18:29 UTC = 2026-05-19 23:59 IST — NOT locked
	sessionDate := time.Date(2026, 5, 19, 0, 0, 0, 0, timezone.IST)
	nowUTC := time.Date(2026, 5, 19, 18, 29, 0, 0, time.UTC)
	if timezone.IsLocked(sessionDate, nowUTC) {
		t.Error("expected NOT locked at 23:59 IST (18:29 UTC)")
	}
}

func TestIsLocked_UTCNextDay_AfterISTMidnight_Locked(t *testing.T) {
	// 2026-05-19 18:31 UTC = 2026-05-20 00:01 IST — LOCKED
	sessionDate := time.Date(2026, 5, 19, 0, 0, 0, 0, timezone.IST)
	nowUTC := time.Date(2026, 5, 19, 18, 31, 0, 0, time.UTC)
	if !timezone.IsLocked(sessionDate, nowUTC) {
		t.Error("expected LOCKED at 00:01 IST (18:31 UTC)")
	}
}

func TestIsLocked_MonthBoundary(t *testing.T) {
	// Last day of month: session 2026-05-31, now: 2026-06-01 00:01 IST
	sessionDate := time.Date(2026, 5, 31, 0, 0, 0, 0, timezone.IST)
	now := time.Date(2026, 6, 1, 0, 0, 1, 0, timezone.IST)
	if !timezone.IsLocked(sessionDate, now) {
		t.Error("expected LOCKED across month boundary")
	}
}

func TestIsLocked_MonthBoundary_StillSameDay(t *testing.T) {
	// Session 2026-05-31, now 23:59 IST same day — NOT locked
	sessionDate := time.Date(2026, 5, 31, 0, 0, 0, 0, timezone.IST)
	now := time.Date(2026, 5, 31, 23, 59, 59, 0, timezone.IST)
	if timezone.IsLocked(sessionDate, now) {
		t.Error("expected NOT locked at 23:59 IST on session day (month boundary)")
	}
}
