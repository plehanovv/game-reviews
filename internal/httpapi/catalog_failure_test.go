package httpapi

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"game-reviews/internal/game"
)

type failingCatalog struct{ t *testing.T }

func (c failingCatalog) List(ctx context.Context, options game.ListOptions) ([]game.Game, error) {
	if _, ok := ctx.Deadline(); !ok {
		c.t.Error("database operation has no deadline")
	}
	return nil, errors.New("private database details")
}

func (c failingCatalog) Get(ctx context.Context, id int64) (game.Game, error) {
	_, err := c.List(ctx, game.ListOptions{Limit: 20})
	return game.Game{}, err
}

func TestCatalogFailure(t *testing.T) {
	for _, path := range []string{"/api/v1/games", "/api/v1/games/1"} {
		w := httptest.NewRecorder()
		NewHandler(failingCatalog{t}).ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 500 {
			t.Fatalf("status = %d, want 500", w.Code)
		}
		if strings.Contains(w.Body.String(), "private") {
			t.Fatal("internal error leaked to client")
		}
		if !strings.Contains(w.Body.String(), "internal_error") {
			t.Fatal("missing public error code")
		}
	}
}
