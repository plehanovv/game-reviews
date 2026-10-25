// Package httpapi owns HTTP routing and transport-level responses.
package httpapi

import (
	"context"
	"net/http"

	"game-reviews/internal/game"
)

type Catalog interface {
	List(context.Context, game.ListOptions) ([]game.Game, error)
	Get(context.Context, int64) (game.Game, error)
}

func NewHandler(catalog Catalog) http.Handler {
	return NewHandlerWithReadiness(catalog, nil)
}

func NewHandlerWithReadiness(catalog Catalog, checkReady func(context.Context) error) http.Handler {
	return NewApplication(catalog, checkReady, nil, true)
}

func NewApplication(catalog Catalog, checkReady func(context.Context) error, auth AuthService, secureCookies bool) http.Handler {
	mux := http.NewServeMux()
	// Liveness only: this does not claim external services are ready.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", readinessHandler(checkReady))
	mux.HandleFunc("GET /api/v1/about", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"name": "game-reviews", "version": "0.1.0"})
	})
	games := gameHandler{catalog: catalog}
	mux.HandleFunc("GET /api/v1/games", games.list)
	mux.HandleFunc("GET /api/v1/games/{id}", games.get)
	if auth != nil {
		h := authHandler{service: auth, secure: secureCookies}
		limiter := newAuthLimiter()
		mux.Handle("POST /api/v1/auth/register", noStore(limiter.wrap(h.register)))
		mux.Handle("POST /api/v1/auth/login", noStore(limiter.wrap(h.login)))
		mux.Handle("POST /api/v1/auth/logout", noStore(http.HandlerFunc(h.logout)))
		mux.Handle("GET /api/v1/auth/me", noStore(http.HandlerFunc(h.me)))
	}
	protection := http.NewCrossOriginProtection()
	protection.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, 403, "cross_origin_denied", "Cross-origin requests are not allowed")
	}))
	return protection.Handler(mux)
}
