package service

import "errors"

// Domain-level sentinel errors. Handlers depend on these — never on repository errors directly.
var (
	ErrBookNotFound       = errors.New("book not found")
	ErrNoStock            = errors.New("no copies available for loan")
	ErrDuplicateLoan      = errors.New("user already has an active loan for this book")
	ErrLoanNotFound       = errors.New("no active loan found")
	ErrStockRestoreFailed = errors.New("loan deleted but stock could not be restored")
)
