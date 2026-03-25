// Package main is the entry point for the e-Library API server.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"e-library/handlers"
	"e-library/models"
	"e-library/repository"
	"e-library/routes"
	"e-library/service"
)

func main() {
	// Structured JSON logging — suitable for log aggregators (Datadog, CloudWatch, etc.)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Initialize store and seed starting inventory.
	store := repository.NewLibraryStore()
	for _, b := range []models.BookDetail{
		{Title: "The Go Programming Language", AvailableCopies: 3},
		{Title: "Clean Code", AvailableCopies: 1},
	} {
		if err := store.AddBook(b); err != nil {
			logger.Error("failed to seed book", "title", b.Title, "error", err)
		}
	}

	// *service.libraryService satisfies both BookService and LoanService.
	svc := service.New(store, logger)
	h := handlers.NewHandler(svc, svc, logger)
	mux := routes.NewRouter(logger, h)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start the server in a goroutine so we can listen for shutdown signals.
	go func() {
		logger.Info("server starting", "port", port)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Block until SIGINT or SIGTERM is received.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	logger.Info("server stopped")
}
