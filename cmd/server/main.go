package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"school-management/backend/internal/apilog"
	"school-management/backend/internal/admission"
	"school-management/backend/internal/attendance"
	"school-management/backend/internal/homework"
	"school-management/backend/internal/library"
	"school-management/backend/internal/auth"
	"school-management/backend/internal/classes"
	"school-management/backend/internal/config"
	"school-management/backend/internal/db"
	"school-management/backend/internal/examination"
	"school-management/backend/internal/exams"
	"school-management/backend/internal/fees"
	"school-management/backend/internal/timetable"
	"school-management/backend/internal/msg91"
	"school-management/backend/internal/reports"
	"school-management/backend/internal/stats"
	"school-management/backend/internal/students"
	"school-management/backend/internal/subjects"
	jwtpkg "school-management/backend/pkg/jwt"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()
	pool := db.NewPool(cfg.DatabaseURL)
	defer pool.Close()

	jwtSvc := jwtpkg.New(cfg.JWTSecret, cfg.JWTAccessExpiry)
	msg91Client := msg91.New(cfg.MSG91AuthToken, cfg.MSG91WidgetID)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(apilog.Middleware(pool, func(r *http.Request) string {
		return auth.TeacherIDFromContext(r.Context())
	}))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	authSvc := auth.NewService(pool, jwtSvc, msg91Client)
	authHandler := auth.NewHandler(authSvc)

	classSvc := classes.NewService(pool)
	classHandler := classes.NewHandler(classSvc)

	studentSvc := students.NewService(pool)
	studentHandler := students.NewHandler(studentSvc)

	attendanceSvc := attendance.NewService(pool)
	attendanceHandler := attendance.NewHandler(attendanceSvc)

	statsSvc := stats.NewService(pool)
	statsHandler := stats.NewHandler(statsSvc)

	subjectSvc := subjects.NewService(pool)
	subjectHandler := subjects.NewHandler(subjectSvc)

	examSvc := exams.NewService(pool)
	examHandler := exams.NewHandler(examSvc)

	feeSvc := fees.NewService(pool)
	feeHandler := fees.NewHandler(feeSvc)

	examEventSvc := examination.NewService(pool)
	examEventHandler := examination.NewHandler(examEventSvc)

	timetableSvc := timetable.NewService(pool)
	timetableHandler := timetable.NewHandler(timetableSvc)

	admissionSvc := admission.NewService(pool)
	admissionHandler := admission.NewHandler(admissionSvc)

	librarySvc := library.NewService(pool)
	libraryHandler := library.NewHandler(librarySvc)

	homeworkSvc := homework.NewService(pool)
	homeworkHandler := homework.NewHandler(homeworkSvc)

	storageClient := reports.NewStorageClient(cfg.SupabaseURL, cfg.SupabaseServiceRoleKey)
	reportSvc := reports.NewService(pool, storageClient)
	reportHandler := reports.NewHandler(reportSvc)
	stopCron := reports.StartCron(pool, reportSvc)
	defer stopCron()

	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", authHandler.Register)
		r.Post("/register/send-otp", authHandler.SendRegistrationOTP)
		r.Post("/register/verify-otp", authHandler.VerifyRegistrationOTP)
		r.Post("/otp/retry", authHandler.RetryOTP)
		r.Post("/login", authHandler.Login)
		r.Post("/refresh", authHandler.Refresh)
		r.Group(func(r chi.Router) {
			r.Use(auth.JWTMiddleware(jwtSvc))
			r.Post("/logout", authHandler.Logout)
		})
	})

	// Protected routes — JWT required for all endpoints below
	r.Group(func(r chi.Router) {
		r.Use(auth.JWTMiddleware(jwtSvc))

		// Classes CRUD
		r.Get("/classes", classHandler.List)
		r.Post("/classes", classHandler.Create)
		r.Put("/classes/{classID}", classHandler.Update)
		r.Delete("/classes/{classID}", classHandler.Delete)

		// Students CRUD (plan 03-02)
		r.Get("/classes/{classID}/students", studentHandler.List)
		r.Post("/classes/{classID}/students", studentHandler.Create)
		r.Put("/classes/{classID}/students/{studentID}", studentHandler.Update)
		r.Delete("/classes/{classID}/students/{studentID}", studentHandler.SoftRemove)
		r.Post("/classes/{classID}/students/seed", studentHandler.Seed)

		// Attendance (plan 03-03)
		r.Get("/classes/{classID}/attendance", attendanceHandler.GetByDate)
		r.Post("/classes/{classID}/attendance", attendanceHandler.SubmitBatch)
		r.Put("/classes/{classID}/attendance/{sessionID}", attendanceHandler.EditRecords)

		// Stats (plan 03-04)
		r.Get("/classes/{classID}/stats/today", statsHandler.Today)
		r.Get("/classes/{classID}/stats/monthly", statsHandler.Monthly)

		// Reports (plan 07-04)
		r.Post("/reports/generate", reportHandler.Generate)
		r.Get("/reports", reportHandler.List)
		r.Get("/reports/{reportID}/status", reportHandler.StatusCheck)

		// Subjects (marksheet)
		r.Get("/classes/{classID}/subjects", subjectHandler.List)
		r.Post("/classes/{classID}/subjects", subjectHandler.Create)
		r.Put("/classes/{classID}/subjects/{subjectID}", subjectHandler.Update)
		r.Delete("/classes/{classID}/subjects/{subjectID}", subjectHandler.Delete)

		// Exams + Marks (marksheet)
		r.Get("/classes/{classID}/exams", examHandler.List)
		r.Post("/classes/{classID}/exams", examHandler.Create)
		r.Delete("/classes/{classID}/exams/{examID}", examHandler.Delete)
		r.Get("/classes/{classID}/exams/{examID}/marks", examHandler.GetMarks)
		r.Post("/classes/{classID}/exams/{examID}/marks", examHandler.UpsertMarks)
		r.Get("/classes/{classID}/exams/{examID}/marksheet", examHandler.DownloadMarksheet)

		// Examination
		r.Get("/classes/{classID}/exam-events", examEventHandler.ListEvents)
		r.Post("/classes/{classID}/exam-events", examEventHandler.CreateEvent)
		r.Delete("/classes/{classID}/exam-events/{eventID}", examEventHandler.DeleteEvent)
		r.Get("/classes/{classID}/exam-events/{eventID}/slots", examEventHandler.ListSlots)
		r.Post("/classes/{classID}/exam-events/{eventID}/slots", examEventHandler.CreateSlot)
		r.Put("/classes/{classID}/exam-events/{eventID}/slots/{slotID}", examEventHandler.UpdateSlot)
		r.Delete("/classes/{classID}/exam-events/{eventID}/slots/{slotID}", examEventHandler.DeleteSlot)
		r.Get("/classes/{classID}/exam-events/{eventID}/timetable", examEventHandler.DownloadTimetable)

		// Homework
		r.Get("/classes/{classID}/assignments", homeworkHandler.List)
		r.Post("/classes/{classID}/assignments", homeworkHandler.Create)
		r.Put("/classes/{classID}/assignments/{assignmentID}", homeworkHandler.Update)
		r.Delete("/classes/{classID}/assignments/{assignmentID}", homeworkHandler.Delete)

		// Library
		r.Get("/books", libraryHandler.ListBooks)
		r.Post("/books", libraryHandler.CreateBook)
		r.Put("/books/{bookID}", libraryHandler.UpdateBook)
		r.Delete("/books/{bookID}", libraryHandler.DeleteBook)
		r.Get("/books/issues", libraryHandler.ListIssues)
		r.Post("/books/{bookID}/issues", libraryHandler.IssueBook)
		r.Patch("/books/issues/{issueID}/return", libraryHandler.ReturnBook)

		// Admission
		r.Get("/admissions", admissionHandler.List)
		r.Post("/admissions", admissionHandler.Create)
		r.Put("/admissions/{inquiryID}", admissionHandler.Update)
		r.Patch("/admissions/{inquiryID}/status", admissionHandler.UpdateStatus)
		r.Delete("/admissions/{inquiryID}", admissionHandler.Delete)

		// Timetable
		r.Get("/classes/{classID}/timetable", timetableHandler.ListSlots)
		r.Post("/classes/{classID}/timetable/slots", timetableHandler.UpsertSlot)
		r.Delete("/classes/{classID}/timetable/slots/{slotID}", timetableHandler.DeleteSlot)
		r.Get("/classes/{classID}/timetable/pdf", timetableHandler.DownloadTimetable)

		// Fees
		r.Get("/classes/{classID}/fee-structures", feeHandler.ListStructures)
		r.Post("/classes/{classID}/fee-structures", feeHandler.CreateStructure)
		r.Put("/classes/{classID}/fee-structures/{structureID}", feeHandler.UpdateStructure)
		r.Delete("/classes/{classID}/fee-structures/{structureID}", feeHandler.DeleteStructure)
		r.Get("/classes/{classID}/fee-structures/{structureID}/payments", feeHandler.GetPayments)
		r.Post("/classes/{classID}/fee-structures/{structureID}/payments", feeHandler.RecordPayment)
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("server starting on :%s (env=%s)", cfg.Port, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}
	log.Println("server stopped")
}
