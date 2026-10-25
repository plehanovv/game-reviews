package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"game-reviews/internal/account"
)

type authStub struct {
	calls        int
	role         string
	currentError error
}

func (s *authStub) Register(context.Context, string, string) (account.User, error) {
	s.calls++
	return account.User{ID: 1, Role: "user"}, nil
}
func (s *authStub) Login(context.Context, string, string) (account.User, string, error) {
	s.calls++
	return account.User{ID: 1}, "token", nil
}
func (s *authStub) CurrentUser(context.Context, string) (account.User, error) {
	return account.User{ID: 1, Role: s.role}, s.currentError
}
func (s *authStub) Logout(context.Context, string) error { return nil }

func TestAuthInputBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, body, contentType string
		status                  int
	}{
		{"type", `{}`, "text/plain", 415},
		{"unknown role", `{"username":"test","password":"password","role":"admin"}`, "application/json", 400},
		{"trailing", `{} {}`, "application/json", 400},
		{"invalid", `{`, "application/json", 400},
		{"too large", `{"password":"` + strings.Repeat("x", 5000) + `"}`, "application/json", 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &authStub{}
			h := NewApplication(nil, nil, stub, true)
			r := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", tc.contentType)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status || stub.calls != 0 {
				t.Fatalf("status %d, service calls %d", w.Code, stub.calls)
			}
		})
	}
}

func TestCrossOriginAuthDenied(t *testing.T) {
	for _, path := range []string{"register", "login", "logout"} {
		stub := &authStub{}
		r := httptest.NewRequest("POST", "https://game.example/api/v1/auth/"+path, strings.NewReader(`{}`))
		r.Header.Set("Origin", "https://attacker.example")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		NewApplication(nil, nil, stub, true).ServeHTTP(w, r)
		if w.Code != 403 || stub.calls != 0 {
			t.Fatalf("cross-origin %s: %d", path, w.Code)
		}
	}
}

func TestSecureSessionCookie(t *testing.T) {
	r := httptest.NewRequest("POST", "https://game.example/api/v1/auth/login", strings.NewReader(`{"username":"test","password":"long enough password"}`))
	r.Header.Set("Origin", "https://game.example")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	NewApplication(nil, nil, &authStub{}, true).ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("status %d", w.Code)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("expected session cookie")
	}
	c := cookies[0]
	if c.Name != "__Host-game_reviews_session" || !c.HttpOnly || !c.Secure || c.Path != "/" || c.Domain != "" || c.SameSite != http.SameSiteStrictMode || c.MaxAge != 86400 {
		t.Fatalf("invalid cookie flags: %+v", c)
	}
	if w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "token") {
		t.Fatal("response exposed session token or can be cached")
	}
}

func TestAuthRateLimit(t *testing.T) {
	now := time.Now()
	l := newAuthLimiter()
	l.now = func() time.Time { return now }
	for i := 0; i < 10; i++ {
		if !l.allow("client") {
			t.Fatal("unexpected limit")
		}
	}
	if l.allow("client") {
		t.Fatal("limit not enforced")
	}
	if !l.allow("other") {
		t.Fatal("different IP blocked")
	}
	now = now.Add(time.Minute)
	if !l.allow("client") {
		t.Fatal("expired window not reset")
	}
	stub := &authStub{}
	h := NewApplication(nil, nil, stub, true)
	for i := 0; i < 11; i++ {
		r := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Forwarded-For", string(rune('a'+i)))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if i == 10 && (w.Code != 429 || w.Header().Get("Retry-After") == "") {
			t.Fatal("forwarded headers bypass rate limiting")
		}
	}
}
