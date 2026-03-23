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
	Loans []LoanDetail
}

// NewLibraryStore initializes the store with some test data
func NewLibraryStore() *LibraryStore {
	store := &LibraryStore{
		Books: make(map[string]*BookDetail),
		Loans: []LoanDetail{},
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

// Handler will hold our store dependency for the API endpoints
type Handler struct {
	store *LibraryStore
}

// GetBook handles GET /Book?title=XYZ
func (h *Handler) GetBook(w http.ResponseWriter, r *http.Request) {
	// 1. Extract the query parameter
	title := r.URL.Query().Get("title")
	if title == "" {
		http.Error(w, "Title query parameter is required", http.StatusBadRequest)
		return
	}

	// 2. Read from the store with a Read-Lock (allows multiple simultaneous readers)
	h.store.mu.RLock()
	book, exists := h.store.Books[title]
	h.store.mu.RUnlock()

	// 3. Handle the "Not Found" case
	if !exists {
		http.Error(w, "Book not found", http.StatusNotFound)
		return
	}

	// 4. Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(book)
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
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	// 3. Validation
	if req.Name == "" || req.Title == "" {
		http.Error(w, "Name and Title are required", http.StatusBadRequest)
		return
	}

	// 4. Critical Section: Update state
	h.store.mu.Lock()
	defer h.store.mu.Unlock() // Ensure unlock happens even if we return early

	book, exists := h.store.Books[req.Title]
	if !exists {
		http.Error(w, "Book does not exist", http.StatusNotFound)
		return
	}

	if book.AvailableCopies <= 0 {
		http.Error(w, "No copies available for loan", http.StatusConflict)
		return
	}

	// 5. Perform the Transaction
	book.AvailableCopies--

	loan := LoanDetail{
		NameOfBorrower: req.Name,
		BookTitle:      req.Title,
		LoanDate:       time.Now(),
		ReturnDate:     time.Now().AddDate(0, 0, 28), // 4 weeks as per requirement
	}
	h.store.Loans = append(h.store.Loans, loan)

	// 6. Respond with the loan details
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(loan)
}

// ExtendLoan handles POST /Extend
func (h *Handler) ExtendLoan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	h.store.mu.Lock()
	defer h.store.mu.Unlock()

	// Search for the active loan
	for i, loan := range h.store.Loans {
		if loan.NameOfBorrower == req.Name && loan.BookTitle == req.Title {
			// Extend by 3 weeks (21 days) from the current return date
			h.store.Loans[i].ReturnDate = loan.ReturnDate.AddDate(0, 0, 21)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(h.store.Loans[i])
			return
		}
	}

	http.Error(w, "No active loan found for this user and book", http.StatusNotFound)
}

// ReturnBook handles POST /Return
func (h *Handler) ReturnBook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Title string `json:"title"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	h.store.mu.Lock()
	defer h.store.mu.Unlock()

	for i, loan := range h.store.Loans {
		if loan.NameOfBorrower == req.Name && loan.BookTitle == req.Title {
			// 1. Remove loan from slice
			h.store.Loans = append(h.store.Loans[:i], h.store.Loans[i+1:]...)

			// 2. Increment the available copies back in the book map
			if book, ok := h.store.Books[req.Title]; ok {
				book.AvailableCopies++
			}

			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "Book returned successfully")
			return
		}
	}

	http.Error(w, "Active loan record not found", http.StatusNotFound)
}

func main() {
	// Initialize store and handlers
	store := NewLibraryStore()
	h := &Handler{store: store}

	// Initialize a new serve mux (router)
	mux := http.NewServeMux()

	// Root health check to verify the server is up
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "e-Library API is running")
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
