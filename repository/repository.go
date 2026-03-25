// Package repository provides the in-memory data store for the e-Library API.
package repository

import (
	"errors"
	"sync"

	"e-library/models"
)

// Sentinel errors returned by store methods. The service layer maps these to domain errors.
var (
	ErrBookNotFound        = errors.New("book not found")
	ErrDuplicateBook       = errors.New("a book with that title already exists")
	ErrNoStock             = errors.New("no copies available for loan")
	ErrDuplicateLoan       = errors.New("user already has an active loan for this book")
	ErrLoanNotFound        = errors.New("no active loan found")
	ErrLoanAlreadyExtended = errors.New("loan has already been extended once")
)

// Store is the interface that repository implementations must satisfy.
// Method names describe the operation, not the implementation mechanism (LSP).
type Store interface {
	GetBook(title string) (models.BookDetail, error)
	CreateLoan(loan models.LoanDetail) error
	UpdateLoanExpiry(name, title string, days int) (models.LoanDetail, error)
	DeleteLoan(name, title string) error
	IncrementStock(title string) error
}

// LibraryStore is the in-memory implementation of Store.
type LibraryStore struct {
	mu    sync.RWMutex
	books map[string]*models.BookDetail
	loans map[string]models.LoanDetail // keyed by loanKey(name, title)
}

// loanKey produces a composite map key for a (borrower, book) pair.
// A null-byte separator prevents collisions between names/titles that share a prefix.
func loanKey(name, title string) string {
	return name + "\x00" + title
}

// NewLibraryStore initialises an empty store. Seed data is the caller's responsibility.
func NewLibraryStore() *LibraryStore {
	return &LibraryStore{
		books: make(map[string]*models.BookDetail),
		loans: make(map[string]models.LoanDetail),
	}
}

// AddBook inserts a book into the store. Returns ErrDuplicateBook if a book
// with the same title already exists. Title uniqueness is enforced here because
// the title is the primary key used throughout the system.
// This is intentionally not part of Store — only used at startup to seed data.
func (s *LibraryStore) AddBook(book models.BookDetail) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.books[book.Title]; exists {
		return ErrDuplicateBook
	}
	s.books[book.Title] = &book
	return nil
}

// GetBook returns a snapshot of the book with the given title.
func (s *LibraryStore) GetBook(title string) (models.BookDetail, error) {
	s.mu.RLock()
	b, ok := s.books[title]
	var snap models.BookDetail
	if ok {
		snap = *b // copy under lock to prevent data race after unlock
	}
	s.mu.RUnlock()

	if !ok {
		return models.BookDetail{}, ErrBookNotFound
	}
	return snap, nil
}

// CreateLoan atomically verifies the book exists, has available stock, and the borrower
// has no duplicate active loan, then decrements stock and stores the loan.
// The caller is responsible for populating all LoanDetail fields including dates.
func (s *LibraryStore) CreateLoan(loan models.LoanDetail) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	book, exists := s.books[loan.BookTitle]
	if !exists {
		return ErrBookNotFound
	}
	if book.AvailableCopies <= 0 {
		return ErrNoStock
	}
	key := loanKey(loan.NameOfBorrower, loan.BookTitle)
	if _, dup := s.loans[key]; dup {
		return ErrDuplicateLoan
	}

	book.AvailableCopies--
	s.loans[key] = loan
	return nil
}

// UpdateLoanExpiry atomically adds the given number of days to an active loan's return
// date and returns the updated record. Returns ErrLoanAlreadyExtended if the loan has
// already been extended once. The service layer provides the number of days.
func (s *LibraryStore) UpdateLoanExpiry(name, title string, days int) (models.LoanDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := loanKey(name, title)
	loan, exists := s.loans[key]
	if !exists {
		return models.LoanDetail{}, ErrLoanNotFound
	}
	if loan.Extended {
		return models.LoanDetail{}, ErrLoanAlreadyExtended
	}
	loan.ReturnDate = loan.ReturnDate.AddDate(0, 0, days)
	loan.Extended = true
	s.loans[key] = loan
	return loan, nil
}

// DeleteLoan removes the active loan for (name, title).
func (s *LibraryStore) DeleteLoan(name, title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := loanKey(name, title)
	if _, exists := s.loans[key]; !exists {
		return ErrLoanNotFound
	}
	delete(s.loans, key)
	return nil
}

// IncrementStock adds one available copy back to the book's stock.
func (s *LibraryStore) IncrementStock(title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	book, ok := s.books[title]
	if !ok {
		return ErrBookNotFound
	}
	book.AvailableCopies++
	return nil
}
