package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"game-reviews/internal/account"
	"game-reviews/internal/game"
	"game-reviews/internal/library"
)

type LibraryStore interface {
	Put(context.Context, int64, int64, library.Status) (library.Entry, error)
	List(context.Context, int64, library.ListOptions) ([]library.Entry, error)
	Delete(context.Context, int64, int64) error
}

func WithLibrary(store LibraryStore) Option { return func(o *applicationOptions) { o.library = store } }

type libraryHandler struct{ store LibraryStore }
type libraryPage struct {
	Items       []library.Entry `json:"items"`
	NextAfterID *int64          `json:"next_after_id"`
}

func libraryGameID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, 400, "invalid_game_id", "Game ID must be a positive integer")
		return 0, false
	}
	return id, true
}

func (h libraryHandler) put(w http.ResponseWriter, r *http.Request, user account.User) {
	id, ok := libraryGameID(w, r)
	if !ok {
		return
	}
	var input struct {
		Status library.Status `json:"status"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if !input.Status.Valid() {
		writeError(w, 400, "invalid_library_status", "Use planned, playing, completed, on_hold or dropped")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	entry, err := h.store.Put(ctx, user.ID, id, input.Status)
	if errors.Is(err, game.ErrNotFound) {
		writeError(w, 404, "game_not_found", "Game not found")
		return
	}
	if err != nil {
		authError(w, err)
		return
	}
	writeJSON(w, 200, entry)
}

func (h libraryHandler) list(w http.ResponseWriter, r *http.Request, user account.User) {
	options, err := parseListOptions(r.URL.RawQuery)
	if err != nil {
		writeError(w, 400, "invalid_pagination", "Use limit between 1 and 100 and a non-negative after_id")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	items, err := h.store.List(ctx, user.ID, library.ListOptions{AfterGameID: options.AfterID, Limit: options.Limit + 1})
	if err != nil {
		authError(w, err)
		return
	}
	page := libraryPage{Items: items}
	if len(items) > options.Limit {
		page.Items = items[:options.Limit]
		id := page.Items[len(page.Items)-1].GameID
		page.NextAfterID = &id
	}
	writeJSON(w, 200, page)
}

func (h libraryHandler) delete(w http.ResponseWriter, r *http.Request, user account.User) {
	id, ok := libraryGameID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if err := h.store.Delete(ctx, user.ID, id); err != nil {
		authError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
