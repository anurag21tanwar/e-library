// Package routes wires URL patterns to handlers and applies middleware.
package routes

import (
	"fmt"
	"log/slog"
	"net/http"

	"e-library/middleware"
)

// Registrar is implemented by any handler group that can register its own routes.
// NewRouter accepts a variadic list of Registrars.
type Registrar interface {
	Register(mux *http.ServeMux)
}

// NewRouter builds a ServeMux, registers the health check, delegates route
// registration to each Registrar, and wraps the result with logging middleware.
func NewRouter(logger *slog.Logger, registrars ...Registrar) http.Handler {
	mux := http.NewServeMux()

	// Health check is a router-level concern, not owned by any handler group.
	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "e-Library API is running")
	})

	for _, r := range registrars {
		r.Register(mux)
	}

	return middleware.Logging(logger, mux)
}
