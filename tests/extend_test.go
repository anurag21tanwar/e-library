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

// TestExtendLoan_ServiceErrors verifies the handler maps each service error to the correct HTTP response.
func TestExtendLoan_ServiceErrors(t *testing.T) {
	cases := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantError  string
	}{
		{"not found", service.ErrLoanNotFound, http.StatusNotFound, "No active loan found for this user and book"},
		{"already extended", service.ErrLoanAlreadyExtended, http.StatusConflict, "Loan has already been extended once and cannot be extended again"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			loans := &mockLoanService{
				extendFn: func(_, _ string) (models.LoanDetail, error) {
					return models.LoanDetail{}, tc.serviceErr
				},
			}
			rr := postJSON(
				handlerWithMocks(nil, loans).ExtendLoan,
				`{"name":"Anurag","title":"Clean Code"}`,
			)
			assertStatus(t, rr, tc.wantStatus)
			assertContentType(t, rr)
			assertErrorBody(t, rr, tc.wantError)
		})
	}
}

// TestExtendLoan_Integration_DeniedOnSecondExtension verifies that a second extension attempt
// on the same loan is rejected with 409.
func TestExtendLoan_Integration_DeniedOnSecondExtension(t *testing.T) {
	h := newIntegrationHandler()
	borrowBook(h, "Anurag", "Clean Code")

	// First extension — must succeed.
	rr := postJSON(h.ExtendLoan, `{"name":"Anurag","title":"Clean Code"}`)
	assertStatus(t, rr, http.StatusOK)

	// The second extension — must be denied.
	rr = postJSON(h.ExtendLoan, `{"name":"Anurag","title":"Clean Code"}`)
	assertStatus(t, rr, http.StatusConflict)
	assertContentType(t, rr)
	assertErrorBody(t, rr, "Loan has already been extended once and cannot be extended again")
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
