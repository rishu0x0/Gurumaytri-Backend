package subjects

import (
	"encoding/json"
	"errors"
	"log"
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

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	teacherID := auth.TeacherIDFromContext(r.Context())
	subjects, err := h.svc.List(r.Context(), classID, teacherID)
	if err != nil {
		if errors.Is(err, ErrClassNotFound) {
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
			return
		}
		log.Printf("ERROR List subjects classID=%s: %v", classID, err)
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"subjects": subjects})
}

type createRequest struct {
	Name         string `json:"name"`
	MaxMarks     int    `json:"max_marks"`
	DisplayOrder int    `json:"display_order"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}
	teacherID := auth.TeacherIDFromContext(r.Context())
	sub, err := h.svc.Create(r.Context(), classID, teacherID, req.Name, req.MaxMarks, req.DisplayOrder)
	if err != nil {
		switch {
		case errors.Is(err, ErrClassNotFound):
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
		case errors.Is(err, ErrDuplicateName):
			writeError(w, http.StatusConflict, "duplicate_subject_name", "A subject with this name already exists in this class")
		default:
			log.Printf("ERROR Create subject classID=%s: %v", classID, err)
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, sub)
}

type updateRequest struct {
	Name     string `json:"name"`
	MaxMarks int    `json:"max_marks"`
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	subjectID := chi.URLParam(r, "subjectID")
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}
	teacherID := auth.TeacherIDFromContext(r.Context())
	sub, err := h.svc.Update(r.Context(), classID, teacherID, subjectID, req.Name, req.MaxMarks)
	if err != nil {
		switch {
		case errors.Is(err, ErrClassNotFound):
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
		case errors.Is(err, ErrSubjectNotFound):
			writeError(w, http.StatusNotFound, "subject_not_found", "Subject not found")
		case errors.Is(err, ErrDuplicateName):
			writeError(w, http.StatusConflict, "duplicate_subject_name", "A subject with this name already exists in this class")
		default:
			log.Printf("ERROR Update subject subjectID=%s: %v", subjectID, err)
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, sub)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	subjectID := chi.URLParam(r, "subjectID")
	teacherID := auth.TeacherIDFromContext(r.Context())
	if err := h.svc.Delete(r.Context(), classID, teacherID, subjectID); err != nil {
		switch {
		case errors.Is(err, ErrClassNotFound):
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
		case errors.Is(err, ErrSubjectNotFound):
			writeError(w, http.StatusNotFound, "subject_not_found", "Subject not found")
		default:
			log.Printf("ERROR Delete subject subjectID=%s: %v", subjectID, err)
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Subject deleted"})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}
