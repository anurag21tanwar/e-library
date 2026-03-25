// Package main provides a RESTful API for an e-Library system,
// allowing users to search, borrow, extend, and return books.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// --- Models ---

// BookDetail represents the library's inventory
type BookDetail struct {
	Title           string `json:"title"`
	AvailableCopies int    `json:"available_copies"`
}

// LoanDetail represents a record of a borrowed book
type LoanDetail struct {
	NameOfBorrower string    `json:"name_of_borrower"`
	BookTitle      string    `json:"book_title"`
	LoanDate       time.Time `json:"loan_date"`
	ReturnDate     time.Time `json:"return_date"`
}

// --- In-Memory Repository ---

// LibraryStore holds our data and a Mutex for thread-safety
type LibraryStore struct {
	mu    sync.RWMutex
	Books map[string]*BookDetail
	Loans map[string]LoanDetail // keyed by loanKey(name, title) for O(1) lookup
}

// loanKey returns a unique map key for a (borrower, book) pair.
// A null byte separator prevents collisions between names/titles that contain colons.
func loanKey(name, title string) string {
	return name + "\x00" + title
}

// NewLibraryStore initializes the store with some test data
func NewLibraryStore() *LibraryStore {
	store := &LibraryStore{
		Books: make(map[string]*BookDetail),
		Loans: make(map[string]LoanDetail),
	}

	// Seed data as per requirement 2
	store.Books["The Go Programming Language"] = &BookDetail{
		Title:           "The Go Programming Language",
		AvailableCopies: 3,
	}
	store.Books["Clean Code"] = &BookDetail{
		Title:           "Clean Code",
		AvailableCopies: 1,
	}

	return store
}

// ErrorResponse is the standard JSON error envelope returned by all endpoints
type ErrorResponse struct {
	Error string `json:"error"`
}

// Handler will hold our store dependency for the API endpoints
type Handler struct {
	store *LibraryStore
}

// writeError writes a JSON-encoded ErrorResponse with the given status code
func writeError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

// writeJSON encodes v into a buffer first; only if that succeeds does it write
// the status code and body. This prevents sending a 200 header followed by a
// broken body when encoding fails.
func writeJSON(w http.ResponseWriter, code int, v any) {
	buf, err := json.Marshal(v)
	if err != nil {
		log.Printf("failed to encode response: %v", err)
		writeError(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(buf)
}

// GetBook handles GET /Book?title=XYZ
func (h *Handler) GetBook(w http.ResponseWriter, r *http.Request) {
	// 1. Extract the query parameter
	title := r.URL.Query().Get("title")
	if title == "" {
		writeError(w, "Title query parameter is required", http.StatusBadRequest)
		return
	}

	// 2. Read from the store with a Read-Lock (allows multiple simultaneous readers)
	h.store.mu.RLock()
	book, exists := h.store.Books[title]
	var bookCopy BookDetail
	if exists {
		bookCopy = *book // copy value while holding the lock to avoid a data race
	}
	h.store.mu.RUnlock()

	// 3. Handle the "Not Found" case
	if !exists {
		writeError(w, "Book not found", http.StatusNotFound)
		return
	}

	// 4. Return JSON response
	writeJSON(w, http.StatusOK, bookCopy)
}

// BorrowBook handles POST /Borrow
func (h *Handler) BorrowBook(w http.ResponseWriter, r *http.Request) {
	// 1. Define the input structure
	var req struct {
		Name  string `json:"name"`
		Title string `json:"title"`
	}

	// 2. Decode JSON body
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// 3. Validation
	if req.Name == "" || req.Title == "" {
		writeError(w, "Name and Title are required", http.StatusBadRequest)
		return
	}

	// 4. Critical Section: Update state
	h.store.mu.Lock()
	defer h.store.mu.Unlock() // Ensure unlock happens even if we return early

	book, exists := h.store.Books[req.Title]
	if !exists {
		writeError(w, "Book does not exist", http.StatusNotFound)
		return
	}

	if book.AvailableCopies <= 0 {
		writeError(w, "No copies available for loan", http.StatusConflict)
		return
	}

	// 5. Perform the Transaction
	book.AvailableCopies--

	now := time.Now()
	loan := LoanDetail{
		NameOfBorrower: req.Name,
		BookTitle:      req.Title,
		LoanDate:       now,
		ReturnDate:     now.AddDate(0, 0, 28), // 4 weeks as per requirement
	}
	h.store.Loans[loanKey(req.Name, req.Title)] = loan

	// 6. Respond with the loan details
	writeJSON(w, http.StatusCreated, loan)
}

// ExtendLoan handles POST /Extend
func (h *Handler) ExtendLoan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Title == "" {
		writeError(w, "Name and Title are required", http.StatusBadRequest)
		return
	}

	h.store.mu.Lock()
	defer h.store.mu.Unlock()

	// O(1) lookup for the active loan
	key := loanKey(req.Name, req.Title)
	loan, exists := h.store.Loans[key]
	if !exists {
		writeError(w, "No active loan found for this user and book", http.StatusNotFound)
		return
	}

	// Extend by 3 weeks (21 days) from the current return date
	loan.ReturnDate = loan.ReturnDate.AddDate(0, 0, 21)
	h.store.Loans[key] = loan

	writeJSON(w, http.StatusOK, loan)
}

// ReturnBook handles POST /Return
func (h *Handler) ReturnBook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if req.Name == "" || req.Title == "" {
		writeError(w, "Name and Title are required", http.StatusBadRequest)
		return
	}

	h.store.mu.Lock()
	defer h.store.mu.Unlock()

	// O(1) lookup and removal
	key := loanKey(req.Name, req.Title)
	if _, exists := h.store.Loans[key]; !exists {
		writeError(w, "Active loan record not found", http.StatusNotFound)
		return
	}

	// 1. Remove loan from the map
	delete(h.store.Loans, key)

	// 2. Increment the available copies back in the book map
	if book, ok := h.store.Books[req.Title]; ok {
		book.AvailableCopies++
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Book returned successfully"})
}

func main() {
	// Initialize store and handlers
	store := NewLibraryStore()
	h := &Handler{store: store}

	// Initialize a new serve mux (router)
	mux := http.NewServeMux()

	// Root health check to verify the server is up
	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "e-Library API is running")
	})

	// Register the GET /Book route
	mux.HandleFunc("GET /Book", h.GetBook)

	// Register the POST /Borrow route
	mux.HandleFunc("POST /Borrow", h.BorrowBook)

	// Register the POST /Extend route
	mux.HandleFunc("POST /Extend", h.ExtendLoan)

	// Register the POST /Return route
	mux.HandleFunc("POST /Return", h.ReturnBook)

	port := ":3000"
	fmt.Printf("Server starting on port %s...\n", port)

	// Start the server
	err := http.ListenAndServe(port, mux)
	if err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
