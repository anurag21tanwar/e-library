// Package validator defines typed request structs for each endpoint.
package validator

import "errors"

// validateNameAndTitle is the shared rule for all loan-related requests.
func validateNameAndTitle(name, title string) error {
	if name == "" || title == "" {
		return errors.New("name and title are required")
	}
	return nil
}

// GetBookQuery holds the parsed query parameters for GET /Book.
type GetBookQuery struct {
	Title string
}

// Validate returns an error if the Title field is empty.
func (q GetBookQuery) Validate() error {
	if q.Title == "" {
		return errors.New("title query parameter is required")
	}
	return nil
}

// BorrowRequest is the decoded and validated body for POST /Borrow.
type BorrowRequest struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

// Validate returns an error if Name or Title is empty.
func (r BorrowRequest) Validate() error {
	return validateNameAndTitle(r.Name, r.Title)
}

// ExtendRequest is the decoded and validated body for POST /Extend.
type ExtendRequest struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

// Validate returns an error if Name or Title is empty.
func (r ExtendRequest) Validate() error {
	return validateNameAndTitle(r.Name, r.Title)
}

// ReturnRequest is the decoded and validated body for POST /Return.
type ReturnRequest struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

// Validate returns an error if Name or Title is empty.
func (r ReturnRequest) Validate() error {
	return validateNameAndTitle(r.Name, r.Title)
}
