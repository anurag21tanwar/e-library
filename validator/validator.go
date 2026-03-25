// Package validator defines typed request structs for each endpoint.
// Each struct owns its own Validate() method, keeping validation rules
// co-located with the data they describe (SRP) and out of handler logic.
package validator

import "errors"

// validateNameAndTitle is the shared rule for all loan-related requests.
func validateNameAndTitle(name, title string) error {
	if name == "" || title == "" {
		return errors.New("Name and Title are required")
	}
	return nil
}

// GetBookQuery holds the parsed query parameters for GET /Book.
type GetBookQuery struct {
	Title string
}

func (q GetBookQuery) Validate() error {
	if q.Title == "" {
		return errors.New("Title query parameter is required")
	}
	return nil
}

// BorrowRequest is the decoded and validated body for POST /Borrow.
type BorrowRequest struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

func (r BorrowRequest) Validate() error {
	return validateNameAndTitle(r.Name, r.Title)
}

// ExtendRequest is the decoded and validated body for POST /Extend.
type ExtendRequest struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

func (r ExtendRequest) Validate() error {
	return validateNameAndTitle(r.Name, r.Title)
}

// ReturnRequest is the decoded and validated body for POST /Return.
type ReturnRequest struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

func (r ReturnRequest) Validate() error {
	return validateNameAndTitle(r.Name, r.Title)
}
