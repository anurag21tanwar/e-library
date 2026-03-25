// Package repository provides the in-memory data store for the e-Library API.
package repository

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"e-library/models"
)

// Sentinel errors returned by store methods. Handlers map these to HTTP status codes.
var (
	ErrBookNotFound  = errors.New("book not found")
	ErrNoStock       = errors.New("no copies available for loan")
	ErrDuplicateLoan = errors.New("user already has an active loan for this book")
	ErrLoanNotFound  = errors.New("no active loan found")
)

// Store is the interface that repository implementations must satisfy.
// Using an interface allows handlers to be tested with a fake/mock store.
type Store interface {
	GetBook(title string) (models.BookDetail, error)
	BorrowBook(name, title string) (models.LoanDetail, error)
	ExtendLoan(name, title string) (models.LoanDetail, error)
	ReturnBook(name, title string) error
}

// LibraryStore is the in-memory implementation of Store.
// All fields are unexported; callers interact exclusively through the Store interface.
type LibraryStore struct {
	mu     sync.RWMutex
	books  map[string]*models.BookDetail
	loans  map[string]models.LoanDetail // keyed by loanKey(name, title)
	logger *slog.Logger
}

// loanKey produces a composite map key for a (borrower, book) pair.
// A null-byte separator prevents collisions where concatenated strings would be equal
// (e.g. name="ab", title="cd" vs name="a", title="bcd").
func loanKey(name, title string) string {
	return name + "\x00" + title
}

// NewLibraryStore initialises the store and seeds it with sample books.
func NewLibraryStore(logger *slog.Logger) *LibraryStore {
	s := &LibraryStore{
		books:  make(map[string]*models.BookDetail),
		loans:  make(map[string]models.LoanDetail),
		logger: logger,
	}

	s.books["The Go Programming Language"] = &models.BookDetail{
		Title:           "The Go Programming Language",
		AvailableCopies: 3,
	}
	s.books["Clean Code"] = &models.BookDetail{
		Title:           "Clean Code",
		AvailableCopies: 1,
	}

	return s
}

// GetBook returns a snapshot of the book with the given title.
// The lock is released before returning so it is never held during response writes.
func (s *LibraryStore) GetBook(title string) (models.BookDetail, error) {
	s.mu.RLock()
	b, ok := s.books[title]
	var snap models.BookDetail
	if ok {
		snap = *b // copy value under lock to prevent data race after unlock
	}
	s.mu.RUnlock()

	if !ok {
		return models.BookDetail{}, ErrBookNotFound
	}
	return snap, nil
}

// BorrowBook creates a 28-day loan for (name, title) if the book exists, has stock,
// and the user does not already hold an active loan for the same book.
// The lock is released before returning.
func (s *LibraryStore) BorrowBook(name, title string) (models.LoanDetail, error) {
	s.mu.Lock()

	book, exists := s.books[title]
	if !exists {
		s.mu.Unlock()
		return models.LoanDetail{}, ErrBookNotFound
	}
	if book.AvailableCopies <= 0 {
		s.mu.Unlock()
		return models.LoanDetail{}, ErrNoStock
	}
	key := loanKey(name, title)
	if _, dup := s.loans[key]; dup {
		s.mu.Unlock()
		return models.LoanDetail{}, ErrDuplicateLoan
	}

	// Atomic update: decrement stock and record the loan together.
	book.AvailableCopies--
	now := time.Now()
	loan := models.LoanDetail{
		NameOfBorrower: name,
		BookTitle:      title,
		LoanDate:       now,
		ReturnDate:     now.AddDate(0, 0, 28),
	}
	s.loans[key] = loan

	s.mu.Unlock()
	return loan, nil
}

// ExtendLoan extends the return date of an active loan by 21 days.
// The lock is released before returning.
func (s *LibraryStore) ExtendLoan(name, title string) (models.LoanDetail, error) {
	s.mu.Lock()

	key := loanKey(name, title)
	loan, exists := s.loans[key]
	if !exists {
		s.mu.Unlock()
		return models.LoanDetail{}, ErrLoanNotFound
	}
	loan.ReturnDate = loan.ReturnDate.AddDate(0, 0, 21)
	s.loans[key] = loan

	s.mu.Unlock()
	return loan, nil
}

// ReturnBook removes the active loan for (name, title) and restores one copy to stock.
// The lock is released before returning.
func (s *LibraryStore) ReturnBook(name, title string) error {
	s.mu.Lock()

	key := loanKey(name, title)
	if _, exists := s.loans[key]; !exists {
		s.mu.Unlock()
		return ErrLoanNotFound
	}
	delete(s.loans, key)

	if book, ok := s.books[title]; ok {
		book.AvailableCopies++
	} else {
		// Invariant violation: loan existed but book record is missing.
		s.logger.Error("book missing from store after loan deletion — stock not restored", "title", title)
	}

	s.mu.Unlock()
	return nil
}
