package attendance

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"school-management/backend/internal/auth"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GetByDate handles GET /classes/{classID}/attendance?date=YYYY-MM-DD
func (h *Handler) GetByDate(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		writeError(w, http.StatusBadRequest, "missing_date", "date query parameter is required (YYYY-MM-DD)")
		return
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_date", "date must be in YYYY-MM-DD format")
		return
	}

	teacherID := auth.TeacherIDFromContext(r.Context())
	session, err := h.svc.GetByDate(r.Context(), classID, teacherID, date)
	if err != nil {
		if errors.Is(err, ErrClassNotFound) {
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
			return
		}
		if errors.Is(err, ErrSessionNotFound) {
			// No session for this date — return null gracefully, not 404
			writeJSON(w, http.StatusOK, map[string]interface{}{"session": nil, "records": []struct{}{}})
			return
		}
		log.Printf("ERROR GetByDate classID=%s: %v", classID, err)
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, session)
}

type submitRequest struct {
	Date    string        `json:"date"`
	Records []BatchRecord `json:"records"`
}

// SubmitBatch handles POST /classes/{classID}/attendance
func (h *Handler) SubmitBatch(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	var req submitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}
	if req.Date == "" {
		writeError(w, http.StatusBadRequest, "missing_date", "date is required (YYYY-MM-DD)")
		return
	}
	if len(req.Records) == 0 {
		writeError(w, http.StatusBadRequest, "missing_records", "records array cannot be empty")
		return
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_date", "date must be in YYYY-MM-DD format")
		return
	}

	teacherID := auth.TeacherIDFromContext(r.Context())
	session, err := h.svc.SubmitBatch(r.Context(), classID, teacherID, date, req.Records)
	if err != nil {
		if errors.Is(err, ErrClassNotFound) {
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
			return
		}
		if errors.Is(err, ErrDuplicateSession) {
			writeError(w, http.StatusConflict, "duplicate_session", "Attendance for this class and date has already been submitted")
			return
		}
		if errors.Is(err, ErrInvalidStatus) {
			writeError(w, http.StatusBadRequest, "invalid_status", "Status must be one of: present, absent, leave")
			return
		}
		log.Printf("ERROR SubmitBatch classID=%s: %v", classID, err)
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

type editRequest struct {
	Records []BatchRecord `json:"records"`
}

// EditRecords handles PUT /classes/{classID}/attendance/{sessionID}
func (h *Handler) EditRecords(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	sessionID := chi.URLParam(r, "sessionID")
	var req editRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}
	if len(req.Records) == 0 {
		writeError(w, http.StatusBadRequest, "missing_records", "records array cannot be empty")
		return
	}

	teacherID := auth.TeacherIDFromContext(r.Context())
	session, err := h.svc.EditRecords(r.Context(), classID, teacherID, sessionID, req.Records)
	if err != nil {
		if errors.Is(err, ErrClassNotFound) {
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
			return
		}
		if errors.Is(err, ErrSessionNotFound) {
			writeError(w, http.StatusNotFound, "session_not_found", "Attendance session not found")
			return
		}
		if errors.Is(err, ErrAttendanceLocked) {
			writeError(w, http.StatusForbidden, "attendance_locked", "Attendance can only be edited on the day of submission before midnight IST")
			return
		}
		if errors.Is(err, ErrInvalidStatus) {
			writeError(w, http.StatusBadRequest, "invalid_status", "Status must be one of: present, absent, leave")
			return
		}
		log.Printf("ERROR EditRecords classID=%s sessionID=%s: %v", classID, sessionID, err)
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}
