package timezone

import (
	"time"
)

// IST is the Asia/Kolkata timezone loaded once at startup.
// Loaded with time.LoadLocation — never UTC+5:30 offset arithmetic.
var IST *time.Location

func init() {
	var err error
	IST, err = time.LoadLocation("Asia/Kolkata")
	if err != nil {
		// This can only fail if the timezone database is missing from the OS.
		// On Linux/macOS it is always available; on minimal Docker images use
		// tzdata package. Fail fast here rather than silently using UTC.
		panic("failed to load Asia/Kolkata timezone: " + err.Error())
	}
}

// IsLocked returns true if the given moment (now) is past midnight IST
// of the session date. The session is locked once it is no longer "today" in IST.
//
// Example: sessionDate = 2026-05-19
//   - now = 2026-05-19 23:59 IST → NOT locked (still same day in IST)
//   - now = 2026-05-20 00:01 IST → LOCKED (past midnight of session date)
func IsLocked(sessionDate time.Time, now time.Time) bool {
	// Determine the end-of-day boundary: midnight IST of the day AFTER sessionDate
	sessionDateIST := sessionDate.In(IST)
	endOfDayIST := time.Date(
		sessionDateIST.Year(),
		sessionDateIST.Month(),
		sessionDateIST.Day()+1, // next calendar day
		0, 0, 0, 0,
		IST,
	)
	return now.UTC().After(endOfDayIST.UTC())
}

// NowIST returns the current time in IST.
func NowIST() time.Time {
	return time.Now().In(IST)
}

// TodayIST returns today's date in IST as a time.Time (midnight IST of today).
func TodayIST() time.Time {
	now := NowIST()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, IST)
}
