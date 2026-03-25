package tests

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"e-library/models"
	"e-library/service"
)

// TestBorrowBook_ValidationErrors covers all input-validation rejections (OCP: add a row).
func TestBorrowBook_ValidationErrors(t *testing.T) {
	for _, tc := range nameAndTitleValidationCases {
		t.Run(tc.name, func(t *testing.T) {
			rr := postJSON(handlerWithMocks(nil, &mockLoanService{}).BorrowBook, tc.body)
			assertStatus(t, rr, tc.wantStatus)
			assertContentType(t, rr)
			assertErrorBody(t, rr, tc.wantError)
		})
	}
}

// TestBorrowBook_ServiceErrors verifies every service error maps to the right HTTP response.
func TestBorrowBook_ServiceErrors(t *testing.T) {
	cases := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantError  string
	}{
		{"book not found", service.ErrBookNotFound, http.StatusNotFound, "Book does not exist"},
		{"no stock", service.ErrNoStock, http.StatusConflict, "No copies available for loan"},
		{"duplicate loan", service.ErrDuplicateLoan, http.StatusConflict, "User already has an active loan for this book"},
		{"unknown error", errors.New("db error"), http.StatusInternalServerError, "Internal server error"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			loans := &mockLoanService{
				borrowFn: func(_, _ string) (models.LoanDetail, error) {
					return models.LoanDetail{}, tc.serviceErr
				},
			}
			rr := postJSON(
				handlerWithMocks(nil, loans).BorrowBook,
				`{"name":"Anurag","title":"Clean Code"}`,
			)
			assertStatus(t, rr, tc.wantStatus)
			assertContentType(t, rr)
			assertErrorBody(t, rr, tc.wantError)
		})
	}
}

// TestBorrowBook_Integration_LoanShape verifies the response body contains correct loan fields.
func TestBorrowBook_Integration_LoanShape(t *testing.T) {
	rr := borrowBook(newIntegrationHandler(), "Anurag", "Clean Code")

	assertStatus(t, rr, http.StatusCreated)
	assertContentType(t, rr)

	loan := decodeBody[models.LoanDetail](t, rr)
	if loan.NameOfBorrower != "Anurag" {
		t.Errorf("borrower: want %q, got %q", "Anurag", loan.NameOfBorrower)
	}
	if loan.BookTitle != "Clean Code" {
		t.Errorf("book title: want %q, got %q", "Clean Code", loan.BookTitle)
	}
	if diff := time.Now().AddDate(0, 0, 28).Sub(loan.ReturnDate); diff < -time.Second || diff > time.Second {
		t.Errorf("return date not ~28 days from now, got %v", loan.ReturnDate)
	}
}

// TestBorrowBook_Integration_ReducesStock verifies available copies decrease after a borrow.
func TestBorrowBook_Integration_ReducesStock(t *testing.T) {
	h := newIntegrationHandler()
	borrowBook(h, "Anurag", "Clean Code")

	req, rr := newGetBookRequest("Clean Code")
	h.GetBook(rr, req)
	book := decodeBody[models.BookDetail](t, rr)

	if book.AvailableCopies != 0 {
		t.Errorf("expected 0 available copies after borrow, got %d", book.AvailableCopies)
	}
}
