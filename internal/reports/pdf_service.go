package reports

import (
	"fmt"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/consts/orientation"
	"github.com/johnfercher/maroto/v2/pkg/consts/pagesize"
	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/johnfercher/maroto/v2/pkg/props"
)

const studentsPerPage = 25

// GeneratePDF generates a PDF attendance report and returns raw bytes.
func GeneratePDF(data ReportData) ([]byte, error) {
	cfg := config.NewBuilder().
		WithPageSize(pagesize.A4).
		WithOrientation(orientation.Horizontal).
		WithLeftMargin(8).
		WithRightMargin(8).
		WithTopMargin(10).
		WithBottomMargin(10).
		Build()

	m := maroto.New(cfg)

	addPDFCoverPage(m, data)
	addPDFDataPages(m, data)
	addPDFSummaryPage(m, data)

	doc, err := m.Generate()
	if err != nil {
		return nil, fmt.Errorf("pdf generate: %w", err)
	}
	return doc.GetBytes(), nil
}

func addPDFCoverPage(m core.Maroto, data ReportData) {
	m.AddRows(
		text.NewRow(40, data.ClassName, props.Text{
			Size:  20,
			Style: fontstyle.Bold,
			Align: align.Center,
			Top:   15,
		}),
		text.NewRow(12, "Attendance Report — "+data.Month, props.Text{
			Size:  14,
			Align: align.Center,
		}),
		text.NewRow(8, "Generated: "+data.GeneratedAt.Format("02 Jan 2006"), props.Text{
			Size:  10,
			Align: align.Center,
			Color: &props.Color{Red: 100, Green: 100, Blue: 100},
		}),
		text.NewRow(8,
			fmt.Sprintf("Students: %d   |   Days Recorded: %d", len(data.Students), len(data.Days)),
			props.Text{Size: 10, Align: align.Center},
		),
	)
}

func addPDFDataPages(m core.Maroto, data ReportData) {
	if len(data.Students) == 0 || len(data.Days) == 0 {
		return
	}

	// Header row for each data page
	buildHeader := func() {
		// 12-grid: Name=3, Roll=1, remaining 8 cols for up to 31 days
		// For simplicity, display max 31 days; columns may be very narrow
		dayColSize := 1
		nameCol := text.NewCol(3, "Student Name", props.Text{Size: 7, Style: fontstyle.Bold})
		rollCol := text.NewCol(1, "Roll", props.Text{Size: 7, Style: fontstyle.Bold, Align: align.Center})

		cols := []core.Col{nameCol, rollCol}
		for _, d := range data.Days {
			cols = append(cols, text.NewCol(dayColSize, fmt.Sprintf("%d", d),
				props.Text{Size: 6, Style: fontstyle.Bold, Align: align.Center}))
		}
		// Only add if we have room (12-grid, name=3, roll=1, so up to 8 day cols)
		// If more days than grid allows, we trim display
		maxDayCols := 8
		if len(data.Days) <= maxDayCols {
			m.AddRow(7, cols...)
		} else {
			// Trim to max and add header
			m.AddRow(7, cols[:2+maxDayCols]...)
		}
	}

	buildHeader()

	for i, stu := range data.Students {
		if i > 0 && i%studentsPerPage == 0 {
			buildHeader()
		}

		nameCol := text.NewCol(3, stu.Name, props.Text{Size: 7})
		rollCol := text.NewCol(1, fmt.Sprintf("%d", stu.RollNumber),
			props.Text{Size: 7, Align: align.Center})

		cols := []core.Col{nameCol, rollCol}
		maxDayCols := 8
		daysToShow := data.Days
		if len(daysToShow) > maxDayCols {
			daysToShow = daysToShow[:maxDayCols]
		}
		for _, d := range daysToShow {
			label := stu.Records[d]
			if label == "" {
				label = "—"
			}
			cols = append(cols, text.NewCol(1, label,
				props.Text{Size: 6, Align: align.Center}))
		}
		m.AddRow(6, cols...)
	}
}

func addPDFSummaryPage(m core.Maroto, data ReportData) {
	m.AddRows(
		text.NewRow(12, "Summary", props.Text{
			Size:  12,
			Style: fontstyle.Bold,
			Top:   4,
		}),
	)

	// Header
	m.AddRow(7,
		text.NewCol(4, "Student Name", props.Text{Size: 8, Style: fontstyle.Bold}),
		text.NewCol(1, "Roll", props.Text{Size: 8, Style: fontstyle.Bold, Align: align.Center}),
		text.NewCol(1, "P", props.Text{Size: 8, Style: fontstyle.Bold, Align: align.Center}),
		text.NewCol(1, "A", props.Text{Size: 8, Style: fontstyle.Bold, Align: align.Center}),
		text.NewCol(1, "L", props.Text{Size: 8, Style: fontstyle.Bold, Align: align.Center}),
		text.NewCol(2, "Days", props.Text{Size: 8, Style: fontstyle.Bold, Align: align.Center}),
		text.NewCol(2, "%", props.Text{Size: 8, Style: fontstyle.Bold, Align: align.Center}),
	)

	for _, stu := range data.Students {
		pct := 0.0
		if stu.Total > 0 {
			pct = float64(stu.Present) / float64(stu.Total) * 100
		}
		m.AddRow(6,
			text.NewCol(4, stu.Name, props.Text{Size: 7}),
			text.NewCol(1, fmt.Sprintf("%d", stu.RollNumber), props.Text{Size: 7, Align: align.Center}),
			text.NewCol(1, fmt.Sprintf("%d", stu.Present), props.Text{Size: 7, Align: align.Center}),
			text.NewCol(1, fmt.Sprintf("%d", stu.Absent), props.Text{Size: 7, Align: align.Center}),
			text.NewCol(1, fmt.Sprintf("%d", stu.Leave), props.Text{Size: 7, Align: align.Center}),
			text.NewCol(2, fmt.Sprintf("%d", stu.Total), props.Text{Size: 7, Align: align.Center}),
			text.NewCol(2, fmt.Sprintf("%.1f%%", pct), props.Text{Size: 7, Align: align.Center}),
		)
	}
}
