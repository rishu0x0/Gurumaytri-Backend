package examination

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

// ── Events ─────────────────────────────────────────────────────────────────

func (h *Handler) ListEvents(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	teacherID := auth.TeacherIDFromContext(r.Context())
	events, err := h.svc.ListEvents(r.Context(), classID, teacherID)
	if err != nil {
		if errors.Is(err, ErrClassNotFound) {
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
			return
		}
		log.Printf("ERROR ListEvents classID=%s: %v", classID, err)
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"events": events})
}

type createEventReq struct {
	Title        string `json:"title"`
	AcademicYear string `json:"academic_year"`
}

func (h *Handler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	var req createEventReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}
	teacherID := auth.TeacherIDFromContext(r.Context())
	ev, err := h.svc.CreateEvent(r.Context(), classID, teacherID, req.Title, req.AcademicYear)
	if err != nil {
		if errors.Is(err, ErrClassNotFound) {
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
			return
		}
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, ev)
}

func (h *Handler) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	eventID := chi.URLParam(r, "eventID")
	teacherID := auth.TeacherIDFromContext(r.Context())
	if err := h.svc.DeleteEvent(r.Context(), classID, teacherID, eventID); err != nil {
		switch {
		case errors.Is(err, ErrClassNotFound):
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
		case errors.Is(err, ErrEventNotFound):
			writeError(w, http.StatusNotFound, "event_not_found", "Exam event not found")
		default:
			log.Printf("ERROR DeleteEvent eventID=%s: %v", eventID, err)
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Exam event deleted"})
}

// ── Slots ───────────────────────────────────────────────────────────────────

func (h *Handler) ListSlots(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	eventID := chi.URLParam(r, "eventID")
	teacherID := auth.TeacherIDFromContext(r.Context())
	slots, err := h.svc.ListSlots(r.Context(), classID, teacherID, eventID)
	if err != nil {
		switch {
		case errors.Is(err, ErrClassNotFound):
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
		case errors.Is(err, ErrEventNotFound):
			writeError(w, http.StatusNotFound, "event_not_found", "Exam event not found")
		default:
			log.Printf("ERROR ListSlots eventID=%s: %v", eventID, err)
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"slots": slots})
}

type slotBody struct {
	SubjectName string `json:"subject_name"`
	ExamDate    string `json:"exam_date"`
	StartTime   string `json:"start_time"`
	EndTime     string `json:"end_time"`
	RoomNumber  string `json:"room_number"`
	MaxMarks    int    `json:"max_marks"`
}

func (h *Handler) CreateSlot(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	eventID := chi.URLParam(r, "eventID")
	var req slotBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}
	teacherID := auth.TeacherIDFromContext(r.Context())
	slot, err := h.svc.CreateSlot(r.Context(), classID, teacherID, eventID,
		req.SubjectName, req.ExamDate, req.StartTime, req.EndTime, req.RoomNumber, req.MaxMarks)
	if err != nil {
		switch {
		case errors.Is(err, ErrClassNotFound):
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
		case errors.Is(err, ErrEventNotFound):
			writeError(w, http.StatusNotFound, "event_not_found", "Exam event not found")
		case errors.Is(err, ErrDuplicateSlot):
			writeError(w, http.StatusConflict, "duplicate_subject_slot", "A slot for this subject already exists in this event")
		default:
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusCreated, slot)
}

func (h *Handler) UpdateSlot(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	eventID := chi.URLParam(r, "eventID")
	slotID := chi.URLParam(r, "slotID")
	var req slotBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body is not valid JSON")
		return
	}
	teacherID := auth.TeacherIDFromContext(r.Context())
	slot, err := h.svc.UpdateSlot(r.Context(), classID, teacherID, eventID, slotID,
		req.SubjectName, req.ExamDate, req.StartTime, req.EndTime, req.RoomNumber, req.MaxMarks)
	if err != nil {
		switch {
		case errors.Is(err, ErrClassNotFound):
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
		case errors.Is(err, ErrEventNotFound):
			writeError(w, http.StatusNotFound, "event_not_found", "Exam event not found")
		case errors.Is(err, ErrSlotNotFound):
			writeError(w, http.StatusNotFound, "slot_not_found", "Exam slot not found")
		case errors.Is(err, ErrDuplicateSlot):
			writeError(w, http.StatusConflict, "duplicate_subject_slot", "A slot for this subject already exists")
		default:
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, slot)
}

func (h *Handler) DeleteSlot(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	eventID := chi.URLParam(r, "eventID")
	slotID := chi.URLParam(r, "slotID")
	teacherID := auth.TeacherIDFromContext(r.Context())
	if err := h.svc.DeleteSlot(r.Context(), classID, teacherID, eventID, slotID); err != nil {
		switch {
		case errors.Is(err, ErrClassNotFound):
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
		case errors.Is(err, ErrEventNotFound):
			writeError(w, http.StatusNotFound, "event_not_found", "Exam event not found")
		case errors.Is(err, ErrSlotNotFound):
			writeError(w, http.StatusNotFound, "slot_not_found", "Exam slot not found")
		default:
			log.Printf("ERROR DeleteSlot slotID=%s: %v", slotID, err)
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Slot deleted"})
}

func (h *Handler) DownloadTimetable(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	eventID := chi.URLParam(r, "eventID")
	teacherID := auth.TeacherIDFromContext(r.Context())

	data, err := h.svc.GetTimetableData(r.Context(), classID, teacherID, eventID)
	if err != nil {
		switch {
		case errors.Is(err, ErrClassNotFound):
			writeError(w, http.StatusNotFound, "class_not_found", "Class not found")
		case errors.Is(err, ErrEventNotFound):
			writeError(w, http.StatusNotFound, "event_not_found", "Exam event not found")
		default:
			log.Printf("ERROR DownloadTimetable eventID=%s: %v", eventID, err)
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		}
		return
	}

	pdfBytes, err := GenerateTimetablePDF(data)
	if err != nil {
		log.Printf("ERROR GenerateTimetablePDF eventID=%s: %v", eventID, err)
		writeError(w, http.StatusInternalServerError, "pdf_error", "Failed to generate PDF")
		return
	}

	fileName := data.ClassName + "_" + data.EventTitle + "_Timetable.pdf"
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
