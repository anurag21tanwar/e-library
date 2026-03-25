// Package handlers implement the HTTP handler layer for the e-Library API.
package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"e-library/respond"
	"e-library/service"
)

const maxBodyBytes = 1 << 20 // 1 MB — guards against oversized request bodies

// Handler holds the service dependencies and logger shared across all HTTP handlers.
type Handler struct {
	books  service.BookService
	loans  service.LoanService
	logger *slog.Logger
}

// NewHandler creates a Handler wired to the given services and logger.
// Since *service.libraryService satisfies both BookService and LoanService,
// callers typically pass the same value for both parameters.
func NewHandler(books service.BookService, loans service.LoanService, logger *slog.Logger) *Handler {
	return &Handler{books: books, loans: loans, logger: logger}
}

// Register registers all handler routes on mux.
// Implementing Register satisfies the routes.Registrar interface, allowing NewRouter
// to accept any number of handler groups without modification (OCP).
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /Book", h.GetBook)
	mux.HandleFunc("POST /Borrow", h.BorrowBook)
	mux.HandleFunc("POST /Extend", h.ExtendLoan)
	mux.HandleFunc("POST /Return", h.ReturnBook)
}

// decodeRequest applies a body size limit, then decodes JSON into v.
// It writes a 400 response and returns false on any failure.
func decodeRequest(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		respond.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return false
	}
	return true
}

// --- Handlers ---

// GetBook handles GET /Book?title=<title>
func (h *Handler) GetBook(w http.ResponseWriter, r *http.Request) {
	title := r.URL.Query().Get("title")
	if title == "" {
		respond.Error(w, "Title query parameter is required", http.StatusBadRequest)
		return
	}

	book, err := h.books.GetBook(title)
	if err != nil {
		respond.Error(w, "Book not found", http.StatusNotFound)
		return
	}

	respond.JSON(w, h.logger, http.StatusOK, book)
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
		respond.Error(w, "Name and Title are required", http.StatusBadRequest)
		return
	}

	loan, err := h.loans.BorrowBook(req.Name, req.Title)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrBookNotFound):
			respond.Error(w, "Book does not exist", http.StatusNotFound)
		case errors.Is(err, service.ErrNoStock):
			respond.Error(w, "No copies available for loan", http.StatusConflict)
		case errors.Is(err, service.ErrDuplicateLoan):
			respond.Error(w, "User already has an active loan for this book", http.StatusConflict)
		default:
			respond.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	respond.JSON(w, h.logger, http.StatusCreated, loan)
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
		respond.Error(w, "Name and Title are required", http.StatusBadRequest)
		return
	}

	loan, err := h.loans.ExtendLoan(req.Name, req.Title)
	if err != nil {
		respond.Error(w, "No active loan found for this user and book", http.StatusNotFound)
		return
	}

	respond.JSON(w, h.logger, http.StatusOK, loan)
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
		respond.Error(w, "Name and Title are required", http.StatusBadRequest)
		return
	}

	err := h.loans.ReturnBook(req.Name, req.Title)
	if err != nil {
		if errors.Is(err, service.ErrLoanNotFound) {
			respond.Error(w, "Active loan record not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, service.ErrStockRestoreFailed) {
			// The loan was successfully deleted; stock restoration is a data-integrity
			// issue logged by the service. Return 200 — the user's action succeeded.
			h.logger.Error("stock restore failed after return",
				"name", req.Name, "title", req.Title)
		} else {
			respond.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	respond.JSON(w, h.logger, http.StatusOK, map[string]string{"message": "Book returned successfully"})
}
