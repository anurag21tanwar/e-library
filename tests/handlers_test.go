package tests

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"e-library/handlers"
	"e-library/models"
	"e-library/repository"
)

// setup creates a fresh handler backed by a real in-memory store for each test.
func setup() *handlers.Handler {
	logger := slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError + 1}))
	store := repository.NewLibraryStore(logger)
	return handlers.NewHandler(store, logger)
}

// borrowBook is a test helper that calls BorrowBook and returns the recorder.
func borrowBook(h *handlers.Handler, name, title string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(map[string]string{"name": name, "title": title})
	req := httptest.NewRequest(http.MethodPost, "/Borrow", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()
	h.BorrowBook(rr, req)
	return rr
}

// assertStatus fails the test if the response code does not match.
func assertStatus(t *testing.T, rr *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rr.Code != want {
		t.Errorf("expected status %d, got %d", want, rr.Code)
	}
}

// assertContentType fails the test if the response is not application/json.
func assertContentType(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

// assertErrorBody decodes the JSON error envelope and checks the message.
func assertErrorBody(t *testing.T, rr *httptest.ResponseRecorder, wantMsg string) {
	t.Helper()
	var resp handlers.ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if resp.Error != wantMsg {
		t.Errorf("expected error %q, got %q", wantMsg, resp.Error)
	}
}

// --- GET /Book ---

func TestGetBook_Success(t *testing.T) {
	h := setup()
	req := httptest.NewRequest(http.MethodGet, "/Book?title=Clean+Code", nil)
	rr := httptest.NewRecorder()

	h.GetBook(rr, req)

	assertStatus(t, rr, http.StatusOK)
	assertContentType(t, rr)

	var book models.BookDetail
	if err := json.NewDecoder(rr.Body).Decode(&book); err != nil {
		t.Fatalf("failed to decode book response: %v", err)
	}
	if book.Title != "Clean Code" {
		t.Errorf("expected title %q, got %q", "Clean Code", book.Title)
	}
	if book.AvailableCopies != 1 {
		t.Errorf("expected 1 available copy, got %d", book.AvailableCopies)
	}
}

func TestGetBook_MissingTitle(t *testing.T) {
	h := setup()
	req := httptest.NewRequest(http.MethodGet, "/Book", nil)
	rr := httptest.NewRecorder()

	h.GetBook(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
	assertContentType(t, rr)
	assertErrorBody(t, rr, "Title query parameter is required")
}

func TestGetBook_NotFound(t *testing.T) {
	h := setup()
	req := httptest.NewRequest(http.MethodGet, "/Book?title=NonExistent", nil)
	rr := httptest.NewRecorder()

	h.GetBook(rr, req)

	assertStatus(t, rr, http.StatusNotFound)
	assertContentType(t, rr)
	assertErrorBody(t, rr, "Book not found")
}

// --- POST /Borrow ---

func TestBorrowBook_Success(t *testing.T) {
	h := setup()
	rr := borrowBook(h, "Anurag", "Clean Code")

	assertStatus(t, rr, http.StatusCreated)
	assertContentType(t, rr)

	var loan models.LoanDetail
	if err := json.NewDecoder(rr.Body).Decode(&loan); err != nil {
		t.Fatalf("failed to decode loan response: %v", err)
	}
	if loan.NameOfBorrower != "Anurag" {
		t.Errorf("expected borrower %q, got %q", "Anurag", loan.NameOfBorrower)
	}
	if loan.BookTitle != "Clean Code" {
		t.Errorf("expected book %q, got %q", "Clean Code", loan.BookTitle)
	}
	diff := time.Now().AddDate(0, 0, 28).Sub(loan.ReturnDate)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("return date not ~28 days from now, got %v", loan.ReturnDate)
	}

	// Verify available copies decreased via GetBook (no direct store access).
	req := httptest.NewRequest(http.MethodGet, "/Book?title=Clean+Code", nil)
	gr := httptest.NewRecorder()
	h.GetBook(gr, req)
	var book models.BookDetail
	_ = json.NewDecoder(gr.Body).Decode(&book)
	if book.AvailableCopies != 0 {
		t.Errorf("expected 0 available copies after borrow, got %d", book.AvailableCopies)
	}
}

func TestBorrowBook_InvalidJSON(t *testing.T) {
	h := setup()
	req := httptest.NewRequest(http.MethodPost, "/Borrow", strings.NewReader("invalid-json"))
	rr := httptest.NewRecorder()

	h.BorrowBook(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
	assertContentType(t, rr)
	assertErrorBody(t, rr, "Invalid JSON payload")
}

func TestBorrowBook_MissingFields(t *testing.T) {
	h := setup()
	body, _ := json.Marshal(map[string]string{"name": "", "title": "Clean Code"})
	req := httptest.NewRequest(http.MethodPost, "/Borrow", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.BorrowBook(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
	assertContentType(t, rr)
	assertErrorBody(t, rr, "Name and Title are required")
}

func TestBorrowBook_OutOfStock(t *testing.T) {
	h := setup()
	// Exhaust the single copy of "Clean Code" with a legitimate borrow.
	borrowBook(h, "FirstUser", "Clean Code")

	// A second user attempting to borrow should get a 409.
	rr := borrowBook(h, "SecondUser", "Clean Code")

	assertStatus(t, rr, http.StatusConflict)
	assertContentType(t, rr)
	assertErrorBody(t, rr, "No copies available for loan")
}

func TestBorrowBook_BookNotFound(t *testing.T) {
	h := setup()
	rr := borrowBook(h, "Anurag", "Unknown Book")

	assertStatus(t, rr, http.StatusNotFound)
	assertContentType(t, rr)
	assertErrorBody(t, rr, "Book does not exist")
}

func TestBorrowBook_DuplicateLoan(t *testing.T) {
	h := setup()
	// First borrow succeeds.
	borrowBook(h, "Anurag", "The Go Programming Language")

	// Same user borrowing the same book again should get a 409.
	rr := borrowBook(h, "Anurag", "The Go Programming Language")

	assertStatus(t, rr, http.StatusConflict)
	assertContentType(t, rr)
	assertErrorBody(t, rr, "User already has an active loan for this book")
}

// --- POST /Extend ---

func TestExtendLoan_Success(t *testing.T) {
	h := setup()
	// Create a loan first so there is something to extend.
	br := borrowBook(h, "Anurag", "Clean Code")
	var original models.LoanDetail
	_ = json.NewDecoder(br.Body).Decode(&original)

	body, _ := json.Marshal(map[string]string{"name": "Anurag", "title": "Clean Code"})
	req := httptest.NewRequest(http.MethodPost, "/Extend", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.ExtendLoan(rr, req)

	assertStatus(t, rr, http.StatusOK)
	assertContentType(t, rr)

	var extended models.LoanDetail
	if err := json.NewDecoder(rr.Body).Decode(&extended); err != nil {
		t.Fatalf("failed to decode extend response: %v", err)
	}
	expected := original.ReturnDate.AddDate(0, 0, 21)
	diff := expected.Sub(extended.ReturnDate)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("expected return date extended by 21 days, got %v", extended.ReturnDate)
	}
}

func TestExtendLoan_InvalidJSON(t *testing.T) {
	h := setup()
	req := httptest.NewRequest(http.MethodPost, "/Extend", strings.NewReader("invalid-json"))
	rr := httptest.NewRecorder()

	h.ExtendLoan(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
	assertContentType(t, rr)
	assertErrorBody(t, rr, "Invalid JSON payload")
}

func TestExtendLoan_MissingFields(t *testing.T) {
	h := setup()
	body, _ := json.Marshal(map[string]string{"name": "", "title": "Clean Code"})
	req := httptest.NewRequest(http.MethodPost, "/Extend", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.ExtendLoan(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
	assertContentType(t, rr)
	assertErrorBody(t, rr, "Name and Title are required")
}

func TestExtendLoan_NotFound(t *testing.T) {
	h := setup()
	body, _ := json.Marshal(map[string]string{"name": "Stranger", "title": "Clean Code"})
	req := httptest.NewRequest(http.MethodPost, "/Extend", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.ExtendLoan(rr, req)

	assertStatus(t, rr, http.StatusNotFound)
	assertContentType(t, rr)
	assertErrorBody(t, rr, "No active loan found for this user and book")
}

// --- POST /Return ---

func TestReturnBook_Success(t *testing.T) {
	h := setup()
	// Create a loan so there is something to return.
	borrowBook(h, "Anurag", "Clean Code")

	body, _ := json.Marshal(map[string]string{"name": "Anurag", "title": "Clean Code"})
	req := httptest.NewRequest(http.MethodPost, "/Return", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.ReturnBook(rr, req)

	assertStatus(t, rr, http.StatusOK)
	assertContentType(t, rr)

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode return response: %v", err)
	}
	if resp["message"] != "Book returned successfully" {
		t.Errorf("unexpected message: %q", resp["message"])
	}

	// Verify available copies restored via GetBook.
	req = httptest.NewRequest(http.MethodGet, "/Book?title=Clean+Code", nil)
	gr := httptest.NewRecorder()
	h.GetBook(gr, req)
	var book models.BookDetail
	_ = json.NewDecoder(gr.Body).Decode(&book)
	if book.AvailableCopies != 1 {
		t.Errorf("expected 1 available copy after return, got %d", book.AvailableCopies)
	}

	// Verify loan is gone by attempting to extend it — should return 404.
	body, _ = json.Marshal(map[string]string{"name": "Anurag", "title": "Clean Code"})
	req = httptest.NewRequest(http.MethodPost, "/Extend", bytes.NewBuffer(body))
	er := httptest.NewRecorder()
	h.ExtendLoan(er, req)
	if er.Code != http.StatusNotFound {
		t.Errorf("expected loan to be removed after return, got status %d", er.Code)
	}
}

func TestReturnBook_MissingFields(t *testing.T) {
	h := setup()
	body, _ := json.Marshal(map[string]string{"name": "Anurag", "title": ""})
	req := httptest.NewRequest(http.MethodPost, "/Return", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.ReturnBook(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
	assertContentType(t, rr)
	assertErrorBody(t, rr, "Name and Title are required")
}

func TestReturnBook_InvalidJSON(t *testing.T) {
	h := setup()
	req := httptest.NewRequest(http.MethodPost, "/Return", strings.NewReader("invalid-json"))
	rr := httptest.NewRecorder()

	h.ReturnBook(rr, req)

	assertStatus(t, rr, http.StatusBadRequest)
	assertContentType(t, rr)
	assertErrorBody(t, rr, "Invalid JSON payload")
}

func TestReturnBook_NotFound(t *testing.T) {
	h := setup()
	body, _ := json.Marshal(map[string]string{"name": "Stranger", "title": "Clean Code"})
	req := httptest.NewRequest(http.MethodPost, "/Return", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.ReturnBook(rr, req)

	assertStatus(t, rr, http.StatusNotFound)
	assertContentType(t, rr)
	assertErrorBody(t, rr, "Active loan record not found")
}
