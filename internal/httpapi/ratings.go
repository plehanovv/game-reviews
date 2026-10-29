package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"game-reviews/internal/account"
	"game-reviews/internal/game"
	"game-reviews/internal/rating"
)

type RatingStore interface {
	Put(context.Context, int64, int64, int) (rating.Rating, error)
	Get(context.Context, int64, int64) (rating.Rating, error)
	Delete(context.Context, int64, int64) error
	Summary(context.Context, int64) (rating.Summary, error)
}

func WithRatings(store RatingStore) Option { return func(o *applicationOptions) { o.ratings = store } }

type ratingHandler struct{ store RatingStore }

func ratingError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, rating.ErrInvalidScore):
		writeError(w, 400, "invalid_score", "Score must be an integer between 1 and 10")
	case errors.Is(err, rating.ErrNotFound):
		writeError(w, 404, "rating_not_found", "You have not rated this game")
	case errors.Is(err, game.ErrNotFound):
		writeError(w, 404, "game_not_found", "Game not found")
	default:
		authError(w, err)
	}
}

func (h ratingHandler) put(w http.ResponseWriter, r *http.Request, user account.User) {
	id, ok := libraryGameID(w, r)
	if !ok {
		return
	}
	var input struct {
		Score int `json:"score"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if !rating.ValidScore(input.Score) {
		ratingError(w, rating.ErrInvalidScore)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	item, err := h.store.Put(ctx, user.ID, id, input.Score)
	if err != nil {
		ratingError(w, err)
		return
	}
	writeJSON(w, 200, item)
}
func (h ratingHandler) get(w http.ResponseWriter, r *http.Request, user account.User) {
	id, ok := libraryGameID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	item, err := h.store.Get(ctx, user.ID, id)
	if err != nil {
		ratingError(w, err)
		return
	}
	writeJSON(w, 200, item)
}
func (h ratingHandler) delete(w http.ResponseWriter, r *http.Request, user account.User) {
	id, ok := libraryGameID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := h.store.Delete(ctx, user.ID, id); err != nil {
		ratingError(w, err)
		return
	}
	w.WriteHeader(204)
}
func (h ratingHandler) summary(w http.ResponseWriter, r *http.Request) {
	id, ok := libraryGameID(w, r)
	if !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	item, err := h.store.Summary(ctx, id)
	if err != nil {
		ratingError(w, err)
		return
	}
	writeJSON(w, 200, item)
}
