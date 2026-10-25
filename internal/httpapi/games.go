package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"game-reviews/internal/game"
)

type gameHandler struct{ catalog Catalog }

type gamesPage struct {
	Items       []game.Game `json:"items"`
	NextAfterID *int64      `json:"next_after_id"`
}

func parseListOptions(raw string) (game.ListOptions, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return game.ListOptions{}, err
	}
	options := game.ListOptions{Limit: 20}
	for key, entries := range values {
		if len(entries) != 1 {
			return options, errors.New("duplicate parameter")
		}
		switch key {
		case "limit":
			options.Limit, err = strconv.Atoi(entries[0])
			if err != nil || options.Limit < 1 || options.Limit > game.MaxPageSize {
				return options, errors.New("invalid limit")
			}
		case "after_id":
			options.AfterID, err = strconv.ParseInt(entries[0], 10, 64)
			if err != nil || options.AfterID < 0 {
				return options, errors.New("invalid cursor")
			}
		default:
			return options, errors.New("unknown parameter")
		}
	}
	return options, nil
}

func (h gameHandler) list(w http.ResponseWriter, r *http.Request) {
	options, err := parseListOptions(r.URL.RawQuery)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_pagination", "Use limit between 1 and 100 and a non-negative after_id; parameters must occur once")
		return
	}
	pageSize := options.Limit
	options.Limit++ // One extra row tells us whether another page exists.
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	items, err := h.catalog.List(ctx, options)
	if err != nil {
		slog.Error("list games failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	page := gamesPage{Items: items}
	if len(items) > pageSize {
		page.Items = items[:pageSize]
		id := page.Items[len(page.Items)-1].ID
		page.NextAfterID = &id
	}
	writeJSON(w, http.StatusOK, page)
}

func (h gameHandler) get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_game_id", "Game ID must be a positive integer")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	item, err := h.catalog.Get(ctx, id)
	if errors.Is(err, game.ErrNotFound) {
		writeError(w, http.StatusNotFound, "game_not_found", "Game not found")
		return
	}
	if err != nil {
		slog.Error("get game failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
		return
	}
	writeJSON(w, http.StatusOK, item)
}
