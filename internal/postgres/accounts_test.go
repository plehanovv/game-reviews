package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"game-reviews/internal/account"
	"game-reviews/internal/game"
	"game-reviews/internal/httpapi"
	"game-reviews/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func TestAccountLifecycleIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("account_test_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if _, err := admin.ExecContext(cleanup, `DROP SCHEMA `+schema+` CASCADE`); err != nil {
			t.Error(err)
		}
	}()
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		t.Fatal("invalid test database configuration")
	}
	cfg.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*cfg)
	defer db.Close()
	store := NewAccounts(db)
	if store.CheckReady(ctx) == nil {
		t.Fatal("unmigrated accounts reported ready")
	}
	if err := migrations.Up(ctx, db); err != nil {
		t.Fatal(err)
	}
	if err := store.CheckReady(ctx); err != nil {
		t.Fatal(err)
	}
	service := account.NewService(store)
	h := httpapi.NewApplication(NewCatalog(db), store.CheckReady, service, false, httpapi.WithGameCreation(NewCatalog(db)))
	request := func(method, path, body string, cookie *http.Cookie, want int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: got %d, want %d; %s", method, path, w.Code, want, w.Body.String())
		}
		return w
	}
	body := `{"username":" Player_1 ","password":"a very long test password"}`
	registered := request("POST", "/api/v1/auth/register", body, nil, 201)
	var user account.User
	if err := json.Unmarshal(registered.Body.Bytes(), &user); err != nil {
		t.Fatal(err)
	}
	if user.Role != "user" || user.Username != "player_1" {
		t.Fatalf("unexpected user: %+v", user)
	}
	request("POST", "/api/v1/auth/register", body, nil, 409)
	request("POST", "/api/v1/auth/register", `{"username":"bad","password":"short"}`, nil, 400)
	request("POST", "/api/v1/auth/login", `{"username":"player_1","password":"the wrong long password"}`, nil, 401)
	request("POST", "/api/v1/auth/login", `{"username":"missing","password":"the wrong long password"}`, nil, 401)
	_, hash, err := store.FindUser(ctx, "player_1")
	if err != nil || !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatal("password not stored as Argon2id")
	}
	login := request("POST", "/api/v1/auth/login", body, nil, 200)
	cookie := login.Result().Cookies()[0]
	if strings.Contains(login.Body.String(), cookie.Value) || strings.Contains(registered.Body.String(), "password") {
		t.Fatal("credentials leaked")
	}
	tokenHash := sha256.Sum256([]byte(cookie.Value))
	var storedHash []byte
	if err := db.QueryRowContext(ctx, `SELECT token_hash FROM sessions WHERE user_id=$1`, user.ID).Scan(&storedHash); err != nil {
		t.Fatal(err)
	}
	if string(storedHash) != string(tokenHash[:]) {
		t.Fatal("session token not hashed")
	}
	request("GET", "/api/v1/auth/me", "", cookie, 200)
	gameBody := `{"title":" Portal 2 ","release_year":2011}`
	request("POST", "/api/v1/games", gameBody, nil, 401)
	request("POST", "/api/v1/games", gameBody, cookie, 403)
	if err := store.SetRole(ctx, "player_1", "admin"); err != nil {
		t.Fatal(err)
	}
	created := request("POST", "/api/v1/games", gameBody, cookie, 201)
	var createdGame game.Game
	if err := json.Unmarshal(created.Body.Bytes(), &createdGame); err != nil {
		t.Fatal(err)
	}
	if createdGame.ID <= 0 || createdGame.Title != "Portal 2" {
		t.Fatalf("created: %+v", createdGame)
	}
	request("GET", created.Header().Get("Location"), "", nil, 200)
	if err := store.SetRole(ctx, "player_1", "user"); err != nil {
		t.Fatal(err)
	}
	request("POST", "/api/v1/games", gameBody, cookie, 403)
	var gameCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM games`).Scan(&gameCount); err != nil || gameCount != 1 {
		t.Fatalf("unauthorized records: %d, %v", gameCount, err)
	}
	if err := store.SetRole(ctx, "missing", "admin"); err == nil {
		t.Fatal("missing user accepted")
	}
	if err := store.SetRole(ctx, "player_1", "owner"); err == nil {
		t.Fatal("invalid role accepted")
	}
	request("GET", "/api/v1/auth/me", "", nil, 401)
	renewed := request("POST", "/api/v1/auth/login", body, cookie, 200).Result().Cookies()[0]
	request("GET", "/api/v1/auth/me", "", cookie, 401)
	if renewed.Value == cookie.Value {
		t.Fatal("session was not rotated")
	}
	logout := request("POST", "/api/v1/auth/logout", "", renewed, 204)
	if logout.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("logout did not clear cookie")
	}
	request("GET", "/api/v1/auth/me", "", renewed, 401)
	request("POST", "/api/v1/auth/logout", "", renewed, 204)
	_, token, err := service.Login(ctx, "player_1", "a very long test password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE sessions SET expires_at=now()-interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CurrentUser(ctx, token); !errors.Is(err, account.ErrUnauthenticated) {
		t.Fatalf("expired session accepted: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	request("POST", "/api/v1/auth/login", body, nil, 500)
}
