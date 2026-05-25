package exams

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
	exams, err := h.svc.List(r.Context(), classID, teacherID)
	if err != nil {
		if errors.Is(err, ErrClassNotFound) {
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
			return
		}
		log.Printf("ERROR List exams classID=%s: %v", classID, err)
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"exams": exams})
}

type createRequest struct {
	Name     string `json:"name"`
	ExamDate string `json:"exam_date"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}
	teacherID := auth.TeacherIDFromContext(r.Context())
	exam, err := h.svc.Create(r.Context(), classID, teacherID, req.Name, req.ExamDate)
	if err != nil {
		if errors.Is(err, ErrClassNotFound) {
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
			return
		}
		log.Printf("ERROR Create exam classID=%s: %v", classID, err)
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, exam)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	examID := chi.URLParam(r, "examID")
	teacherID := auth.TeacherIDFromContext(r.Context())
	if err := h.svc.Delete(r.Context(), classID, teacherID, examID); err != nil {
		switch {
		case errors.Is(err, ErrClassNotFound):
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
		case errors.Is(err, ErrExamNotFound):
			writeError(w, http.StatusNotFound, "exam_not_found", "Exam not found")
		default:
			log.Printf("ERROR Delete exam examID=%s: %v", examID, err)
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Exam deleted"})
}

func (h *Handler) GetMarks(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	examID := chi.URLParam(r, "examID")
	teacherID := auth.TeacherIDFromContext(r.Context())
	grid, err := h.svc.GetMarks(r.Context(), classID, teacherID, examID)
	if err != nil {
		switch {
		case errors.Is(err, ErrClassNotFound):
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
		case errors.Is(err, ErrExamNotFound):
			writeError(w, http.StatusNotFound, "exam_not_found", "Exam not found")
		default:
			log.Printf("ERROR GetMarks examID=%s: %v", examID, err)
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, grid)
}

type upsertRequest struct {
	Entries []MarkEntry `json:"entries"`
}

func (h *Handler) UpsertMarks(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	examID := chi.URLParam(r, "examID")
	var req upsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}
	if len(req.Entries) == 0 {
		writeError(w, http.StatusBadRequest, "missing_entries", "entries array cannot be empty")
		return
	}
	teacherID := auth.TeacherIDFromContext(r.Context())
	if err := h.svc.UpsertMarks(r.Context(), classID, teacherID, examID, req.Entries); err != nil {
		switch {
		case errors.Is(err, ErrClassNotFound):
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
		case errors.Is(err, ErrExamNotFound):
			writeError(w, http.StatusNotFound, "exam_not_found", "Exam not found")
		default:
			log.Printf("ERROR UpsertMarks examID=%s: %v", examID, err)
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Marks saved"})
}

func (h *Handler) DownloadMarksheet(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	examID := chi.URLParam(r, "examID")
	teacherID := auth.TeacherIDFromContext(r.Context())

	data, err := h.svc.GetMarksheetData(r.Context(), classID, teacherID, examID)
	if err != nil {
		switch {
		case errors.Is(err, ErrClassNotFound):
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
		case errors.Is(err, ErrExamNotFound):
			writeError(w, http.StatusNotFound, "exam_not_found", "Exam not found")
		default:
			log.Printf("ERROR DownloadMarksheet examID=%s: %v", examID, err)
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}

	pdfBytes, err := GenerateMarksheetPDF(data)
	if err != nil {
		log.Printf("ERROR GenerateMarksheetPDF examID=%s: %v", examID, err)
		writeError(w, http.StatusInternalServerError, "pdf_error", "Failed to generate PDF")
		return
	}

	fileName := data.ClassName + "_" + data.ExamName + "_Marksheet.pdf"
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+fileName+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfBytes)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}
