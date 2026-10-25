package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"game-reviews/internal/game"
)

func TestListGames(t *testing.T) {
	for _, items := range [][]game.Game{nil, {{ID: 7, Title: "Example", ReleaseYear: 2020}}} {
		w := httptest.NewRecorder()
		NewHandler(game.NewCatalog(items)).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/games", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
		if w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
			t.Fatal("missing JSON content type")
		}
		var page gamesPage
		if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		got := page.Items
		if page.NextAfterID != nil {
			t.Fatal("unexpected next page")
		}
		if got == nil {
			t.Fatal("empty catalog must return [], not null")
		}
		if len(got) != len(items) {
			t.Fatalf("got %d games, want %d", len(got), len(items))
		}
		for i := range items {
			if got[i] != items[i] {
				t.Fatalf("got %+v, want %+v", got[i], items[i])
			}
		}
	}
}

func TestGetGame(t *testing.T) {
	item := game.Game{ID: 7, Title: "Example", ReleaseYear: 2020}
	handler := NewHandler(game.NewCatalog([]game.Game{item}))
	for _, tc := range []struct {
		id     string
		status int
		code   string
	}{
		{"7", 200, ""}, {"8", 404, "game_not_found"},
		{"abc", 400, "invalid_game_id"}, {"0", 400, "invalid_game_id"},
		{"-1", 400, "invalid_game_id"}, {"9223372036854775808", 400, "invalid_game_id"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/games/"+tc.id, nil))
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d", w.Code, tc.status)
			}
			if w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
				t.Fatal("missing JSON content type")
			}
			if tc.status == 200 {
				var got game.Game
				if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if got != item {
					t.Fatalf("got %+v, want %+v", got, item)
				}
			} else {
				var got errorResponse
				if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if got.Error.Code != tc.code || got.Error.Message == "" {
					t.Fatalf("unexpected error: %+v", got)
				}
			}
		})
	}
}

func TestWriteJSONEncodingFailure(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, make(chan int))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", w.Code)
	}
	var got errorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Error.Code != "internal_error" {
		t.Fatalf("unexpected response: %+v", got)
	}
}
