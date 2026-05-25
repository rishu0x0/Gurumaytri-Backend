package reports

import (
	"bytes"
	"fmt"

	"github.com/xuri/excelize/v2"
)

// GenerateExcel generates an xlsx attendance report and returns raw bytes.
func GenerateExcel(data ReportData) ([]byte, error) {
	f := excelize.NewFile()

	f.SetSheetName("Sheet1", "Attendance Data")
	f.NewSheet("Summary")

	if err := writeAttendanceSheet(f, data); err != nil {
		return nil, err
	}
	if err := writeSummarySheet(f, data); err != nil {
		return nil, err
	}

	sheetIdx, _ := f.GetSheetIndex("Attendance Data")
	f.SetActiveSheet(sheetIdx)

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("excel write: %w", err)
	}
	return buf.Bytes(), nil
}

func writeAttendanceSheet(f *excelize.File, data ReportData) error {
	sheet := "Attendance Data"

	// Title
	f.SetCellValue(sheet, "A1", data.ClassName+" — Attendance "+data.Month)

	// Header row (row 2)
	headers := []interface{}{"Student Name", "Roll"}
	for _, d := range data.Days {
		headers = append(headers, fmt.Sprintf("%d", d))
	}
	for i, h := range headers {
		cell, err := excelize.CoordinatesToCellName(i+1, 2)
		if err != nil {
			return err
		}
		f.SetCellValue(sheet, cell, h)
	}

	// Bold header style
	boldStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"D3D3D3"}, Pattern: 1},
	})
	if err != nil {
		return err
	}
	endHeaderCell, _ := excelize.CoordinatesToCellName(len(headers), 2)
	f.SetCellStyle(sheet, "A2", endHeaderCell, boldStyle)

	// Cell styles for P/A/L
	greenStyle, _ := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Color: []string{"C8E6C9"}, Pattern: 1},
	})
	redStyle, _ := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Color: []string{"FFCDD2"}, Pattern: 1},
	})
	amberStyle, _ := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{Type: "pattern", Color: []string{"FFF9C4"}, Pattern: 1},
	})

	// Student rows start at row 3
	for rowIdx, stu := range data.Students {
		excelRow := rowIdx + 3
		nameCell, _ := excelize.CoordinatesToCellName(1, excelRow)
		rollCell, _ := excelize.CoordinatesToCellName(2, excelRow)
		f.SetCellValue(sheet, nameCell, stu.Name)
		f.SetCellValue(sheet, rollCell, stu.RollNumber)

		for colIdx, d := range data.Days {
			label := stu.Records[d]
			if label == "" {
				label = "—"
			}
			cell, _ := excelize.CoordinatesToCellName(colIdx+3, excelRow)
			f.SetCellValue(sheet, cell, label)
			switch label {
			case "P":
				f.SetCellStyle(sheet, cell, cell, greenStyle)
			case "A":
				f.SetCellStyle(sheet, cell, cell, redStyle)
			case "L":
				f.SetCellStyle(sheet, cell, cell, amberStyle)
			}
		}
	}

	f.SetColWidth(sheet, "A", "A", 24)
	f.SetColWidth(sheet, "B", "B", 6)
	if len(data.Days) > 0 {
		endColName, _ := excelize.ColumnNumberToName(len(data.Days) + 2)
		f.SetColWidth(sheet, "C", endColName, 4)
	}

	return nil
}

func writeSummarySheet(f *excelize.File, data ReportData) error {
	sheet := "Summary"

	headers := []interface{}{"Student Name", "Roll", "Present", "Absent", "Leave", "Total Days", "%"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	boldStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"D3D3D3"}, Pattern: 1},
	})
	f.SetCellStyle(sheet, "A1", "G1", boldStyle)

	redFontStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Color: "D32F2F"},
	})

	for rowIdx, stu := range data.Students {
		excelRow := rowIdx + 2
		pct := 0.0
		if stu.Total > 0 {
			pct = float64(stu.Present) / float64(stu.Total) * 100
		}
		values := []interface{}{
			stu.Name,
			stu.RollNumber,
			stu.Present,
			stu.Absent,
			stu.Leave,
			stu.Total,
			fmt.Sprintf("%.1f%%", pct),
		}
		for colIdx, v := range values {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, excelRow)
			f.SetCellValue(sheet, cell, v)
		}
		if pct < 75.0 && stu.Total > 0 {
			f.SetCellStyle(sheet, fmt.Sprintf("A%d", excelRow), fmt.Sprintf("G%d", excelRow), redFontStyle)
		}
	}

	f.SetColWidth(sheet, "A", "A", 24)
	f.SetColWidth(sheet, "B", "B", 6)
	f.SetColWidth(sheet, "C", "G", 12)

	return nil
}
