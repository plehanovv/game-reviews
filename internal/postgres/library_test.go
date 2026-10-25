package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"game-reviews/internal/account"
	"game-reviews/internal/game"
	"game-reviews/internal/httpapi"
	"game-reviews/internal/library"
	"game-reviews/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func libraryTestDB(t *testing.T) (*sql.DB, context.Context) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	admin, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	schema := fmt.Sprintf("library_test_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if _, err := admin.ExecContext(cleanup, `DROP SCHEMA `+schema+` CASCADE`); err != nil {
			t.Error(err)
		}
	})
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		t.Fatal("invalid database config")
	}
	cfg.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*cfg)
	t.Cleanup(func() { db.Close() })
	if err := NewLibrary(db).CheckReady(ctx); err == nil {
		t.Fatal("unmigrated library ready")
	}
	for i := 0; i < 2; i++ {
		if err := migrations.Up(ctx, db); err != nil {
			t.Fatal(err)
		}
	}
	return db, ctx
}

func TestLibraryIntegration(t *testing.T) {
	db, ctx := libraryTestDB(t)
	store := NewLibrary(db)
	if err := store.CheckReady(ctx); err != nil {
		t.Fatal(err)
	}
	auth := account.NewService(NewAccounts(db))
	u1, err := auth.Register(ctx, "alice", "a long test passphrase")
	if err != nil {
		t.Fatal(err)
	}
	_, err = auth.Register(ctx, "bob", "a long test passphrase")
	if err != nil {
		t.Fatal(err)
	}
	_, token1, err := auth.Login(ctx, "alice", "a long test passphrase")
	if err != nil {
		t.Fatal(err)
	}
	_, token2, err := auth.Login(ctx, "bob", "a long test passphrase")
	if err != nil {
		t.Fatal(err)
	}
	catalog := NewCatalog(db)
	g1, err := catalog.Create(ctx, game.CreateInput{Title: "First", ReleaseYear: 2020})
	if err != nil {
		t.Fatal(err)
	}
	g2, err := catalog.Create(ctx, game.CreateInput{Title: "Second", ReleaseYear: 2021})
	if err != nil {
		t.Fatal(err)
	}
	h := httpapi.NewApplication(catalog, store.CheckReady, auth, false, httpapi.WithLibrary(store))
	request := func(method, path, body, token string, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(&http.Cookie{Name: "game_reviews_session", Value: token})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: %d, want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w
	}
	path := fmt.Sprintf("/api/v1/me/library/%d", g1.ID)
	first := request("PUT", path, `{"status":"planned"}`, token1, 200)
	repeat := request("PUT", path, `{"status":"planned"}`, token1, 200)
	if first.Body.String() != repeat.Body.String() {
		t.Fatal("repeated PUT changed entry")
	}
	request("PUT", path, `{"status":"playing"}`, token1, 200)
	request("PUT", path, `{"status":"completed"}`, token2, 200)
	for _, tc := range []struct {
		token  string
		status library.Status
	}{{token1, library.Playing}, {token2, library.Completed}} {
		w := request("GET", "/api/v1/me/library", "", tc.token, 200)
		var page struct {
			Items []library.Entry `json:"items"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 1 || page.Items[0].Status != tc.status {
			t.Fatalf("owner isolation failed: %+v", page)
		}
	}
	request("DELETE", path, "", token2, 204)
	request("DELETE", path, "", token2, 204)
	entries, err := store.List(ctx, u1.ID, library.ListOptions{Limit: 20})
	if err != nil || len(entries) != 1 {
		t.Fatal("another user deleted Alice's entry")
	}
	request("PUT", fmt.Sprintf("/api/v1/me/library/%d", g2.ID), `{"status":"on_hold"}`, token1, 200)
	page := request("GET", "/api/v1/me/library?limit=1", "", token1, 200)
	var decoded struct {
		Items []library.Entry `json:"items"`
		Next  *int64          `json:"next_after_id"`
	}
	if err := json.Unmarshal(page.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Items) != 1 || decoded.Next == nil || *decoded.Next != g1.ID {
		t.Fatalf("invalid page: %+v", decoded)
	}
	last := request("GET", fmt.Sprintf("/api/v1/me/library?limit=1&after_id=%d", g1.ID), "", token1, 200)
	decoded.Next = nil
	if err := json.Unmarshal(last.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Items) != 1 || decoded.Items[0].GameID != g2.ID || decoded.Next != nil {
		t.Fatalf("invalid final page: %+v", decoded)
	}
	request("PUT", "/api/v1/me/library/9223372036854775807", `{"status":"planned"}`, token1, 404)
	if _, err := db.ExecContext(ctx, `UPDATE library_entries SET status='invalid' WHERE user_id=$1`, u1.ID); err == nil {
		t.Fatal("database accepted invalid status")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.Put(ctx, u1.ID, g1.ID, library.Dropped); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM library_entries WHERE user_id=$1 AND game_id=$2`, u1.ID, g1.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("concurrent duplicates: %d %v", count, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.CheckReady(ctx); err == nil {
		t.Fatal("closed library ready")
	}
	request("GET", "/api/v1/me/library", "", token1, 500)
}
