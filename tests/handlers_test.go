// Package tests contain integration and unit tests for the e-Library HTTP handlers.
// Files are split by handler (SRP): one file per endpoint group.
//
//	handlers_test.go  — shared infrastructure: fakes, setup, assertion helpers
//	book_test.go      — GET /Book
//	borrow_test.go    — POST /Borrow
//	extend_test.go    — POST /Extend
//	return_test.go    — POST /Return
package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"e-library/handlers"
	"e-library/models"
	"e-library/repository"
	"e-library/respond"
	"e-library/service"
)

// mockBookService implements service.BookService.
type mockBookService struct {
	getBookFn func(string) (models.BookDetail, error)
}

func (m *mockBookService) GetBook(title string) (models.BookDetail, error) {
	if m.getBookFn != nil {
		return m.getBookFn(title)
	}
	return models.BookDetail{}, nil
}

// mockLoanService implements service.LoanService.
type mockLoanService struct {
	borrowFn func(string, string) (models.LoanDetail, error)
	extendFn func(string, string) (models.LoanDetail, error)
	returnFn func(string, string) error
}

func (m *mockLoanService) BorrowBook(name, title string) (models.LoanDetail, error) {
	if m.borrowFn != nil {
		return m.borrowFn(name, title)
	}
	return models.LoanDetail{}, nil
}

func (m *mockLoanService) ExtendLoan(name, title string) (models.LoanDetail, error) {
	if m.extendFn != nil {
		return m.extendFn(name, title)
	}
	return models.LoanDetail{}, nil
}

func (m *mockLoanService) ReturnBook(name, title string) error {
	if m.returnFn != nil {
		return m.returnFn(name, title)
	}
	return nil
}

// =============================================================================
// Setup functions
// =============================================================================

// mustAddBook seeds a book into the store, panicking if the title is a duplicate.
func mustAddBook(store *repository.LibraryStore, book models.BookDetail) {
	if err := store.AddBook(book); err != nil {
		panic(fmt.Sprintf("test setup: failed to add book %q: %v", book.Title, err))
	}
}

// newIntegrationHandler returns a Handler wired to a real in-memory store.
// Use this when the test needs to verify end-to-end state changes.
func newIntegrationHandler() *handlers.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := repository.NewLibraryStore()
	mustAddBook(store, models.BookDetail{Title: "The Go Programming Language", AvailableCopies: 3})
	mustAddBook(store, models.BookDetail{Title: "Clean Code", AvailableCopies: 1})
	svc := service.New(store, logger)
	return handlers.NewHandler(svc, svc, logger)
}

// handlerWithMocks returns a Handler wired to the given mock services.
// Pass nil for either service when it is not exercised by the test.
func handlerWithMocks(books service.BookService, loans service.LoanService) *handlers.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return handlers.NewHandler(books, loans, logger)
}

// =============================================================================
// Request helpers
// =============================================================================

// postJSON sends a POST request to fn with the given body string.
func postJSON(fn func(http.ResponseWriter, *http.Request), body string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	fn(rr, req)
	return rr
}

// newGetBookRequest builds a GET /Book?title=<title> request and recorder.
// The title is query-escaped so titles with spaces are handled correctly.
func newGetBookRequest(title string) (*http.Request, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(http.MethodGet, "/Book?title="+url.QueryEscape(title), nil)
	return req, httptest.NewRecorder()
}

// borrowBook is an integration helper that sends a valid BorrowBook request.
func borrowBook(h *handlers.Handler, name, title string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(map[string]string{"name": name, "title": title})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/Borrow", bytes.NewReader(b))
	h.BorrowBook(rr, req)
	return rr
}

// =============================================================================
// Assertion helpers
// =============================================================================

func assertStatus(t *testing.T, rr *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rr.Code != want {
		t.Errorf("expected status %d, got %d", want, rr.Code)
	}
}

func assertContentType(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func assertErrorBody(t *testing.T, rr *httptest.ResponseRecorder, wantMsg string) {
	t.Helper()
	var body respond.ErrorBody
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if body.Error != wantMsg {
		t.Errorf("expected error %q, got %q", wantMsg, body.Error)
	}
}

// decodeBody decodes the response body into T, failing the test on error.
func decodeBody[T any](t *testing.T, rr *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(rr.Body).Decode(&v); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	return v
}

// nameAndTitleValidationCases covers the two validation rules common to all
// POST endpoints that accept {"name", "title"} bodies.
var nameAndTitleValidationCases = []struct {
	name       string
	body       string
	wantStatus int
	wantError  string
}{
	{
		name:       "invalid JSON",
		body:       "not-json",
		wantStatus: http.StatusBadRequest,
		wantError:  "Invalid JSON payload",
	},
	{
		name:       "empty name",
		body:       `{"name":"","title":"Clean Code"}`,
		wantStatus: http.StatusBadRequest,
		wantError:  "name and title are required",
	},
	{
		name:       "empty title",
		body:       `{"name":"Anurag","title":""}`,
		wantStatus: http.StatusBadRequest,
		wantError:  "name and title are required",
	},
}
