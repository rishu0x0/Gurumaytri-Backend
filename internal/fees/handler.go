package fees

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

func (h *Handler) ListStructures(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	teacherID := auth.TeacherIDFromContext(r.Context())
	structures, err := h.svc.ListStructures(r.Context(), classID, teacherID)
	if err != nil {
		if errors.Is(err, ErrClassNotFound) {
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
			return
		}
		log.Printf("ERROR ListStructures classID=%s: %v", classID, err)
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"fee_structures": structures})
}

type createStructureReq struct {
	Title        string  `json:"title"`
	Amount       float64 `json:"amount"`
	DueDate      string  `json:"due_date"`
	AcademicYear string  `json:"academic_year"`
}

func (h *Handler) CreateStructure(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	var req createStructureReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}
	teacherID := auth.TeacherIDFromContext(r.Context())
	fs, err := h.svc.CreateStructure(r.Context(), classID, teacherID, req.Title, req.Amount, req.DueDate, req.AcademicYear)
	if err != nil {
		if errors.Is(err, ErrClassNotFound) {
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
			return
		}
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, fs)
}

type updateStructureReq struct {
	Title        string  `json:"title"`
	Amount       float64 `json:"amount"`
	DueDate      string  `json:"due_date"`
	AcademicYear string  `json:"academic_year"`
}

func (h *Handler) UpdateStructure(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	structureID := chi.URLParam(r, "structureID")
	var req updateStructureReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}
	teacherID := auth.TeacherIDFromContext(r.Context())
	fs, err := h.svc.UpdateStructure(r.Context(), classID, teacherID, structureID, req.Title, req.Amount, req.DueDate, req.AcademicYear)
	if err != nil {
		switch {
		case errors.Is(err, ErrClassNotFound):
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
		case errors.Is(err, ErrStructureNotFound):
			writeError(w, http.StatusNotFound, "fee_structure_not_found", "Fee structure not found")
		default:
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, fs)
}

func (h *Handler) DeleteStructure(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	structureID := chi.URLParam(r, "structureID")
	teacherID := auth.TeacherIDFromContext(r.Context())
	if err := h.svc.DeleteStructure(r.Context(), classID, teacherID, structureID); err != nil {
		switch {
		case errors.Is(err, ErrClassNotFound):
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
		case errors.Is(err, ErrStructureNotFound):
			writeError(w, http.StatusNotFound, "fee_structure_not_found", "Fee structure not found")
		default:
			log.Printf("ERROR DeleteStructure structureID=%s: %v", structureID, err)
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Fee structure deleted"})
}

func (h *Handler) GetPayments(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	structureID := chi.URLParam(r, "structureID")
	teacherID := auth.TeacherIDFromContext(r.Context())
	records, feeAmount, err := h.svc.GetPayments(r.Context(), classID, teacherID, structureID)
	if err != nil {
		switch {
		case errors.Is(err, ErrClassNotFound):
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
		case errors.Is(err, ErrStructureNotFound):
			writeError(w, http.StatusNotFound, "fee_structure_not_found", "Fee structure not found")
		default:
			log.Printf("ERROR GetPayments structureID=%s: %v", structureID, err)
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"fee_amount": feeAmount,
		"records":    records,
	})
}

type recordPaymentReq struct {
	StudentID   string  `json:"student_id"`
	AmountPaid  float64 `json:"amount_paid"`
	PaidAt      string  `json:"paid_at"`
	PaymentMode string  `json:"payment_mode"`
	Notes       string  `json:"notes"`
}

func (h *Handler) RecordPayment(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	structureID := chi.URLParam(r, "structureID")
	var req recordPaymentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}
	if req.StudentID == "" {
		writeError(w, http.StatusBadRequest, "missing_student_id", "student_id is required")
		return
	}
	teacherID := auth.TeacherIDFromContext(r.Context())
	if err := h.svc.RecordPayment(r.Context(), classID, teacherID, structureID, PaymentInput{
		StudentID:   req.StudentID,
		AmountPaid:  req.AmountPaid,
		PaidAt:      req.PaidAt,
		PaymentMode: req.PaymentMode,
		Notes:       req.Notes,
	}); err != nil {
		switch {
		case errors.Is(err, ErrClassNotFound):
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
		case errors.Is(err, ErrStructureNotFound):
			writeError(w, http.StatusNotFound, "fee_structure_not_found", "Fee structure not found")
		default:
			log.Printf("ERROR RecordPayment structureID=%s student=%s: %v", structureID, req.StudentID, err)
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Payment recorded"})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}
