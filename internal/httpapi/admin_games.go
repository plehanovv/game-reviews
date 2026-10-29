package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"game-reviews/internal/account"
	"game-reviews/internal/game"
)

type GameCreator interface {
	Create(context.Context, game.CreateInput) (game.Game, error)
}
type applicationOptions struct {
	creator GameCreator
	library LibraryStore
	ratings RatingStore
}
type Option func(*applicationOptions)

func WithGameCreation(creator GameCreator) Option {
	return func(options *applicationOptions) { options.creator = creator }
}

func (h authHandler) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return h.requireUser(func(w http.ResponseWriter, r *http.Request, user account.User) {
		if user.Role != "admin" {
			writeError(w, 403, "forbidden", "Administrator access required")
			return
		}
		next(w, r)
	})
}

func (h authHandler) requireUser(next func(http.ResponseWriter, *http.Request, account.User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(h.cookieName())
		if err != nil {
			authError(w, account.ErrUnauthenticated)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		user, err := h.service.CurrentUser(ctx, cookie.Value)
		cancel()
		if err != nil {
			authError(w, err)
			return
		}
		next(w, r, user)
	}
}

func createGame(creator GameCreator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input game.CreateInput
		if !decodeJSON(w, r, &input) {
			return
		}
		input, err := input.Validate()
		if err != nil {
			writeError(w, 400, "invalid_game", "Title must contain 1-300 characters and release_year must be between 1950 and 2100")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		item, err := creator.Create(ctx, input)
		if errors.Is(err, game.ErrInvalidGame) {
			writeError(w, 400, "invalid_game", "Invalid game data")
			return
		}
		if err != nil {
			authError(w, err)
			return
		}
		w.Header().Set("Location", "/api/v1/games/"+strconv.FormatInt(item.ID, 10))
		writeJSON(w, http.StatusCreated, item)
	}
}
