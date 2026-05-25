package timetable

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"school-management/backend/internal/auth"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) ListSlots(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	teacherID := auth.TeacherIDFromContext(r.Context())
	slots, err := h.svc.List(r.Context(), classID, teacherID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"slots": slots})
}

func (h *Handler) UpsertSlot(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	teacherID := auth.TeacherIDFromContext(r.Context())

	var body struct {
		DayOfWeek    int    `json:"day_of_week"`
		PeriodNumber int    `json:"period_number"`
		SubjectName  string `json:"subject_name"`
		StartTime    string `json:"start_time"`
		EndTime      string `json:"end_time"`
		RoomNumber   string `json:"room_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	slot, err := h.svc.Upsert(r.Context(), classID, teacherID, UpsertInput{
		DayOfWeek:    body.DayOfWeek,
		PeriodNumber: body.PeriodNumber,
		SubjectName:  body.SubjectName,
		StartTime:    body.StartTime,
		EndTime:      body.EndTime,
		RoomNumber:   body.RoomNumber,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, slot)
}

func (h *Handler) DeleteSlot(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	slotID := chi.URLParam(r, "slotID")
	teacherID := auth.TeacherIDFromContext(r.Context())
	if err := h.svc.Delete(r.Context(), classID, teacherID, slotID); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DownloadTimetable(w http.ResponseWriter, r *http.Request) {
	classID := chi.URLParam(r, "classID")
	teacherID := auth.TeacherIDFromContext(r.Context())

	data, err := h.svc.GetTimetableData(r.Context(), classID, teacherID)
	if err != nil {
		writeErr(w, err)
		return
	}
	pdf, err := GenerateTimetablePDF(data)
	if err != nil {
		http.Error(w, "PDF generation failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition",
		`attachment; filename="`+data.ClassName+`_Timetable.pdf"`)
	_, _ = w.Write(pdf)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	code := "internal_error"
	status := http.StatusInternalServerError
	if errors.Is(err, ErrClassNotFound) {
		code, status = "class_not_found", http.StatusNotFound
	} else if errors.Is(err, ErrSlotNotFound) {
		code, status = "slot_not_found", http.StatusNotFound
	}
	writeJSON(w, status, map[string]string{"error": code, "message": err.Error()})
}
