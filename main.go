package main

import (
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

func main() {
	// Initialize a new serve mux (router)
	mux := http.NewServeMux()

	// Root health check to verify the server is up
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "e-Library API is running")
	})

	port := ":3000"
	fmt.Printf("Server starting on port %s...\n", port)

	// Start the server
	err := http.ListenAndServe(port, mux)
	if err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
