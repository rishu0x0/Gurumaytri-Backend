package classes

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

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	teacherID := auth.TeacherIDFromContext(r.Context())
	classes, err := h.svc.List(r.Context(), teacherID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list classes")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"classes": classes})
}

type createRequest struct {
	Name    string `json:"name"`
	Section string `json:"section"`
	Subject string `json:"subject"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Class name is required")
		return
	}

	teacherID := auth.TeacherIDFromContext(r.Context())
	class, err := h.svc.Create(r.Context(), teacherID, req.Name, req.Section, req.Subject)
	if err != nil {
		if errors.Is(err, ErrDuplicateName) {
			writeError(w, http.StatusConflict, "duplicate_name", "A class with this name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create class")
		return
	}
	writeJSON(w, http.StatusCreated, class)
}

type updateRequest struct {
	Name string `json:"name"`
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Class name is required")
		return
	}

	teacherID := auth.TeacherIDFromContext(r.Context())
	class, err := h.svc.Update(r.Context(), teacherID, classID, req.Name)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Class not found")
			return
		}
		if errors.Is(err, ErrDuplicateName) {
			writeError(w, http.StatusConflict, "duplicate_name", "A class with this name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to update class")
		return
	}
	writeJSON(w, http.StatusOK, class)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	confirm := r.URL.Query().Get("confirm") == "true"

	teacherID := auth.TeacherIDFromContext(r.Context())
	warning, err := h.svc.Delete(r.Context(), teacherID, classID, confirm)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Class not found")
			return
		}
		if errors.Is(err, ErrConfirmRequired) {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"confirm_required": true,
				"warning":          warning,
			})
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to delete class")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}
