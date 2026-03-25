package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Helper function to set up a fresh handler for each test
func setup() *Handler {
	return &Handler{store: NewLibraryStore()}
}

// assertContentType checks that the response has Content-Type: application/json
func assertContentType(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Expected Content-Type application/json, got %q", ct)
	}
}

// assertErrorBody checks that the response body is a JSON error with the expected message
func assertErrorBody(t *testing.T, rr *httptest.ResponseRecorder, wantMsg string) {
	t.Helper()
	var resp ErrorResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode error response body: %v", err)
	}
	if resp.Error != wantMsg {
		t.Errorf("Expected error %q, got %q", wantMsg, resp.Error)
	}
}

// 1. Tests for GET /Book
func TestGetBook_Success(t *testing.T) {
	h := setup()
	req := httptest.NewRequest("GET", "/Book?title=Clean+Code", nil)
	rr := httptest.NewRecorder()

	h.GetBook(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
	assertContentType(t, rr)

	var book BookDetail
	if err := json.NewDecoder(rr.Body).Decode(&book); err != nil {
		t.Fatalf("Failed to decode book response: %v", err)
	}
	if book.Title != "Clean Code" {
		t.Errorf("Expected title %q, got %q", "Clean Code", book.Title)
	}
	if book.AvailableCopies != 1 {
		t.Errorf("Expected 1 available copy, got %d", book.AvailableCopies)
	}
}

func TestGetBook_MissingTitle(t *testing.T) {
	h := setup()
	req := httptest.NewRequest("GET", "/Book", nil)
	rr := httptest.NewRecorder()

	h.GetBook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rr.Code)
	}
	assertContentType(t, rr)
	assertErrorBody(t, rr, "Title query parameter is required")
}

func TestGetBook_NotFound(t *testing.T) {
	h := setup()
	req := httptest.NewRequest("GET", "/Book?title=NonExistent", nil)
	rr := httptest.NewRecorder()

	h.GetBook(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}
	assertContentType(t, rr)
	assertErrorBody(t, rr, "Book not found")
}

// 2. Tests for POST /Borrow
func TestBorrowBook_Success(t *testing.T) {
	h := setup()
	body, _ := json.Marshal(map[string]string{"name": "Anurag", "title": "Clean Code"})
	req := httptest.NewRequest("POST", "/Borrow", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.BorrowBook(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected 201, got %d", rr.Code)
	}
	assertContentType(t, rr)

	var loan LoanDetail
	if err := json.NewDecoder(rr.Body).Decode(&loan); err != nil {
		t.Fatalf("Failed to decode loan response: %v", err)
	}
	if loan.NameOfBorrower != "Anurag" {
		t.Errorf("Expected borrower %q, got %q", "Anurag", loan.NameOfBorrower)
	}
	if loan.BookTitle != "Clean Code" {
		t.Errorf("Expected book title %q, got %q", "Clean Code", loan.BookTitle)
	}
	expectedReturn := time.Now().AddDate(0, 0, 28)
	diff := expectedReturn.Sub(loan.ReturnDate)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("Return date not ~28 days from now, got %v", loan.ReturnDate)
	}
	if h.store.Books["Clean Code"].AvailableCopies != 0 {
		t.Errorf("Expected stock to decrease to 0, got %d", h.store.Books["Clean Code"].AvailableCopies)
	}
}

func TestBorrowBook_InvalidJSON(t *testing.T) {
	h := setup()
	req := httptest.NewRequest("POST", "/Borrow", strings.NewReader("invalid-json"))
	rr := httptest.NewRecorder()

	h.BorrowBook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rr.Code)
	}
	assertContentType(t, rr)
	assertErrorBody(t, rr, "Invalid JSON payload")
}

func TestBorrowBook_MissingFields(t *testing.T) {
	h := setup()
	body, _ := json.Marshal(map[string]string{"name": "", "title": "Clean Code"})
	req := httptest.NewRequest("POST", "/Borrow", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.BorrowBook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rr.Code)
	}
	assertContentType(t, rr)
	assertErrorBody(t, rr, "Name and Title are required")
}

