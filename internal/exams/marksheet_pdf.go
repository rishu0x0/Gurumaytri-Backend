package exams

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

func grade(pct float64) string {
	switch {
	case pct >= 90:
		return "A+"
	case pct >= 80:
		return "A"
	case pct >= 70:
		return "B+"
	case pct >= 60:
		return "B"
	case pct >= 50:
		return "C"
	case pct >= 40:
		return "D"
	default:
		return "F"
	}
}

// GenerateMarksheetPDF builds a landscape PDF marksheet and returns raw bytes.
func GenerateMarksheetPDF(data *MarksheetData) ([]byte, error) {
	cfg := config.NewBuilder().
		WithPageSize(pagesize.A4).
		WithOrientation(orientation.Horizontal).
		WithLeftMargin(8).WithRightMargin(8).
		WithTopMargin(10).WithBottomMargin(10).
		Build()

	m := maroto.New(cfg)

	dateStr := ""
	if data.ExamDate != nil {
		dateStr = "  |  " + *data.ExamDate
	}

	m.AddRows(
		text.NewRow(14, data.ClassName, props.Text{
			Size: 16, Style: fontstyle.Bold, Align: align.Center, Top: 4,
		}),
		text.NewRow(10, data.ExamName+dateStr, props.Text{
			Size: 12, Align: align.Center,
		}),
		text.NewRow(6, "Marksheet", props.Text{
			Size:  9,
			Align: align.Center,
			Color: &props.Color{Red: 100, Green: 100, Blue: 100},
		}),
	)

	addMarksheetTable(m, data)

	doc, err := m.Generate()
	if err != nil {
		return nil, fmt.Errorf("marksheet pdf generate: %w", err)
	}
	return doc.GetBytes(), nil
}

func addMarksheetTable(m core.Maroto, data *MarksheetData) {
	subjects := data.Grid.Subjects
	students := data.Grid.Students

	if len(students) == 0 {
		m.AddRows(text.NewRow(12, "No students found.", props.Text{Align: align.Center, Top: 4}))
		return
	}
	if len(subjects) == 0 {
		m.AddRows(text.NewRow(12, "No subjects defined for this class.", props.Text{Align: align.Center, Top: 4}))
		return
	}

	// Build mark lookup: "studentID:subjectID" → MarkEntry
	markMap := map[string]MarkEntry{}
	for _, mk := range data.Grid.Marks {
		markMap[mk.StudentID+":"+mk.SubjectID] = mk
	}

	// 12-col grid: Name=3, Roll=1, up to 5 subject cols, Total=1, %=1, Grade=1
	maxSubjectCols := 5
	subjectsToShow := subjects
	if len(subjectsToShow) > maxSubjectCols {
		subjectsToShow = subjectsToShow[:maxSubjectCols]
	}

	subColSize := 1

	buildHeader := func() {
		cols := []core.Col{
			text.NewCol(3, "Student Name", props.Text{Size: 7, Style: fontstyle.Bold}),
			text.NewCol(1, "Roll", props.Text{Size: 7, Style: fontstyle.Bold, Align: align.Center}),
		}
		for _, sub := range subjectsToShow {
			label := sub.Name
			if len(label) > 8 {
				label = label[:8]
			}
			cols = append(cols, text.NewCol(subColSize, label,
				props.Text{Size: 6, Style: fontstyle.Bold, Align: align.Center}))
		}
		cols = append(cols,
			text.NewCol(1, "Total", props.Text{Size: 7, Style: fontstyle.Bold, Align: align.Center}),
			text.NewCol(1, "%", props.Text{Size: 7, Style: fontstyle.Bold, Align: align.Center}),
			text.NewCol(1, "Grade", props.Text{Size: 7, Style: fontstyle.Bold, Align: align.Center}),
		)
		m.AddRow(7, cols...)
	}

	buildHeader()

	for i, stu := range students {
		if i > 0 && i%30 == 0 {
			buildHeader()
		}

		var totalObtained, totalMax float64
		allAbsent := true
		subCols := []core.Col{}

		for _, sub := range subjectsToShow {
			key := stu.ID + ":" + sub.ID
			mk, ok := markMap[key]
			var cellText string
			if !ok {
				cellText = "—"
			} else if mk.IsAbsent {
				cellText = "AB"
			} else if mk.Obtained != nil {
				cellText = fmt.Sprintf("%.0f", *mk.Obtained)
				totalObtained += *mk.Obtained
				totalMax += float64(sub.MaxMarks)
				allAbsent = false
			} else {
				cellText = "—"
			}
			subCols = append(subCols, text.NewCol(subColSize, cellText,
				props.Text{Size: 6, Align: align.Center}))
		}

		// Also accumulate total for subjects not shown
		for _, sub := range subjects[len(subjectsToShow):] {
			key := stu.ID + ":" + sub.ID
			if mk, ok := markMap[key]; ok && !mk.IsAbsent && mk.Obtained != nil {
				totalObtained += *mk.Obtained
				totalMax += float64(sub.MaxMarks)
				allAbsent = false
			} else if ok && !mk.IsAbsent {
				totalMax += float64(sub.MaxMarks)
			}
		}

		totalStr := "—"
		pctStr := "—"
		gradeStr := "—"
		if !allAbsent && totalMax > 0 {
			totalStr = fmt.Sprintf("%.0f/%.0f", totalObtained, totalMax)
			pct := totalObtained / totalMax * 100
			pctStr = fmt.Sprintf("%.1f", pct)
			gradeStr = grade(pct)
		}

		cols := []core.Col{
			text.NewCol(3, stu.FullName, props.Text{Size: 7}),
			text.NewCol(1, fmt.Sprintf("%d", stu.RollNumber), props.Text{Size: 7, Align: align.Center}),
		}
		cols = append(cols, subCols...)
		cols = append(cols,
			text.NewCol(1, totalStr, props.Text{Size: 6, Align: align.Center}),
			text.NewCol(1, pctStr, props.Text{Size: 6, Align: align.Center}),
			text.NewCol(1, gradeStr, props.Text{Size: 7, Style: fontstyle.Bold, Align: align.Center}),
		)
		m.AddRow(6, cols...)
	}
}
