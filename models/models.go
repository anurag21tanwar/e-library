// Package models define the core data structures used across the e-Library API.
package models

import "time"

// BookDetail represents a book in the library's inventory.
type BookDetail struct {
	Title           string `json:"title"`
	AvailableCopies int    `json:"available_copies"`
}

// LoanDetail represents an active loan record for a borrowed book.
type LoanDetail struct {
	NameOfBorrower string    `json:"name_of_borrower"`
	BookTitle      string    `json:"book_title"`
	LoanDate       time.Time `json:"loan_date"`
	ReturnDate     time.Time `json:"return_date"`
}
