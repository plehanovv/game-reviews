package account

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidInput    = errors.New("invalid account input")
	ErrUsernameTaken   = errors.New("username unavailable")
	ErrUnauthenticated = errors.New("invalid credentials or session")
	ErrBusy            = errors.New("authentication capacity exceeded")
	usernamePattern    = regexp.MustCompile(`^[a-z0-9_]{3,32}$`)
)

const SessionLifetime = 24 * time.Hour

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type Store interface {
	CreateUser(context.Context, string, string) (User, error)
	FindUser(context.Context, string) (User, string, error)
	CreateSession(context.Context, []byte, int64, time.Time) error
	SessionUser(context.Context, []byte) (User, error)
	DeleteSession(context.Context, []byte) error
}

type Service struct {
	store     Store
	dummyHash string
	workers   chan struct{}
}

func NewService(store Store) *Service {
	return &Service{store: store, dummyHash: HashPassword("dummy password never accepted"), workers: make(chan struct{}, 4)}
}

func normalize(username string) string { return strings.ToLower(strings.TrimSpace(username)) }

func validPassword(password string) bool {
	n := utf8.RuneCountInString(password)
	return utf8.ValidString(password) && n >= 15 && n <= 128
}

func (s *Service) acquire() bool {
	select {
	case s.workers <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *Service) Register(ctx context.Context, username, password string) (User, error) {
	if err := ctx.Err(); err != nil {
		return User{}, err
	}
	username = normalize(username)
	if !usernamePattern.MatchString(username) || !validPassword(password) {
		return User{}, ErrInvalidInput
	}
	if !s.acquire() {
		return User{}, ErrBusy
	}
	defer func() { <-s.workers }()
	return s.store.CreateUser(ctx, username, HashPassword(password))
}

func (s *Service) Login(ctx context.Context, username, password string) (User, string, error) {
	if err := ctx.Err(); err != nil {
		return User{}, "", err
	}
	username = normalize(username)
	if !usernamePattern.MatchString(username) || !validPassword(password) {
		return User{}, "", ErrUnauthenticated
	}
	if !s.acquire() {
		return User{}, "", ErrBusy
	}
	defer func() { <-s.workers }()
	user, encoded, err := s.store.FindUser(ctx, username)
	if err != nil && !errors.Is(err, ErrUnauthenticated) {
		return User{}, "", err
	}
	if errors.Is(err, ErrUnauthenticated) {
		encoded = s.dummyHash
	}
	valid := VerifyPassword(encoded, password)
	if !valid || err != nil {
		return User{}, "", ErrUnauthenticated
	}
	bytes := make([]byte, 32)
	_, _ = rand.Read(bytes)
	token := base64.RawURLEncoding.EncodeToString(bytes)
	hash := sha256.Sum256([]byte(token))
	if err := s.store.CreateSession(ctx, hash[:], user.ID, time.Now().Add(SessionLifetime)); err != nil {
		return User{}, "", err
	}
	return user, token, nil
}

func tokenHash(token string) ([]byte, error) {
	if len(token) != 43 {
		return nil, ErrUnauthenticated
	}
	bytes, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(bytes) != 32 || base64.RawURLEncoding.EncodeToString(bytes) != token {
		return nil, ErrUnauthenticated
	}
	hash := sha256.Sum256([]byte(token))
	return hash[:], nil
}

func (s *Service) CurrentUser(ctx context.Context, token string) (User, error) {
	hash, err := tokenHash(token)
	if err != nil {
		return User{}, err
	}
	return s.store.SessionUser(ctx, hash)
}

func (s *Service) Logout(ctx context.Context, token string) error {
	hash, err := tokenHash(token)
	if err != nil {
		return nil
	}
	return s.store.DeleteSession(ctx, hash)
}
