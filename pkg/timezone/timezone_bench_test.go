package timezone_test

import (
	"testing"
	"time"

	"school-management/backend/pkg/timezone"
)

func BenchmarkIsLocked(b *testing.B) {
	sessionDate := time.Date(2026, 5, 19, 0, 0, 0, 0, timezone.IST)
	now := timezone.NowIST()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		timezone.IsLocked(sessionDate, now)
	}
}

func BenchmarkNowIST(b *testing.B) {
	for i := 0; i < b.N; i++ {
		timezone.NowIST()
	}
}
