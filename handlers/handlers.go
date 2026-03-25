// Package handlers implement the HTTP handler layer for the e-Library API.
package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"e-library/repository"
)

const maxBodyBytes = 1 << 20 // 1 MB — guards against oversized request bodies

// ErrorResponse is the standard JSON error envelope for all error responses.
type ErrorResponse struct {
	Error string `json:"error"`
}

// Handler holds the dependencies shared across all HTTP handlers.
type Handler struct {
	store  repository.Store
	logger *slog.Logger
}

// NewHandler creates a Handler wired to the given store and logger.
func NewHandler(s repository.Store, logger *slog.Logger) *Handler {
	return &Handler{store: s, logger: logger}
}

// responseWriter wraps http.ResponseWriter to capture the status code written by handlers.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// loggingMiddleware logs method, path, status code, and latency for every request.
func (h *Handler) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		h.logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"latency_ms", time.Since(start).Milliseconds(),
		)
	})
}

// NewRouter registers all routes on a new ServeMux and returns it.
// Keeping routing here (rather than in the main) makes the handler package self-contained.
func NewRouter(h *Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "e-Library API is running")
	})
	mux.HandleFunc("GET /Book", h.GetBook)
	mux.HandleFunc("POST /Borrow", h.BorrowBook)
	mux.HandleFunc("POST /Extend", h.ExtendLoan)
	mux.HandleFunc("POST /Return", h.ReturnBook)

	return h.loggingMiddleware(mux)
}

// --- Response helpers ---

// writeError writes a JSON ErrorResponse with the given status code.
func writeError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

// writeJSON marshals v into a buffer before touching the response writer.
// This ensures a failed marshal results in a 500 error rather than a broken 200.
func (h *Handler) writeJSON(w http.ResponseWriter, code int, v any) {
	buf, err := json.Marshal(v)
	if err != nil {
		h.logger.Error("failed to marshal response", "error", err)
		writeError(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(buf)
}

// decodeRequest applies a body size limit, then decodes JSON into v.
// It writes a 400 response and returns false on any failure.
func decodeRequest(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, "Invalid JSON payload", http.StatusBadRequest)
		return false
	}
	return true
}

// --- Handlers ---

// GetBook handles GET /Book?title=<title>
func (h *Handler) GetBook(w http.ResponseWriter, r *http.Request) {
	title := r.URL.Query().Get("title")
	if title == "" {
		writeError(w, "Title query parameter is required", http.StatusBadRequest)
		return
	}

	book, err := h.store.GetBook(title)
	if err != nil {
		writeError(w, "Book not found", http.StatusNotFound)
		return
	}

	h.writeJSON(w, http.StatusOK, book)
}

// BorrowBook handles POST /Borrow
func (h *Handler) BorrowBook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Title string `json:"title"`
	}
	if !decodeRequest(w, r, &req) {
		return
	}
	if req.Name == "" || req.Title == "" {
		writeError(w, "Name and Title are required", http.StatusBadRequest)
		return
	}

	loan, err := h.store.BorrowBook(req.Name, req.Title)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrBookNotFound):
			writeError(w, "Book does not exist", http.StatusNotFound)
		case errors.Is(err, repository.ErrNoStock):
			writeError(w, "No copies available for loan", http.StatusConflict)
		case errors.Is(err, repository.ErrDuplicateLoan):
			writeError(w, "User already has an active loan for this book", http.StatusConflict)
		default:
			writeError(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	h.writeJSON(w, http.StatusCreated, loan)
}

// ExtendLoan handles POST /Extend
func (h *Handler) ExtendLoan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Title string `json:"title"`
	}
	if !decodeRequest(w, r, &req) {
		return
	}
	if req.Name == "" || req.Title == "" {
		writeError(w, "Name and Title are required", http.StatusBadRequest)
		return
	}

	loan, err := h.store.ExtendLoan(req.Name, req.Title)
	if err != nil {
		writeError(w, "No active loan found for this user and book", http.StatusNotFound)
		return
	}

	h.writeJSON(w, http.StatusOK, loan)
}

// ReturnBook handles POST /Return
func (h *Handler) ReturnBook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Title string `json:"title"`
	}
	if !decodeRequest(w, r, &req) {
		return
	}
	if req.Name == "" || req.Title == "" {
		writeError(w, "Name and Title are required", http.StatusBadRequest)
		return
	}

	if err := h.store.ReturnBook(req.Name, req.Title); err != nil {
		writeError(w, "Active loan record not found", http.StatusNotFound)
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{"message": "Book returned successfully"})
}
