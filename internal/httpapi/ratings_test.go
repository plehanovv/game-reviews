package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"game-reviews/internal/rating"
)

type ratingsStub struct {
	calls int
	owner int64
}

func (s *ratingsStub) Put(_ context.Context, owner, id int64, score int) (rating.Rating, error) {
	s.calls++
	s.owner = owner
	return rating.Rating{GameID: id, Score: score}, nil
}
func (s *ratingsStub) Get(_ context.Context, owner, id int64) (rating.Rating, error) {
	s.calls++
	s.owner = owner
	return rating.Rating{GameID: id, Score: 8}, nil
}
func (s *ratingsStub) Delete(_ context.Context, owner, id int64) error {
	s.calls++
	s.owner = owner
	return nil
}
func (s *ratingsStub) Summary(_ context.Context, id int64) (rating.Summary, error) {
	s.calls++
	return rating.Summary{GameID: id}, nil
}

func TestRatingBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, body string
		cookie                   bool
		origin                   string
		status, calls            int
	}{
		{"anonymous put", "PUT", "/api/v1/me/ratings/1", `{"score":8}`, false, "", 401, 0},
		{"anonymous get", "GET", "/api/v1/me/ratings/1", "", false, "", 401, 0},
		{"anonymous delete", "DELETE", "/api/v1/me/ratings/1", "", false, "", 401, 0},
		{"public summary", "GET", "/api/v1/games/1/rating", "", false, "", 200, 1},
		{"minimum", "PUT", "/api/v1/me/ratings/1", `{"score":1}`, true, "", 200, 1},
		{"maximum", "PUT", "/api/v1/me/ratings/1", `{"score":10}`, true, "", 200, 1},
		{"zero", "PUT", "/api/v1/me/ratings/1", `{"score":0}`, true, "", 400, 0},
		{"too high", "PUT", "/api/v1/me/ratings/1", `{"score":11}`, true, "", 400, 0},
		{"fraction", "PUT", "/api/v1/me/ratings/1", `{"score":7.5}`, true, "", 400, 0},
		{"string", "PUT", "/api/v1/me/ratings/1", `{"score":"8"}`, true, "", 400, 0},
		{"null", "PUT", "/api/v1/me/ratings/1", `{"score":null}`, true, "", 400, 0},
		{"spoof owner", "PUT", "/api/v1/me/ratings/1", `{"score":8,"user_id":2}`, true, "", 400, 0},
		{"invalid id", "GET", "/api/v1/games/abc/rating", "", false, "", 400, 0},
		{"cross origin", "PUT", "/api/v1/me/ratings/1", `{"score":8}`, true, "https://attacker.example", 403, 0},
		{"cross origin delete", "DELETE", "/api/v1/me/ratings/1", "", true, "https://attacker.example", 403, 0},
		{"owner get", "GET", "/api/v1/me/ratings/1", "", true, "", 200, 1},
		{"owner delete", "DELETE", "/api/v1/me/ratings/1", "", true, "", 204, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &ratingsStub{}
			h := NewApplication(nil, nil, &authStub{}, false, WithRatings(store))
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			if tc.cookie {
				r.AddCookie(&http.Cookie{Name: "game_reviews_session", Value: "test"})
			}
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || store.calls != tc.calls {
				t.Fatalf("status %d calls %d: %s", w.Code, store.calls, w.Body.String())
			}
			if tc.cookie && store.calls > 0 && store.owner != 1 {
				t.Fatal("owner not taken from session")
			}
		})
	}
}
