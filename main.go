package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	// Initialize a new serve mux (router)
	mux := http.NewServeMux()

	// Root health check to verify the server is up
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "e-Library API is running")
	})

	port := ":3000"
	fmt.Printf("Server starting on port %s...\n", port)

	// Start the server
	err := http.ListenAndServe(port, mux)
	if err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
