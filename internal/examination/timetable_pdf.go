package examination

import (
	"fmt"
	"time"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/consts/pagesize"
	"github.com/johnfercher/maroto/v2/pkg/props"
)

// GenerateTimetablePDF generates a portrait A4 exam timetable PDF.
func GenerateTimetablePDF(data *TimetableData) ([]byte, error) {
	cfg := config.NewBuilder().
		WithPageSize(pagesize.A4).
		WithLeftMargin(15).WithRightMargin(15).
		WithTopMargin(12).WithBottomMargin(12).
		Build()

	m := maroto.New(cfg)

	yearStr := ""
	if data.AcademicYear != nil {
		yearStr = "  |  " + *data.AcademicYear
	}

	m.AddRows(
		text.NewRow(16, "EXAMINATION TIMETABLE", props.Text{
			Size: 16, Style: fontstyle.Bold, Align: align.Center, Top: 4,
		}),
		text.NewRow(10, data.ClassName+" — "+data.EventTitle+yearStr, props.Text{
			Size: 12, Align: align.Center,
		}),
		text.NewRow(8, "Generated: "+time.Now().Format("02 Jan 2006"), props.Text{
			Size: 9, Align: align.Center,
			Color: &props.Color{Red: 120, Green: 120, Blue: 120},
		}),
		text.NewRow(4, "", props.Text{}),
	)

	// Table header
	m.AddRow(8,
		text.NewCol(1, "#", props.Text{Size: 8, Style: fontstyle.Bold, Align: align.Center}),
		text.NewCol(3, "Date", props.Text{Size: 8, Style: fontstyle.Bold}),
		text.NewCol(3, "Subject", props.Text{Size: 8, Style: fontstyle.Bold}),
		text.NewCol(2, "Time", props.Text{Size: 8, Style: fontstyle.Bold, Align: align.Center}),
		text.NewCol(2, "Room", props.Text{Size: 8, Style: fontstyle.Bold, Align: align.Center}),
		text.NewCol(1, "Max", props.Text{Size: 8, Style: fontstyle.Bold, Align: align.Center}),
	)

	for i, slot := range data.Slots {
		timeStr := "—"
		if slot.StartTime != nil {
			timeStr = fmtTime(*slot.StartTime)
			if slot.EndTime != nil {
				timeStr += "\n" + fmtTime(*slot.EndTime)
			}
		}
		roomStr := "—"
		if slot.RoomNumber != nil {
			roomStr = *slot.RoomNumber
		}

		dateStr := slot.ExamDate
		if t, err := time.Parse("2006-01-02", slot.ExamDate); err == nil {
			dateStr = t.Format("Mon, 02 Jan 2006")
		}

		m.AddRow(7,
			text.NewCol(1, fmt.Sprintf("%d", i+1), props.Text{Size: 8, Align: align.Center}),
			text.NewCol(3, dateStr, props.Text{Size: 8}),
			text.NewCol(3, slot.SubjectName, props.Text{Size: 8, Style: fontstyle.Bold}),
			text.NewCol(2, timeStr, props.Text{Size: 7, Align: align.Center}),
			text.NewCol(2, roomStr, props.Text{Size: 8, Align: align.Center}),
			text.NewCol(1, fmt.Sprintf("%d", slot.MaxMarks), props.Text{Size: 8, Align: align.Center}),
		)
	}

	if len(data.Slots) == 0 {
		m.AddRows(text.NewRow(12, "No exam slots defined.", props.Text{
			Align: align.Center, Top: 4,
			Color: &props.Color{Red: 150, Green: 150, Blue: 150},
		}))
	}

	m.AddRows(
		text.NewRow(6, "", props.Text{}),
		text.NewRow(8, fmt.Sprintf("Total Subjects: %d    |    Students: %d",
			len(data.Slots), data.StudentCount),
			props.Text{Size: 9, Align: align.Center,
				Color: &props.Color{Red: 100, Green: 100, Blue: 100}},
		),
	)

	doc, err := m.Generate()
	if err != nil {
		return nil, fmt.Errorf("timetable pdf generate: %w", err)
	}
	return doc.GetBytes(), nil
}

// fmtTime converts "15:04:05" to "03:04 PM".
func fmtTime(s string) string {
	t, err := time.Parse("15:04:05", s)
	if err != nil {
		t, err = time.Parse("15:04", s)
		if err != nil {
			return s
		}
	}
	return t.Format("03:04 PM")
}
