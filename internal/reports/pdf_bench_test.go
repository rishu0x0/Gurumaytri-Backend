package reports_test

import (
	"testing"
	"time"

	"school-management/backend/internal/reports"
)

func sampleReportData(students int) reports.ReportData {
	days := make([]int, 22) // ~22 school days in a month
	for i := range days {
		days[i] = i + 1
	}
	studs := make([]reports.StudentRow, students)
	statuses := []string{"P", "A", "L"}
	for i := range studs {
		recs := map[int]string{}
		for _, d := range days {
			recs[d] = statuses[d%3]
		}
		present := 0
		for _, v := range recs {
			if v == "P" {
				present++
			}
		}
		studs[i] = reports.StudentRow{
			Name:       "Student Name",
			RollNumber: i + 1,
			Records:    recs,
			Present:    present,
			Total:      len(days),
		}
	}
	return reports.ReportData{
		ClassName:   "Class 10A",
		Month:       "2026-05",
		GeneratedAt: time.Now(),
		Days:        days,
		Students:    studs,
	}
}

func BenchmarkGeneratePDF_30Students(b *testing.B) {
	data := sampleReportData(30)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := reports.GeneratePDF(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenerateExcel_30Students(b *testing.B) {
	data := sampleReportData(30)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := reports.GenerateExcel(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGeneratePDF_100Students(b *testing.B) {
	data := sampleReportData(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := reports.GeneratePDF(data); err != nil {
			b.Fatal(err)
		}
	}
}
