package tests

import (
	"net/http"
	"testing"

	"e-library/models"
	"e-library/service"
)

// TestReturnBook_ValidationErrors covers all input-validation rejections (OCP: add a row).
func TestReturnBook_ValidationErrors(t *testing.T) {
	for _, tc := range nameAndTitleValidationCases {
		t.Run(tc.name, func(t *testing.T) {
			rr := postJSON(handlerWithMocks(nil, &mockLoanService{}).ReturnBook, tc.body)
			assertStatus(t, rr, tc.wantStatus)
			assertContentType(t, rr)
			assertErrorBody(t, rr, tc.wantError)
		})
	}
}

// TestReturnBook_NotFound verifies the handler returns 404 when no active loan exists.
func TestReturnBook_NotFound(t *testing.T) {
	loans := &mockLoanService{
		returnFn: func(_, _ string) error { return service.ErrLoanNotFound },
	}
	rr := postJSON(
		handlerWithMocks(nil, loans).ReturnBook,
		`{"name":"Stranger","title":"Clean Code"}`,
	)
	assertStatus(t, rr, http.StatusNotFound)
	assertContentType(t, rr)
	assertErrorBody(t, rr, "Active loan record not found")
}

// TestReturnBook_StockRestoreFailed verifies the handler returns 200 even when
// ErrStockRestoreFailed is returned — the loan was deleted, so the user's action succeeded.
func TestReturnBook_StockRestoreFailed(t *testing.T) {
	loans := &mockLoanService{
		returnFn: func(_, _ string) error { return service.ErrStockRestoreFailed },
	}
	rr := postJSON(
		handlerWithMocks(nil, loans).ReturnBook,
		`{"name":"Anurag","title":"Clean Code"}`,
	)
	assertStatus(t, rr, http.StatusOK)
	assertContentType(t, rr)

	resp := decodeBody[map[string]string](t, rr)
	if resp["message"] != "Book returned successfully" {
		t.Errorf("unexpected message: %q", resp["message"])
	}
}

// TestReturnBook_Integration_SuccessMessage verifies the response body message on success.
func TestReturnBook_Integration_SuccessMessage(t *testing.T) {
	h := newIntegrationHandler()
	borrowBook(h, "Anurag", "Clean Code")

	rr := postJSON(h.ReturnBook, `{"name":"Anurag","title":"Clean Code"}`)

	assertStatus(t, rr, http.StatusOK)
	assertContentType(t, rr)
	resp := decodeBody[map[string]string](t, rr)
	if resp["message"] != "Book returned successfully" {
		t.Errorf("unexpected message: %q", resp["message"])
	}
}

// TestReturnBook_Integration_RestoresStock verifies available copies increase after return.
func TestReturnBook_Integration_RestoresStock(t *testing.T) {
	h := newIntegrationHandler()
	borrowBook(h, "Anurag", "Clean Code")
	postJSON(h.ReturnBook, `{"name":"Anurag","title":"Clean Code"}`)

	req, rr := newGetBookRequest("Clean Code")
	h.GetBook(rr, req)
	book := decodeBody[models.BookDetail](t, rr)

	if book.AvailableCopies != 1 {
		t.Errorf("expected 1 available copy after return, got %d", book.AvailableCopies)
	}
}

// TestReturnBook_Integration_LoanDeleted verifies the loan record is removed after return.
func TestReturnBook_Integration_LoanDeleted(t *testing.T) {
	h := newIntegrationHandler()
	borrowBook(h, "Anurag", "Clean Code")
	postJSON(h.ReturnBook, `{"name":"Anurag","title":"Clean Code"}`)

	// Attempting to extend the now-deleted loan must return 404.
	rr := postJSON(h.ExtendLoan, `{"name":"Anurag","title":"Clean Code"}`)
	assertStatus(t, rr, http.StatusNotFound)
}
