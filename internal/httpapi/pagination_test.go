package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"game-reviews/internal/game"
)

func TestPagination(t *testing.T) {
	h := NewHandler(game.NewCatalog([]game.Game{{ID: 9}, {ID: 2}, {ID: 5}}))
	for _, tc := range []struct {
		query string
		ids   []int64
		next  int64
	}{
		{"?limit=2", []int64{2, 5}, 5},
		{"?limit=2&after_id=5", []int64{9}, 0},
		{"?limit=2&after_id=9", []int64{}, 0},
		{"?limit=1&after_id=3", []int64{5}, 5},
		{"?limit=3", []int64{2, 5, 9}, 0},
	} {
		t.Run(tc.query, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/games"+tc.query, nil))
			if w.Code != 200 {
				t.Fatalf("status = %d", w.Code)
			}
			var got gamesPage
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if len(got.Items) != len(tc.ids) {
				t.Fatalf("items = %+v", got.Items)
			}
			for i, id := range tc.ids {
				if got.Items[i].ID != id {
					t.Fatalf("items = %+v", got.Items)
				}
			}
			if tc.next == 0 && got.NextAfterID != nil {
				t.Fatal("unexpected next cursor")
			}
			if tc.next != 0 && (got.NextAfterID == nil || *got.NextAfterID != tc.next) {
				t.Fatal("incorrect next cursor")
			}
		})
	}
}

func TestInvalidPagination(t *testing.T) {
	for _, query := range []string{"limit=0", "limit=-1", "limit=101", "limit=abc", "limit=", "limit=1&limit=2", "after_id=-1", "after_id=", "after_id=9223372036854775808", "after_id=1&after_id=2", "offset=2", "limit=%zz", "limit=1;after_id=2"} {
		t.Run(query, func(t *testing.T) {
			w := httptest.NewRecorder()
			// A nil catalog proves validation happens before database access.
			NewHandler(nil).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/games?"+query, nil))
			if w.Code != 400 {
				t.Fatalf("status = %d", w.Code)
			}
		})
	}
}

func TestPageSizeBounds(t *testing.T) {
	items := make([]game.Game, 105)
	for i := range items {
		items[i].ID = int64(i + 1)
	}
	h := NewHandler(game.NewCatalog(items))
	for _, tc := range []struct {
		query string
		count int
	}{{"", 20}, {"?limit=100", 100}} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/games"+tc.query, nil))
		var page gamesPage
		if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || len(page.Items) != tc.count || page.NextAfterID == nil || *page.NextAfterID != int64(tc.count) {
			t.Fatalf("unexpected bounded page: %+v", page)
		}
	}
}
