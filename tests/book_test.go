package tests

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"e-library/models"
	"e-library/service"
)

// TestGetBook_MissingTitle verifies the handler rejects a request with no title param.
func TestGetBook_MissingTitle(t *testing.T) {
	h := handlerWithMocks(&mockBookService{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/Book", nil)
	rr := httptest.NewRecorder()

	h.GetBook(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
	assertContentType(t, rr)
	assertErrorBody(t, rr, "Title query parameter is required")
}

// TestGetBook_ServiceErrors verifies the handler maps each service error to the
// correct HTTP status and message (OCP: add a row to cover a new error).
func TestGetBook_ServiceErrors(t *testing.T) {
	cases := []struct {
		name       string
		serviceErr error
		wantStatus int
		wantError  string
	}{
		{"not found", service.ErrBookNotFound, http.StatusNotFound, "Book not found"},
		{"unknown error", errors.New("unexpected"), http.StatusNotFound, "Book not found"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			books := &mockBookService{
				getBookFn: func(string) (models.BookDetail, error) {
					return models.BookDetail{}, tc.serviceErr
				},
			}
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/Book?title=X", nil)
			handlerWithMocks(books, nil).GetBook(rr, req)

			assertStatus(t, rr, tc.wantStatus)
			assertContentType(t, rr)
			assertErrorBody(t, rr, tc.wantError)
		})
	}
}

// TestGetBook_Success_ResponseShape verifies the response body fields on a hit.
func TestGetBook_Success_ResponseShape(t *testing.T) {
	want := models.BookDetail{Title: "Clean Code", AvailableCopies: 1}
	books := &mockBookService{
		getBookFn: func(string) (models.BookDetail, error) { return want, nil },
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/Book?title=Clean+Code", nil)
	handlerWithMocks(books, nil).GetBook(rr, req)

	assertStatus(t, rr, http.StatusOK)
	assertContentType(t, rr)

	got := decodeBody[models.BookDetail](t, rr)
	if got.Title != want.Title {
		t.Errorf("title: want %q, got %q", want.Title, got.Title)
	}
	if got.AvailableCopies != want.AvailableCopies {
		t.Errorf("available_copies: want %d, got %d", want.AvailableCopies, got.AvailableCopies)
	}
}

// TestGetBook_Integration_SeededData verifies the handler returns seeded inventory correctly.
func TestGetBook_Integration_SeededData(t *testing.T) {
	h := newIntegrationHandler()
	req := httptest.NewRequest(http.MethodGet, "/Book?title=Clean+Code", nil)
	rr := httptest.NewRecorder()

	h.GetBook(rr, req)

	assertStatus(t, rr, http.StatusOK)
	book := decodeBody[models.BookDetail](t, rr)
	if book.AvailableCopies != 1 {
		t.Errorf("expected 1 seeded copy, got %d", book.AvailableCopies)
	}
}
