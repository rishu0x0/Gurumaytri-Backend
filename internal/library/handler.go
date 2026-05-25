package library

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"school-management/backend/internal/auth"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) ListBooks(w http.ResponseWriter, r *http.Request) {
	teacherID := auth.TeacherIDFromContext(r.Context())
	books, err := h.svc.ListBooks(r.Context(), teacherID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"books": books})
}

func (h *Handler) CreateBook(w http.ResponseWriter, r *http.Request) {
	teacherID := auth.TeacherIDFromContext(r.Context())
	var body struct {
		Title       string `json:"title"`
		Author      string `json:"author"`
		ISBN        string `json:"isbn"`
		Category    string `json:"category"`
		TotalCopies int    `json:"total_copies"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	book, err := h.svc.CreateBook(r.Context(), teacherID, BookInput{
		Title: body.Title, Author: body.Author, ISBN: body.ISBN,
		Category: body.Category, TotalCopies: body.TotalCopies,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, book)
}

func (h *Handler) UpdateBook(w http.ResponseWriter, r *http.Request) {
	teacherID := auth.TeacherIDFromContext(r.Context())
	bookID := chi.URLParam(r, "bookID")
	var body struct {
		Title       string `json:"title"`
		Author      string `json:"author"`
		ISBN        string `json:"isbn"`
		Category    string `json:"category"`
		TotalCopies int    `json:"total_copies"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	book, err := h.svc.UpdateBook(r.Context(), teacherID, bookID, BookInput{
		Title: body.Title, Author: body.Author, ISBN: body.ISBN,
		Category: body.Category, TotalCopies: body.TotalCopies,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, book)
}

func (h *Handler) DeleteBook(w http.ResponseWriter, r *http.Request) {
	teacherID := auth.TeacherIDFromContext(r.Context())
	bookID := chi.URLParam(r, "bookID")
	if err := h.svc.DeleteBook(r.Context(), teacherID, bookID); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListIssues(w http.ResponseWriter, r *http.Request) {
	teacherID := auth.TeacherIDFromContext(r.Context())
	activeOnly, _ := strconv.ParseBool(r.URL.Query().Get("active"))
	issues, err := h.svc.ListIssues(r.Context(), teacherID, activeOnly)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"issues": issues})
}

func (h *Handler) IssueBook(w http.ResponseWriter, r *http.Request) {
	teacherID := auth.TeacherIDFromContext(r.Context())
	bookID := chi.URLParam(r, "bookID")
	var body struct {
		StudentName string `json:"student_name"`
		ClassName   string `json:"class_name"`
		DueDate     string `json:"due_date"`
		Notes       string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	issue, err := h.svc.IssueBook(r.Context(), teacherID, bookID,
		body.StudentName, body.ClassName, body.DueDate, body.Notes)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, issue)
}

func (h *Handler) ReturnBook(w http.ResponseWriter, r *http.Request) {
	teacherID := auth.TeacherIDFromContext(r.Context())
	issueID := chi.URLParam(r, "issueID")
	issue, err := h.svc.ReturnBook(r.Context(), teacherID, issueID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, issue)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	code := "internal_error"
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrBookNotFound):
		code, status = "book_not_found", http.StatusNotFound
	case errors.Is(err, ErrIssueNotFound):
		code, status = "issue_not_found", http.StatusNotFound
	case errors.Is(err, ErrNoCopiesLeft):
		code, status = "no_copies_available", http.StatusConflict
	}
	writeJSON(w, status, map[string]string{"error": code, "message": err.Error()})
}
