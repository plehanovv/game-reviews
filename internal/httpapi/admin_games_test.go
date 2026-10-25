package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"game-reviews/internal/account"
	"game-reviews/internal/game"
)

type creatorStub struct {
	calls int
	input game.CreateInput
}

func (s *creatorStub) Create(ctx context.Context, input game.CreateInput) (game.Game, error) {
	s.calls++
	s.input = input
	if _, ok := ctx.Deadline(); !ok {
		return game.Game{}, errors.New("missing deadline")
	}
	return game.Game{ID: 42, Title: input.Title, ReleaseYear: input.ReleaseYear}, nil
}

func TestCreateGameAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name, role string
		cookie     bool
		err        error
		status     int
	}{
		{"anonymous", "admin", false, nil, 401},
		{"member", "user", true, nil, 403},
		{"expired", "", true, account.ErrUnauthenticated, 401},
		{"database failed", "", true, errors.New("database failed"), 500},
		{"administrator", "admin", true, nil, 201},
	} {
		t.Run(tc.name, func(t *testing.T) {
			creator := &creatorStub{}
			h := NewApplication(nil, nil, &authStub{role: tc.role, currentError: tc.err}, false, WithGameCreation(creator))
			r := httptest.NewRequest("POST", "/api/v1/games", strings.NewReader(`{"title":" Portal 2 ","release_year":2011}`))
			r.Header.Set("Content-Type", "application/json")
			if tc.cookie {
				r.AddCookie(&http.Cookie{Name: "game_reviews_session", Value: "test"})
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("got %d want %d: %s", w.Code, tc.status, w.Body.String())
			}
			if tc.status == 201 {
				if creator.calls != 1 || creator.input.Title != "Portal 2" || w.Header().Get("Location") != "/api/v1/games/42" {
					t.Fatal("incorrect creation")
				}
			} else if creator.calls != 0 {
				t.Fatal("unauthorized write reached database")
			}
		})
	}
}

func TestCreateGameRejectsInvalidBody(t *testing.T) {
	for _, body := range []string{`null`, `{}`, `{"title":" ","release_year":2011}`, `{"title":"Test","release_year":0}`, `{"title":"Test","release_year":2011,"id":1}`, `{"title":"Test","release_year":"2011"}`} {
		creator := &creatorStub{}
		h := NewApplication(nil, nil, &authStub{role: "admin"}, false, WithGameCreation(creator))
		r := httptest.NewRequest("POST", "/api/v1/games", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(&http.Cookie{Name: "game_reviews_session", Value: "test"})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 400 || creator.calls != 0 {
			t.Fatalf("invalid input accepted: %s", body)
		}
	}
}
