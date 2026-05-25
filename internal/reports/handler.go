package reports

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"school-management/backend/internal/auth"
)

// ReportHandler handles HTTP requests for report generation and listing.
type ReportHandler struct {
	svc *ReportService
}

func NewHandler(svc *ReportService) *ReportHandler {
	return &ReportHandler{svc: svc}
}

var monthRE = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

// Generate handles POST /reports/generate
// Body: {"class_id": "...", "month": "YYYY-MM", "format": "pdf|excel"}
// Returns 202: {"report_id": "..."}
func (h *ReportHandler) Generate(w http.ResponseWriter, r *http.Request) {
	teacherID := auth.TeacherIDFromContext(r.Context())

	var body struct {
		ClassID string `json:"class_id"`
		Month   string `json:"month"`
		Format  string `json:"format"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if body.ClassID == "" {
		http.Error(w, "class_id required", http.StatusBadRequest)
		return
	}
	if !monthRE.MatchString(body.Month) {
		http.Error(w, "month must be YYYY-MM", http.StatusBadRequest)
		return
	}
	if body.Format != "pdf" && body.Format != "excel" {
		http.Error(w, "format must be pdf or excel", http.StatusBadRequest)
		return
	}

	reportID, err := h.svc.CreateReportRow(r.Context(), teacherID, body.ClassID, body.Month, body.Format)
	if err != nil {
		http.Error(w, "failed to create report: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Generate in background
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := h.svc.GenerateReport(ctx, reportID, body.ClassID, teacherID, body.Month, body.Format); err != nil {
			_ = err // status already set to "error" by GenerateReport
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"report_id": reportID})
}

// List handles GET /reports — returns all non-expired reports for the teacher.
func (h *ReportHandler) List(w http.ResponseWriter, r *http.Request) {
	teacherID := auth.TeacherIDFromContext(r.Context())

	rpts, err := h.svc.ListReports(r.Context(), teacherID)
	if err != nil {
		http.Error(w, "failed to list reports: "+err.Error(), http.StatusInternalServerError)
		return
	}

	type reportJSON struct {
		ID        string  `json:"id"`
		ClassID   string  `json:"class_id"`
		Month     string  `json:"month"`
		Format    string  `json:"format"`
		FileName  *string `json:"file_name"`
		Status    string  `json:"status"`
		ErrorMsg  *string `json:"error_msg,omitempty"`
		SignedURL *string `json:"signed_url,omitempty"`
		CreatedAt string  `json:"created_at"`
	}

	out := make([]reportJSON, len(rpts))
	for i, rpt := range rpts {
		out[i] = reportJSON{
			ID:        rpt.ID,
			ClassID:   rpt.ClassID,
			Month:     rpt.Month,
			Format:    rpt.Format,
			FileName:  rpt.FileName,
			Status:    rpt.Status,
			ErrorMsg:  rpt.ErrorMsg,
			SignedURL: rpt.SignedURL,
			CreatedAt: rpt.CreatedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// StatusCheck handles GET /reports/{reportID}/status
func (h *ReportHandler) StatusCheck(w http.ResponseWriter, r *http.Request) {
	teacherID := auth.TeacherIDFromContext(r.Context())
	reportID := chi.URLParam(r, "reportID")

	rpts, err := h.svc.ListReports(r.Context(), teacherID)
	if err != nil {
		http.Error(w, "query error", http.StatusInternalServerError)
		return
	}
	for _, rpt := range rpts {
		if rpt.ID == reportID {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":     rpt.Status,
				"signed_url": rpt.SignedURL,
				"error_msg":  rpt.ErrorMsg,
			})
			return
		}
	}
	http.Error(w, "report not found", http.StatusNotFound)
}
