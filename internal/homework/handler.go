package homework

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"school-management/backend/internal/auth"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	teacherID := auth.TeacherIDFromContext(r.Context())
	assignments, err := h.svc.List(r.Context(), classID, teacherID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"assignments": assignments})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	teacherID := auth.TeacherIDFromContext(r.Context())
	var body struct {
		Subject     string `json:"subject"`
		Title       string `json:"title"`
		Description string `json:"description"`
		DueDate     string `json:"due_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	a, err := h.svc.Create(r.Context(), classID, teacherID, Input{
		Subject: body.Subject, Title: body.Title,
		Description: body.Description, DueDate: body.DueDate,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, a)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	assignmentID := chi.URLParam(r, "assignmentID")
	teacherID := auth.TeacherIDFromContext(r.Context())
	var body struct {
		Subject     string `json:"subject"`
		Title       string `json:"title"`
		Description string `json:"description"`
		DueDate     string `json:"due_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	a, err := h.svc.Update(r.Context(), classID, teacherID, assignmentID, Input{
		Subject: body.Subject, Title: body.Title,
		Description: body.Description, DueDate: body.DueDate,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	assignmentID := chi.URLParam(r, "assignmentID")
	teacherID := auth.TeacherIDFromContext(r.Context())
	if err := h.svc.Delete(r.Context(), classID, teacherID, assignmentID); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	code, status := "internal_error", http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrClassNotFound):
		code, status = "class_not_found", http.StatusNotFound
	case errors.Is(err, ErrAssignmentNotFound):
		code, status = "assignment_not_found", http.StatusNotFound
	}
	writeJSON(w, status, map[string]string{"error": code, "message": err.Error()})
}
