package reports

import "time"

// ReportData is the input to both PDF and Excel generators.
type ReportData struct {
	ClassName   string
	Month       string // YYYY-MM
	GeneratedAt time.Time
	// Days is sorted list of calendar days with records. Max 31.
	Days     []int // 1-31
	Students []StudentRow
}

// StudentRow holds one student's attendance for the report month.
type StudentRow struct {
	Name       string
	RollNumber int
	// Records maps day-of-month → "P", "A", "L", or "—"
	Records map[int]string
	Present int
	Absent  int
	Leave   int
	Total   int // days with any record
}

// AttendanceLabel converts a DB status string to the display label.
func AttendanceLabel(status string) string {
	switch status {
	case "present":
		return "P"
	case "absent":
		return "A"
	case "leave":
		return "L"
	default:
		return "—"
	}
}
