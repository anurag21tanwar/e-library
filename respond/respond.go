// Package respond provides shared HTTP response helpers for the e-Library API.
package respond

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ErrorBody is the standard JSON error envelope returned by all error responses.
type ErrorBody struct {
	Error string `json:"error"`
}

// Error writes a JSON-encoded ErrorBody with the given HTTP status code.
func Error(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(ErrorBody{Error: message})
}

// JSON marshals v into a buffer before writing to w.
// If
// marshaling fails, it logs the error and writes a 500 response instead.
func JSON(w http.ResponseWriter, logger *slog.Logger, code int, v any) {
	buf, err := json.Marshal(v)
	if err != nil {
		logger.Error("failed to marshal response", "error", err)
		Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(buf)
}
