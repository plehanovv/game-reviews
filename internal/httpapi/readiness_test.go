package httpapi

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadiness(t *testing.T) {
	for _, tc := range []struct {
		name   string
		check  func(context.Context) error
		status int
	}{
		{"unconfigured", nil, 503},
		{"healthy", func(ctx context.Context) error {
			if _, ok := ctx.Deadline(); !ok {
				t.Error("missing deadline")
			}
			return nil
		}, 200},
		{"failed", func(context.Context) error { return errors.New("private connection details") }, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHandlerWithReadiness(nil, tc.check)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", "/readyz", nil))
			if w.Code != tc.status {
				t.Fatalf("status = %d", w.Code)
			}
			if w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("readiness may be cached")
			}
			if strings.Contains(w.Body.String(), "private") {
				t.Fatal("internal details leaked")
			}
			w = httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
			if w.Code != 200 {
				t.Fatal("liveness must not depend on database")
			}
		})
	}
}
