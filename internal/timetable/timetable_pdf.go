package timetable

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

var dayNames = [6]string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

// GenerateTimetablePDF generates a portrait A4 weekly timetable PDF grouped by day.
func GenerateTimetablePDF(data *TimetableData) ([]byte, error) {
	cfg := config.NewBuilder().
		WithPageSize(pagesize.A4).
		WithLeftMargin(15).WithRightMargin(15).
		WithTopMargin(12).WithBottomMargin(12).
		Build()

	m := maroto.New(cfg)

	// Title
	m.AddRows(
		text.NewRow(16, "WEEKLY TIMETABLE", props.Text{
			Size: 16, Style: fontstyle.Bold, Align: align.Center, Top: 4,
		}),
		text.NewRow(10, data.ClassName, props.Text{
			Size: 12, Align: align.Center,
		}),
		text.NewRow(8, "Generated: "+time.Now().Format("02 Jan 2006"), props.Text{
			Size: 9, Align: align.Center,
			Color: &props.Color{Red: 120, Green: 120, Blue: 120},
		}),
		text.NewRow(4, "", props.Text{}),
	)

	// Group slots by day
	byDay := make(map[int][]Slot)
	for _, sl := range data.Slots {
		byDay[sl.DayOfWeek] = append(byDay[sl.DayOfWeek], sl)
	}

	totalSlots := 0
	for day := 0; day < 6; day++ {
		slots := byDay[day]
		if len(slots) == 0 {
			continue
		}
		totalSlots += len(slots)

		// Day header
		m.AddRow(8,
			text.NewCol(12, dayNames[day], props.Text{
				Size: 10, Style: fontstyle.Bold, Top: 1,
				Color: &props.Color{Red: 30, Green: 80, Blue: 160},
			}),
		)
		// Column header
		m.AddRow(6,
			text.NewCol(1, "P#", props.Text{Size: 7, Style: fontstyle.Bold, Align: align.Center}),
			text.NewCol(5, "Subject", props.Text{Size: 7, Style: fontstyle.Bold}),
			text.NewCol(3, "Time", props.Text{Size: 7, Style: fontstyle.Bold, Align: align.Center}),
			text.NewCol(3, "Room", props.Text{Size: 7, Style: fontstyle.Bold, Align: align.Center}),
		)
		for _, sl := range slots {
			timeStr := "—"
			if sl.StartTime != nil {
				timeStr = fmtTime(*sl.StartTime)
				if sl.EndTime != nil {
					timeStr += "–" + fmtTime(*sl.EndTime)
				}
			}
			roomStr := "—"
			if sl.RoomNumber != nil {
				roomStr = *sl.RoomNumber
			}
			m.AddRow(7,
				text.NewCol(1, fmt.Sprintf("%d", sl.PeriodNumber), props.Text{Size: 8, Align: align.Center}),
				text.NewCol(5, sl.SubjectName, props.Text{Size: 8, Style: fontstyle.Bold}),
				text.NewCol(3, timeStr, props.Text{Size: 7, Align: align.Center}),
				text.NewCol(3, roomStr, props.Text{Size: 8, Align: align.Center}),
			)
		}
		m.AddRows(text.NewRow(3, "", props.Text{}))
	}

	if totalSlots == 0 {
		m.AddRows(text.NewRow(12, "No periods scheduled.", props.Text{
			Align: align.Center, Top: 4,
			Color: &props.Color{Red: 150, Green: 150, Blue: 150},
		}))
	}

	m.AddRows(
		text.NewRow(6, "", props.Text{}),
		text.NewRow(8, fmt.Sprintf("Total periods: %d", totalSlots),
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

func fmtTime(s string) string {
	t, err := time.Parse("15:04:05", s)
	if err != nil {
		t, err = time.Parse("15:04", s)
		if err != nil {
			return s
		}
	}
	return t.Format("3:04PM")
}
