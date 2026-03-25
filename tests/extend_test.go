package tests

import (
	"net/http"
	"testing"
	"time"

	"e-library/models"
	"e-library/service"
)

// TestExtendLoan_ValidationErrors covers all input-validation rejections (OCP: add a row).
func TestExtendLoan_ValidationErrors(t *testing.T) {
	for _, tc := range nameAndTitleValidationCases {
		t.Run(tc.name, func(t *testing.T) {
			rr := postJSON(handlerWithMocks(nil, &mockLoanService{}).ExtendLoan, tc.body)
			assertStatus(t, rr, tc.wantStatus)
			assertContentType(t, rr)
			assertErrorBody(t, rr, tc.wantError)
		})
	}
}

// TestExtendLoan_NotFound verifies the handler returns 404 when no active loan exists.
func TestExtendLoan_NotFound(t *testing.T) {
	loans := &mockLoanService{
		extendFn: func(_, _ string) (models.LoanDetail, error) {
			return models.LoanDetail{}, service.ErrLoanNotFound
		},
	}
	rr := postJSON(
		handlerWithMocks(nil, loans).ExtendLoan,
		`{"name":"Stranger","title":"Clean Code"}`,
	)
	assertStatus(t, rr, http.StatusNotFound)
	assertContentType(t, rr)
	assertErrorBody(t, rr, "No active loan found for this user and book")
}

// TestExtendLoan_Integration_ExtendsBy21Days verifies the return date is pushed out by 21 days.
func TestExtendLoan_Integration_ExtendsBy21Days(t *testing.T) {
	h := newIntegrationHandler()
	br := borrowBook(h, "Anurag", "Clean Code")
	original := decodeBody[models.LoanDetail](t, br)

	rr := postJSON(h.ExtendLoan, `{"name":"Anurag","title":"Clean Code"}`)

	assertStatus(t, rr, http.StatusOK)
	assertContentType(t, rr)

	extended := decodeBody[models.LoanDetail](t, rr)
	want := original.ReturnDate.AddDate(0, 0, 21)
	if diff := want.Sub(extended.ReturnDate); diff < -time.Second || diff > time.Second {
		t.Errorf("expected return date extended by 21 days, got %v", extended.ReturnDate)
	}
}
