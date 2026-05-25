package admission

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
	teacherID := auth.TeacherIDFromContext(r.Context())
	status := r.URL.Query().Get("status")
	inquiries, err := h.svc.List(r.Context(), teacherID, status)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"inquiries": inquiries})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	teacherID := auth.TeacherIDFromContext(r.Context())
	var body struct {
		StudentName    string `json:"student_name"`
		DateOfBirth    string `json:"date_of_birth"`
		ParentName     string `json:"parent_name"`
		ContactNumber  string `json:"contact_number"`
		Email          string `json:"email"`
		PreviousSchool string `json:"previous_school"`
		ClassApplied   string `json:"class_applied"`
		AcademicYear   string `json:"academic_year"`
		Notes          string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	q, err := h.svc.Create(r.Context(), teacherID, CreateInput{
		StudentName:    body.StudentName,
		DateOfBirth:    body.DateOfBirth,
		ParentName:     body.ParentName,
		ContactNumber:  body.ContactNumber,
		Email:          body.Email,
		PreviousSchool: body.PreviousSchool,
		ClassApplied:   body.ClassApplied,
		AcademicYear:   body.AcademicYear,
		Notes:          body.Notes,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, q)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	teacherID := auth.TeacherIDFromContext(r.Context())
	id := chi.URLParam(r, "inquiryID")
	var body struct {
		StudentName    string `json:"student_name"`
		DateOfBirth    string `json:"date_of_birth"`
		ParentName     string `json:"parent_name"`
		ContactNumber  string `json:"contact_number"`
		Email          string `json:"email"`
		PreviousSchool string `json:"previous_school"`
		ClassApplied   string `json:"class_applied"`
		AcademicYear   string `json:"academic_year"`
		Notes          string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	q, err := h.svc.Update(r.Context(), teacherID, id, CreateInput{
		StudentName:    body.StudentName,
		DateOfBirth:    body.DateOfBirth,
		ParentName:     body.ParentName,
		ContactNumber:  body.ContactNumber,
		Email:          body.Email,
		PreviousSchool: body.PreviousSchool,
		ClassApplied:   body.ClassApplied,
		AcademicYear:   body.AcademicYear,
		Notes:          body.Notes,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, q)
}

func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	teacherID := auth.TeacherIDFromContext(r.Context())
	id := chi.URLParam(r, "inquiryID")
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	q, err := h.svc.UpdateStatus(r.Context(), teacherID, id, body.Status)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, q)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	teacherID := auth.TeacherIDFromContext(r.Context())
	id := chi.URLParam(r, "inquiryID")
	if err := h.svc.Delete(r.Context(), teacherID, id); err != nil {
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
	code := "internal_error"
	status := http.StatusInternalServerError
	if errors.Is(err, ErrNotFound) {
		code, status = "inquiry_not_found", http.StatusNotFound
	}
	writeJSON(w, status, map[string]string{"error": code, "message": err.Error()})
}
