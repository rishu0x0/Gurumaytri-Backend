package apilog

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

// actionMap maps "METHOD /route/pattern" → human-readable action description.
var actionMap = map[string]string{
	// Auth
	"POST /auth/register":              "Register teacher account",
	"POST /auth/register/send-otp":     "Send registration OTP",
	"POST /auth/register/verify-otp":   "Verify registration OTP",
	"POST /auth/otp/retry":             "Retry OTP",
	"POST /auth/login":                 "Teacher login",
	"POST /auth/refresh":               "Refresh access token",
	"POST /auth/logout":                "Teacher logout",
	// System
	"GET /health": "Health check",
	// Classes
	"GET /classes":              "List classes",
	"POST /classes":             "Create class",
	"PUT /classes/{classID}":    "Update class",
	"DELETE /classes/{classID}": "Delete class",
	// Students
	"GET /classes/{classID}/students":                           "List students",
	"POST /classes/{classID}/students":                          "Add student",
	"PUT /classes/{classID}/students/{studentID}":               "Update student",
	"DELETE /classes/{classID}/students/{studentID}":            "Remove student",
	"POST /classes/{classID}/students/seed":                     "Generate test students",
	// Attendance
	"GET /classes/{classID}/attendance":                  "Get attendance by date",
	"POST /classes/{classID}/attendance":                 "Submit attendance batch",
	"PUT /classes/{classID}/attendance/{sessionID}":      "Edit attendance records",
	// Stats
	"GET /classes/{classID}/stats/today":   "Get today's attendance stats",
	"GET /classes/{classID}/stats/monthly": "Get monthly attendance stats",
	// Reports
	"POST /reports/generate":             "Generate attendance report",
	"GET /reports":                       "List reports",
	"GET /reports/{reportID}/status":     "Check report status",
	// Subjects (marksheet)
	"GET /classes/{classID}/subjects":                           "List subjects",
	"POST /classes/{classID}/subjects":                          "Create subject",
	"PUT /classes/{classID}/subjects/{subjectID}":               "Update subject",
	"DELETE /classes/{classID}/subjects/{subjectID}":            "Delete subject",
	// Exams (marksheet)
	"GET /classes/{classID}/exams":                              "List exams",
	"POST /classes/{classID}/exams":                             "Create exam",
	"DELETE /classes/{classID}/exams/{examID}":                  "Delete exam",
	"GET /classes/{classID}/exams/{examID}/marks":               "Get marks grid",
	"POST /classes/{classID}/exams/{examID}/marks":              "Save marks",
	"GET /classes/{classID}/exams/{examID}/marksheet":           "Download marksheet PDF",
	// Examination
	"GET /classes/{classID}/exam-events":                                       "List exam events",
	"POST /classes/{classID}/exam-events":                                      "Create exam event",
	"DELETE /classes/{classID}/exam-events/{eventID}":                          "Delete exam event",
	"GET /classes/{classID}/exam-events/{eventID}/slots":                       "List exam slots",
	"POST /classes/{classID}/exam-events/{eventID}/slots":                      "Add exam slot",
	"PUT /classes/{classID}/exam-events/{eventID}/slots/{slotID}":              "Update exam slot",
	"DELETE /classes/{classID}/exam-events/{eventID}/slots/{slotID}":           "Delete exam slot",
	"GET /classes/{classID}/exam-events/{eventID}/timetable":                   "Download timetable PDF",
	// Fees
	"GET /classes/{classID}/fee-structures":                           "List fee structures",
	"POST /classes/{classID}/fee-structures":                          "Create fee structure",
	"PUT /classes/{classID}/fee-structures/{structureID}":             "Update fee structure",
	"DELETE /classes/{classID}/fee-structures/{structureID}":          "Delete fee structure",
	"GET /classes/{classID}/fee-structures/{structureID}/payments":    "Get fee payments",
	"POST /classes/{classID}/fee-structures/{structureID}/payments":   "Record payment",
}

// responseWriter wraps http.ResponseWriter to capture the status code and
// the first 512 bytes of the response body without blocking the client.
type responseWriter struct {
	http.ResponseWriter
	status int
	buf    bytes.Buffer
	capped bool
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.capped {
		remaining := 512 - rw.buf.Len()
		if len(b) >= remaining {
			rw.buf.Write(b[:remaining])
			rw.capped = true
		} else {
			rw.buf.Write(b)
		}
	}
	return rw.ResponseWriter.Write(b)
}

// GetTeacherIDFn is a function that extracts the authenticated teacher ID
// from a request. Returns "" for unauthenticated requests.
type GetTeacherIDFn func(r *http.Request) string

// Middleware returns a Chi-compatible middleware that logs every HTTP request
// to the api_logs table asynchronously.
func Middleware(pool *pgxpool.Pool, getTeacherID GetTeacherIDFn) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rec, r)

			duration := time.Since(start).Milliseconds()

			// Capture all fields after handler returns (route context is populated now)
			routePattern := ""
			if rctx := chi.RouteContext(r.Context()); rctx != nil {
				routePattern = rctx.RoutePattern()
			}
			if routePattern == "" {
				routePattern = r.URL.Path
			}

			method := r.Method
			actionKey := method + " " + routePattern
			action := actionMap[actionKey]

			teacherID := getTeacherID(r)
			deviceID := r.Header.Get("X-Device-ID")
			requestID := middleware.GetReqID(r.Context())

			status := "success"
			if rec.status >= 400 {
				status = "failed"
			}

			entry := logEntry{
				RequestID:  nullableStr(requestID),
				DeviceID:   nullableStr(deviceID),
				TeacherID:  nullableStr(teacherID),
				Method:     method,
				Endpoint:   routePattern,
				Action:     nullableStr(action),
				StatusCode: rec.status,
				Status:     status,
				Message:    nullableStr(extractMessage(rec.buf.Bytes(), rec.status)),
				IPAddress:  nullableStr(realIP(r)),
				UserAgent:  nullableStr(truncate(r.Header.Get("User-Agent"), 500)),
				DurationMS: duration,
			}

			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := writeLog(ctx, pool, entry); err != nil {
					log.Printf("[apilog] write failed: %v", err)
				}
			}()
		})
	}
}

type logEntry struct {
	RequestID  *string
	DeviceID   *string
	TeacherID  *string
	Method     string
	Endpoint   string
	Action     *string
	StatusCode int
	Status     string
	Message    *string
	IPAddress  *string
	UserAgent  *string
	DurationMS int64
}

func writeLog(ctx context.Context, pool *pgxpool.Pool, e logEntry) error {
	_, err := pool.Exec(ctx, `
		INSERT INTO api_logs
			(request_id, device_id, teacher_id, method, endpoint, action,
			 status_code, status, message, ip_address, user_agent, duration_ms)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		e.RequestID, e.DeviceID, e.TeacherID, e.Method, e.Endpoint, e.Action,
		e.StatusCode, e.Status, e.Message, e.IPAddress, e.UserAgent, e.DurationMS,
	)
	return err
}

// extractMessage pulls a human-readable message out of a JSON response body.
func extractMessage(body []byte, status int) string {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		if status < 400 {
			return "OK"
		}
		return "Error"
	}
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		s := string(body)
		return truncate(strings.TrimSpace(s), 300)
	}
	for _, key := range []string{"message", "error", "msg", "status", "detail"} {
		if v, ok := data[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	if status < 400 {
		return "Success"
	}
	return "Error"
}

func realIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.SplitN(xff, ",", 2)[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func nullableStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
