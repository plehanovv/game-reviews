// Package httpapi owns HTTP routing and transport-level responses.
package httpapi

import (
	"encoding/json"
	"net/http"
)

func NewHandler() http.Handler {
	mux := http.NewServeMux()
	// Liveness only: this does not claim PostgreSQL, Redis or Kafka are ready.

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte("{\"status\":\"ok\"}\n"))
	})

	mux.HandleFunc("GET /api/v1/about", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		response := map[string]string{
			"name":    "game-reviews",
			"version": "0.1.0",
		}

		err := json.NewEncoder(w).Encode(response)
		if err != nil {
			return
		}
	})
	return mux
}
