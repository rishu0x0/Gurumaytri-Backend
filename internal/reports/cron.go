package reports

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"
)

const maxConcurrentReports = 10

// StartCron starts the monthly report generation cron and 90-day cleanup cron.
// Returns a stop function to gracefully shut down the scheduler.
func StartCron(pool *pgxpool.Pool, svc *ReportService) func() {
	c := cron.New()

	// 35 18 1 * * UTC = 00:05 IST on the 1st of every month
	c.AddFunc("35 18 1 * *", func() { //nolint:errcheck
		log.Println("cron: starting monthly report generation")
		if err := generateMonthlyReports(pool, svc); err != nil {
			log.Printf("cron: monthly reports error: %v", err)
		}
	})

	// Cleanup expired reports daily at 02:00 UTC
	c.AddFunc("0 2 * * *", func() { //nolint:errcheck
		log.Println("cron: cleaning up expired reports")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := svc.DeleteExpired(ctx); err != nil {
			log.Printf("cron: cleanup error: %v", err)
		}
	})

	c.Start()
	return func() { <-c.Stop().Done() }
}

func generateMonthlyReports(pool *pgxpool.Pool, svc *ReportService) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Previous month in YYYY-MM
	now := time.Now().UTC()
	prev := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC)
	month := prev.Format("2006-01")

	// Fetch all (teacher_id, class_id) pairs with attendance in the previous month
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT c.teacher_id::text, s.class_id::text
		FROM attendance_sessions s
		JOIN classes c ON c.id = s.class_id
		WHERE TO_CHAR(s.date, 'YYYY-MM') = $1`, month)
	if err != nil {
		return err
	}
	type job struct {
		teacherID string
		classID   string
	}
	var jobs []job
	for rows.Next() {
		var j job
		if err := rows.Scan(&j.teacherID, &j.classID); err != nil {
			rows.Close()
			return err
		}
		jobs = append(jobs, j)
	}
	rows.Close()

	sem := make(chan struct{}, maxConcurrentReports)
	var wg sync.WaitGroup

	for _, j := range jobs {
		for _, format := range []string{"pdf", "excel"} {
			wg.Add(1)
			sem <- struct{}{}
			go func(teacherID, classID, fmt string) {
				defer wg.Done()
				defer func() { <-sem }()

				reportID, err := svc.CreateReportRow(ctx, teacherID, classID, month, fmt)
				if err != nil {
					log.Printf("cron: create report row (%s/%s/%s): %v", classID, month, fmt, err)
					return
				}
				if err := svc.GenerateReport(ctx, reportID, classID, teacherID, month, fmt); err != nil {
					log.Printf("cron: generate report %s: %v", reportID, err)
				}
			}(j.teacherID, j.classID, format)
		}
	}
	wg.Wait()
	log.Printf("cron: monthly reports done for %s (%d classes)", month, len(jobs))
	return nil
}
