package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"time"

	"game-reviews/internal/account"
)

type AuthService interface {
	Register(context.Context, string, string) (account.User, error)
	Login(context.Context, string, string) (account.User, string, error)
	CurrentUser(context.Context, string) (account.User, error)
	Logout(context.Context, string) error
}

type authHandler struct {
	service AuthService
	secure  bool
}

func (h authHandler) cookieName() string {
	if h.secure {
		return "__Host-game_reviews_session"
	}
	return "game_reviews_session"
}

func (h authHandler) cookie(token string, maxAge int) *http.Cookie {
	return &http.Cookie{Name: h.cookieName(), Value: token, Path: "/", HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteStrictMode, MaxAge: maxAge}
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func decodeCredentials(w http.ResponseWriter, r *http.Request) (credentials, bool) {
	var input credentials
	ok := decodeJSON(w, r, &input)
	return input, ok
}

func decodeJSON(w http.ResponseWriter, r *http.Request, input any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(input)
	if err == nil {
		var extra any
		if trailing := decoder.Decode(&extra); trailing != io.EOF {
			if trailing == nil {
				err = errors.New("extra JSON value")
			} else {
				err = trailing
			}
		}
	}
	if err != nil {
		var sizeErr *http.MaxBytesError
		if errors.As(err, &sizeErr) {
			writeError(w, 413, "body_too_large", "Request body exceeds 4096 bytes")
		} else {
			writeError(w, 400, "invalid_json", "Expected one JSON object with supported fields")
		}
		return false
	}
	return true
}

func authError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, account.ErrInvalidInput):
		writeError(w, 400, "invalid_credentials_format", "Username: 3-32 letters a-z, digits or underscores; password: 15-128 characters")
	case errors.Is(err, account.ErrUsernameTaken):
		writeError(w, 409, "username_unavailable", "Username is unavailable")
	case errors.Is(err, account.ErrUnauthenticated):
		writeError(w, 401, "unauthenticated", "Invalid credentials or session")
	case errors.Is(err, account.ErrBusy):
		w.Header().Set("Retry-After", "1")
		writeError(w, 503, "auth_busy", "Try again later")
	default:
		slog.Error("account operation failed", "error", err)
		writeError(w, 500, "internal_error", "Internal server error")
	}
}

func (h authHandler) register(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeCredentials(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	user, err := h.service.Register(ctx, input.Username, input.Password)
	if err != nil {
		authError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (h authHandler) login(w http.ResponseWriter, r *http.Request) {
	input, ok := decodeCredentials(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	user, token, err := h.service.Login(ctx, input.Username, input.Password)
	if err != nil {
		authError(w, err)
		return
	}
	// Revoke the browser's previous session when replacing its cookie.
	if old, err := r.Cookie(h.cookieName()); err == nil {
		if err := h.service.Logout(ctx, old.Value); err != nil {
			_ = h.service.Logout(ctx, token)
			authError(w, err)
			return
		}
	}
	http.SetCookie(w, h.cookie(token, int(account.SessionLifetime.Seconds())))
	writeJSON(w, 200, user)
}

func (h authHandler) me(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(h.cookieName())
	if err != nil {
		authError(w, account.ErrUnauthenticated)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	user, err := h.service.CurrentUser(ctx, cookie.Value)
	if err != nil {
		authError(w, err)
		return
	}
	writeJSON(w, 200, user)
}

func (h authHandler) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(h.cookieName()); err == nil {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := h.service.Logout(ctx, cookie.Value); err != nil {
			authError(w, err)
			return
		}
	}
	http.SetCookie(w, h.cookie("", -1))
	w.WriteHeader(http.StatusNoContent)
}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
