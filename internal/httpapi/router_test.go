package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouting(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/healthz", 200}, {"POST", "/healthz", 405}, {"GET", "/missing", 404},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			NewHandler().ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
			if w.Code != tc.status {
				t.Fatalf("got %d, want %d", w.Code, tc.status)
			}
			if tc.status == 200 && (w.Body.String() != "{\"status\":\"ok\"}\n" || w.Header().Get("Content-Type") != "application/json; charset=utf-8") {
				t.Fatal("invalid health response")
			}
		})
	}
}

func TestAbout(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/about", nil)
	response := httptest.NewRecorder()

	NewHandler().ServeHTTP(response, request)

	if response.Code != 200 {
		t.Fatalf("got %d, want 200", response.Code)
	}

	if response.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("got %s, want application/json; charset=utf-8", response.Header().Get("Content-Type"))
	}

	var body map[string]string

	err := json.NewDecoder(response.Body).Decode(&body)
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body["name"] != "game-reviews" {
		t.Fatalf("got %s, want %s", body["name"], "game-reviews")
	}

	if body["version"] != "0.1.0" {
		t.Fatalf("got %s, want %s", body["version"], "0.1.0")
	}
}
