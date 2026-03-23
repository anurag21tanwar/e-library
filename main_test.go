package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Helper function to set up a fresh handler for each test
func setup() *Handler {
	return &Handler{store: NewLibraryStore()}
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
}

func TestGetBook_MissingTitle(t *testing.T) {
	h := setup()
	req := httptest.NewRequest("GET", "/Book", nil) // No query param
	rr := httptest.NewRecorder()

	h.GetBook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing title, got %d", rr.Code)
	}
}

func TestGetBook_NotFound(t *testing.T) {
	h := setup()
	req := httptest.NewRequest("GET", "/Book?title=NonExistent", nil)
	rr := httptest.NewRecorder()

	h.GetBook(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for missing book, got %d", rr.Code)
	}
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
}

func TestBorrowBook_InvalidJSON(t *testing.T) {
	h := setup()
	req := httptest.NewRequest("POST", "/Borrow", strings.NewReader("invalid-json"))
	rr := httptest.NewRecorder()

	h.BorrowBook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for bad JSON, got %d", rr.Code)
	}
}

func TestBorrowBook_MissingFields(t *testing.T) {
	h := setup()
	body, _ := json.Marshal(map[string]string{"name": "", "title": "Clean Code"})
	req := httptest.NewRequest("POST", "/Borrow", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.BorrowBook(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing name, got %d", rr.Code)
	}
}

func TestBorrowBook_OutOfStock(t *testing.T) {
	h := setup()
	// Clean Code only has 1 copy in seed. Borrow it first.
	h.store.Books["Clean Code"].AvailableCopies = 0

	body, _ := json.Marshal(map[string]string{"name": "Anurag", "title": "Clean Code"})
	req := httptest.NewRequest("POST", "/Borrow", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.BorrowBook(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("Expected 409 for out of stock, got %d", rr.Code)
	}
}

// 3. Tests for POST /Extend
func TestExtendLoan_Success(t *testing.T) {
	h := setup()
	// Manually inject a loan to extend
	h.store.Loans = append(h.store.Loans, LoanDetail{
		NameOfBorrower: "Anurag",
		BookTitle:      "Clean Code",
	})

	body, _ := json.Marshal(map[string]string{"name": "Anurag", "title": "Clean Code"})
	req := httptest.NewRequest("POST", "/Extend", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.ExtendLoan(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}
}

func TestExtendLoan_NotFound(t *testing.T) {
	h := setup()
	body, _ := json.Marshal(map[string]string{"name": "Stranger", "title": "Clean Code"})
	req := httptest.NewRequest("POST", "/Extend", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.ExtendLoan(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for no active loan, got %d", rr.Code)
	}
}

// 4. Tests for POST /Return
func TestReturnBook_Success(t *testing.T) {
	h := setup()
	// Borrow first
	h.store.Loans = append(h.store.Loans, LoanDetail{NameOfBorrower: "Anurag", BookTitle: "Clean Code"})

	body, _ := json.Marshal(map[string]string{"name": "Anurag", "title": "Clean Code"})
	req := httptest.NewRequest("POST", "/Return", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	h.ReturnBook(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	if h.store.Books["Clean Code"].AvailableCopies != 2 { // Seeded with 1, +1 returned
		t.Errorf("Expected stock to increase to 2, got %d", h.store.Books["Clean Code"].AvailableCopies)
	}
}
