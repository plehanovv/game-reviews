package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"game-reviews/internal/library"
)

type libraryStub struct {
	calls int
	owner int64
}

func (s *libraryStub) Put(_ context.Context, owner, id int64, status library.Status) (library.Entry, error) {
	s.calls++
	s.owner = owner
	return library.Entry{GameID: id, Status: status}, nil
}
func (s *libraryStub) List(_ context.Context, owner int64, _ library.ListOptions) ([]library.Entry, error) {
	s.calls++
	s.owner = owner
	return []library.Entry{}, nil
}
func (s *libraryStub) Delete(_ context.Context, owner, id int64) error {
	s.calls++
	s.owner = owner
	return nil
}

func TestLibraryHTTPBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, body string
		cookie                   bool
		origin                   string
		status                   int
		calls                    int
	}{
		{"anonymous list", "GET", "/api/v1/me/library", "", false, "", 401, 0},
		{"anonymous put", "PUT", "/api/v1/me/library/2", `{"status":"planned"}`, false, "", 401, 0},
		{"anonymous delete", "DELETE", "/api/v1/me/library/2", "", false, "", 401, 0},
		{"spoof owner", "PUT", "/api/v1/me/library/2", `{"status":"planned","user_id":2}`, true, "", 400, 0},
		{"invalid status", "PUT", "/api/v1/me/library/2", `{"status":"unknown"}`, true, "", 400, 0},
		{"missing status", "PUT", "/api/v1/me/library/2", `{}`, true, "", 400, 0},
		{"null", "PUT", "/api/v1/me/library/2", `null`, true, "", 400, 0},
		{"bad id", "DELETE", "/api/v1/me/library/0", "", true, "", 400, 0},
		{"bad pagination", "GET", "/api/v1/me/library?limit=101", "", true, "", 400, 0},
		{"cross-origin put", "PUT", "/api/v1/me/library/2", `{"status":"planned"}`, true, "https://attacker.example", 403, 0},
		{"cross-origin delete", "DELETE", "/api/v1/me/library/2", "", true, "https://attacker.example", 403, 0},
		{"put", "PUT", "/api/v1/me/library/2", `{"status":"planned"}`, true, "", 200, 1},
		{"list", "GET", "/api/v1/me/library", "", true, "", 200, 1},
		{"delete", "DELETE", "/api/v1/me/library/2", "", true, "", 204, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &libraryStub{}
			h := NewApplication(nil, nil, &authStub{}, false, WithLibrary(store))
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			if tc.cookie {
				r.AddCookie(&http.Cookie{Name: "game_reviews_session", Value: "test"})
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || store.calls != tc.calls {
				t.Fatalf("status=%d calls=%d: %s", w.Code, store.calls, w.Body.String())
			}
			if store.calls > 0 && store.owner != 1 {
				t.Fatal("owner not taken from session")
			}
			if tc.origin == "" && w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("private response may be cached")
			}
		})
	}
}
