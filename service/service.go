// Package service implements the business logic layer for the e-Library API.
// It sits between the HTTP handlers and the repository, owning all domain rules.
package service

import (
	"errors"
	"log/slog"
	"time"

	"e-library/models"
	"e-library/repository"
)

// Business rule constants — the only place loan periods are defined.
const (
	loanDurationDays = 28
	extensionDays    = 21
)

// BookService covers read-only book queries.
// Callers that only read books depend solely on this narrow interface (ISP).
type BookService interface {
	GetBook(title string) (models.BookDetail, error)
}

// LoanService covers all loan lifecycle operations.
// Callers that only manage loans depend solely on this narrow interface (ISP).
type LoanService interface {
	BorrowBook(name, title string) (models.LoanDetail, error)
	ExtendLoan(name, title string) (models.LoanDetail, error)
	ReturnBook(name, title string) error
}

type libraryService struct {
	store  repository.Store
	logger *slog.Logger
}

// New returns a *libraryService that satisfies both BookService and LoanService.
// Callers assign the result to whichever interface(s) they need.
func New(store repository.Store, logger *slog.Logger) *libraryService {
	return &libraryService{store: store, logger: logger}
}

func (s *libraryService) GetBook(title string) (models.BookDetail, error) {
	book, err := s.store.GetBook(title)
	if err != nil {
		if errors.Is(err, repository.ErrBookNotFound) {
			return models.BookDetail{}, ErrBookNotFound
		}
		return models.BookDetail{}, err
	}
	return book, nil
}

// BorrowBook builds a loan with the correct dates and delegates atomically to the store.
func (s *libraryService) BorrowBook(name, title string) (models.LoanDetail, error) {
	now := time.Now()
	loan := models.LoanDetail{
		NameOfBorrower: name,
		BookTitle:      title,
		LoanDate:       now,
		ReturnDate:     now.AddDate(0, 0, loanDurationDays),
	}
	if err := s.store.CreateLoan(loan); err != nil {
		switch {
		case errors.Is(err, repository.ErrBookNotFound):
			return models.LoanDetail{}, ErrBookNotFound
		case errors.Is(err, repository.ErrNoStock):
			return models.LoanDetail{}, ErrNoStock
		case errors.Is(err, repository.ErrDuplicateLoan):
			return models.LoanDetail{}, ErrDuplicateLoan
		default:
			return models.LoanDetail{}, err
		}
	}
	return loan, nil
}

// ExtendLoan extends the active loan's return date by extensionDays.
func (s *libraryService) ExtendLoan(name, title string) (models.LoanDetail, error) {
	loan, err := s.store.UpdateLoanExpiry(name, title, extensionDays)
	if err != nil {
		if errors.Is(err, repository.ErrLoanNotFound) {
			return models.LoanDetail{}, ErrLoanNotFound
		}
		return models.LoanDetail{}, err
	}
	return loan, nil
}

// ReturnBook deletes the active loan and restores the book's stock.
// Returns ErrLoanNotFound if no active loan exists.
// Returns ErrStockRestoreFailed if the loan was deleted but stock could not be restored
// (invariant violation — logged here for observability).
func (s *libraryService) ReturnBook(name, title string) error {
	if err := s.store.DeleteLoan(name, title); err != nil {
		if errors.Is(err, repository.ErrLoanNotFound) {
			return ErrLoanNotFound
		}
		return err
	}
	if err := s.store.IncrementStock(title); err != nil {
		s.logger.Error("book missing from store after loan deletion — stock not restored",
			"title", title)
		return ErrStockRestoreFailed
	}
	return nil
}
