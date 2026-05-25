package stats

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"school-management/backend/internal/auth"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Today handles GET /classes/{classID}/stats/today
// Returns present_count, absent_count, leave_count, total_count for today's IST date.
// Returns zeros with submitted=false if no session exists for today.
func (h *Handler) Today(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	teacherID := auth.TeacherIDFromContext(r.Context())

	result, err := h.svc.TodaySummary(r.Context(), classID, teacherID)
	if err != nil {
		if errors.Is(err, ErrClassNotFound) {
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to get today's stats")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Monthly handles GET /classes/{classID}/stats/monthly?month=YYYY-MM
// Returns days_recorded, average_percentage, and below_threshold list (students under 75%).
func (h *Handler) Monthly(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	month := r.URL.Query().Get("month")
	if month == "" {
		writeError(w, http.StatusBadRequest, "missing_month", "month query parameter is required (YYYY-MM)")
		return
	}

	teacherID := auth.TeacherIDFromContext(r.Context())
	result, err := h.svc.MonthlySummary(r.Context(), classID, teacherID, month)
	if err != nil {
		if errors.Is(err, ErrClassNotFound) {
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_month", "month must be in YYYY-MM format")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}