func TestBorrowBook_OutOfStock(t *testing.T) {
	h := setup()
	h.store.Books["Clean Code"].AvailableCopies = 0

	body, _ := json.Marshal(map[string]string{"name": "Anurag", "title": "Clean Code"})
	req := httptest.NewRequest("POST", "/Borrow", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.BorrowBook(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("Expected 409, got %d", rr.Code)
	}
	assertContentType(t, rr)
	assertErrorBody(t, rr, "No copies available for loan")
}

func TestBorrowBook_BookNotFound(t *testing.T) {
	h := setup()
	body, _ := json.Marshal(map[string]string{"name": "Anurag", "title": "Unknown Book"})
	req := httptest.NewRequest("POST", "/Borrow", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.BorrowBook(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}
	assertContentType(t, rr)
	assertErrorBody(t, rr, "Book does not exist")
}

func TestBorrowBook_DuplicateLoan(t *testing.T) {
	h := setup()
	h.store.Loans[loanKey("Anurag", "Clean Code")] = LoanDetail{NameOfBorrower: "Anurag", BookTitle: "Clean Code"}

	body, _ := json.Marshal(map[string]string{"name": "Anurag", "title": "Clean Code"})
	req := httptest.NewRequest("POST", "/Borrow", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.BorrowBook(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("Expected 409, got %d", rr.Code)
	}
	assertContentType(t, rr)
	assertErrorBody(t, rr, "User already has an active loan for this book")
}

// 3. Tests for POST /Extend
func TestExtendLoan_Success(t *testing.T) {
	h := setup()
	originalReturn := time.Now().AddDate(0, 0, 28)
	h.store.Loans[loanKey("Anurag", "Clean Code")] = LoanDetail{
		NameOfBorrower: "Anurag",
		BookTitle:      "Clean Code",
		ReturnDate:     originalReturn,
	}

	body, _ := json.Marshal(map[string]string{"name": "Anurag", "title": "Clean Code"})
	req := httptest.NewRequest("POST", "/Extend", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.ExtendLoan(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
	assertContentType(t, rr)

	var loan LoanDetail
	if err := json.NewDecoder(rr.Body).Decode(&loan); err != nil {
		t.Fatalf("Failed to decode loan response: %v", err)
	}
	expectedReturn := originalReturn.AddDate(0, 0, 21)
	diff := expectedReturn.Sub(loan.ReturnDate)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("Expected return date extended by 21 days, got %v", loan.ReturnDate)
	}
}

func TestExtendLoan_InvalidJSON(t *testing.T) {
	h := setup()
	req := httptest.NewRequest("POST", "/Extend", strings.NewReader("invalid-json"))
	rr := httptest.NewRecorder()

	h.ExtendLoan(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rr.Code)
	}
	assertContentType(t, rr)
	assertErrorBody(t, rr, "Invalid JSON payload")
}

func TestExtendLoan_NotFound(t *testing.T) {
	h := setup()
	body, _ := json.Marshal(map[string]string{"name": "Stranger", "title": "Clean Code"})
	req := httptest.NewRequest("POST", "/Extend", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.ExtendLoan(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}
	assertContentType(t, rr)
	assertErrorBody(t, rr, "No active loan found for this user and book")
}

func TestExtendLoan_MissingFields(t *testing.T) {
	h := setup()
	body, _ := json.Marshal(map[string]string{"name": "", "title": "Clean Code"})
	req := httptest.NewRequest("POST", "/Extend", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.ExtendLoan(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rr.Code)
	}
	assertContentType(t, rr)
	assertErrorBody(t, rr, "Name and Title are required")
}

// 4. Tests for POST /Return
func TestReturnBook_Success(t *testing.T) {
	h := setup()
	h.store.Loans[loanKey("Anurag", "Clean Code")] = LoanDetail{NameOfBorrower: "Anurag", BookTitle: "Clean Code"}

	body, _ := json.Marshal(map[string]string{"name": "Anurag", "title": "Clean Code"})
	req := httptest.NewRequest("POST", "/Return", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.ReturnBook(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
	assertContentType(t, rr)

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode return response: %v", err)
	}
	if resp["message"] != "Book returned successfully" {
		t.Errorf("Unexpected message: %q", resp["message"])
	}
	if h.store.Books["Clean Code"].AvailableCopies != 2 {
		t.Errorf("Expected stock to increase to 2, got %d", h.store.Books["Clean Code"].AvailableCopies)
	}
	if len(h.store.Loans) != 0 {
		t.Errorf("Expected loan to be removed, got %d loans", len(h.store.Loans))
	}
}

func TestReturnBook_MissingFields(t *testing.T) {
	h := setup()
	body, _ := json.Marshal(map[string]string{"name": "Anurag", "title": ""})
	req := httptest.NewRequest("POST", "/Return", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.ReturnBook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rr.Code)
	}
	assertContentType(t, rr)
	assertErrorBody(t, rr, "Name and Title are required")
}

func TestReturnBook_InvalidJSON(t *testing.T) {
	h := setup()
	req := httptest.NewRequest("POST", "/Return", strings.NewReader("invalid-json"))
	rr := httptest.NewRecorder()

	h.ReturnBook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rr.Code)
	}
	assertContentType(t, rr)
	assertErrorBody(t, rr, "Invalid JSON payload")
}

func TestReturnBook_NotFound(t *testing.T) {
	h := setup()
	body, _ := json.Marshal(map[string]string{"name": "Stranger", "title": "Clean Code"})
	req := httptest.NewRequest("POST", "/Return", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.ReturnBook(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rr.Code)
	}
	assertContentType(t, rr)
	assertErrorBody(t, rr, "Active loan record not found")
}
