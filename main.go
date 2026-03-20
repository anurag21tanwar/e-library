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

	port := ":3000"
	fmt.Printf("Server starting on port %s...\n", port)

	// Start the server
	err := http.ListenAndServe(port, mux)
	if err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
