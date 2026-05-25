package students

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"school-management/backend/internal/auth"
)

// Handler holds the students service and exposes HTTP handler methods.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// List handles GET /classes/{classID}/students
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	teacherID := auth.TeacherIDFromContext(r.Context())

	list, err := h.svc.List(r.Context(), classID, teacherID)
	if err != nil {
		if errors.Is(err, ErrClassNotFound) {
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list students")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"students": list})
}

type createRequest struct {
	FullName   string `json:"full_name"`
	RollNumber *int   `json:"roll_number"`
	PhotoURL   string `json:"photo_url"`
}

// Create handles POST /classes/{classID}/students
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}
	if req.FullName == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "full_name is required")
		return
	}

	teacherID := auth.TeacherIDFromContext(r.Context())
	student, err := h.svc.Create(r.Context(), classID, teacherID, req.FullName, req.RollNumber, req.PhotoURL)
	if err != nil {
		if errors.Is(err, ErrClassNotFound) {
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
			return
		}
		if errors.Is(err, ErrDuplicateRoll) {
			writeError(w, http.StatusConflict, "duplicate_roll", "A student with this roll number already exists in this class")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create student")
		return
	}
	writeJSON(w, http.StatusCreated, student)
}

type updateRequest struct {
	FullName   *string `json:"full_name"`
	RollNumber *int    `json:"roll_number"`
	PhotoURL   *string `json:"photo_url"`
}

// Update handles PUT /classes/{classID}/students/{studentID}
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	studentID := chi.URLParam(r, "studentID")
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}

	teacherID := auth.TeacherIDFromContext(r.Context())
	student, err := h.svc.Update(r.Context(), classID, teacherID, studentID, req.FullName, req.RollNumber, req.PhotoURL)
	if err != nil {
		if errors.Is(err, ErrClassNotFound) {
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
			return
		}
		if errors.Is(err, ErrStudentNotFound) {
			writeError(w, http.StatusNotFound, "student_not_found", "Student not found")
			return
		}
		if errors.Is(err, ErrDuplicateRoll) {
			writeError(w, http.StatusConflict, "duplicate_roll", "A student with this roll number already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update student")
		return
	}
	writeJSON(w, http.StatusOK, student)
}

// SoftRemove handles DELETE /classes/{classID}/students/{studentID}
// Sets is_active=false — the row is retained so attendance history keeps its student references.
func (h *Handler) SoftRemove(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	studentID := chi.URLParam(r, "studentID")
	teacherID := auth.TeacherIDFromContext(r.Context())

	if err := h.svc.SoftRemove(r.Context(), classID, teacherID, studentID); err != nil {
		if errors.Is(err, ErrClassNotFound) {
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
			return
		}
		if errors.Is(err, ErrStudentNotFound) {
			writeError(w, http.StatusNotFound, "student_not_found", "Student not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to remove student")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"removed": true})
}

type seedRequest struct {
	Count int `json:"count"`
}

// Seed handles POST /classes/{classID}/students/seed
// Accepts count in JSON body or ?count= query param; defaults to 30.
func (h *Handler) Seed(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	var req seedRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Count <= 0 {
		if n, err := strconv.Atoi(r.URL.Query().Get("count")); err == nil && n > 0 {
			req.Count = n
		} else {
			req.Count = 30
		}
	}

	teacherID := auth.TeacherIDFromContext(r.Context())
	created, err := h.svc.Seed(r.Context(), classID, teacherID, req.Count)
	if err != nil {
		if errors.Is(err, ErrClassNotFound) {
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to seed students")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]int{"created": created})
}

// writeJSON serialises v as JSON and writes status.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

// writeError writes a standard error envelope.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}
