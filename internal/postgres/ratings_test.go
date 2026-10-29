package postgres

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"game-reviews/internal/account"
	"game-reviews/internal/game"
	"game-reviews/internal/httpapi"
	"game-reviews/internal/library"
	"game-reviews/internal/rating"
)

func TestRatingsIntegration(t *testing.T) {
	db, ctx := libraryTestDB(t)
	store := NewRatings(db)
	if err := store.CheckReady(ctx); err != nil {
		t.Fatal(err)
	}
	auth := account.NewService(NewAccounts(db))
	alice, err := auth.Register(ctx, "alice", "a long test passphrase")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := auth.Register(ctx, "bob", "a long test passphrase")
	if err != nil {
		t.Fatal(err)
	}
	_, tokenA, err := auth.Login(ctx, "alice", "a long test passphrase")
	if err != nil {
		t.Fatal(err)
	}
	_, tokenB, err := auth.Login(ctx, "bob", "a long test passphrase")
	if err != nil {
		t.Fatal(err)
	}
	catalog := NewCatalog(db)
	g, err := catalog.Create(ctx, game.CreateInput{Title: "Example", ReleaseYear: 2020})
	if err != nil {
		t.Fatal(err)
	}
	h := httpapi.NewApplication(catalog, store.CheckReady, auth, false, httpapi.WithRatings(store))
	path := fmt.Sprintf("/api/v1/me/ratings/%d", g.ID)
	request := func(method, path, body, token string, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.AddCookie(&http.Cookie{Name: "game_reviews_session", Value: token})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: %d, want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w
	}
	summary := func(count int64, average *float64) {
		t.Helper()
		w := request("GET", fmt.Sprintf("/api/v1/games/%d/rating", g.ID), "", "", 200)
		var got rating.Summary
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Count != count || (got.Average == nil) != (average == nil) {
			t.Fatalf("summary: %+v", got)
		}
		if average != nil && *got.Average != *average {
			t.Fatalf("average %v want %v", *got.Average, *average)
		}
	}
	summary(0, nil)
	request("GET", path, "", tokenA, 404)
	first := request("PUT", path, `{"score":8}`, tokenA, 200)
	repeat := request("PUT", path, `{"score":8}`, tokenA, 200)
	if first.Body.String() != repeat.Body.String() {
		t.Fatal("repeat changed timestamp")
	}
	request("PUT", path, `{"score":3}`, tokenB, 200)
	avg := 5.5
	summary(2, &avg)
	request("PUT", path, `{"score":10}`, tokenA, 200)
	avg = 6.5
	summary(2, &avg)
	b, err := store.Get(ctx, bob.ID, g.ID)
	if err != nil || b.Score != 3 {
		t.Fatal("Alice overwrote Bob")
	}
	lib := NewLibrary(db)
	if _, err := lib.Put(ctx, alice.ID, g.ID, library.Planned); err != nil {
		t.Fatal(err)
	}
	if err := lib.Delete(ctx, alice.ID, g.ID); err != nil {
		t.Fatal(err)
	}
	summary(2, &avg)
	request("DELETE", path, "", tokenA, 204)
	request("DELETE", path, "", tokenA, 204)
	avg = 3
	summary(1, &avg)
	if _, err := store.Get(ctx, alice.ID, g.ID); !errors.Is(err, rating.ErrNotFound) {
		t.Fatal("deleted rating remains")
	}
	request("PUT", "/api/v1/me/ratings/9223372036854775807", `{"score":8}`, tokenA, 404)
	request("GET", "/api/v1/games/9223372036854775807/rating", "", "", 404)
	for _, score := range []int{0, 11} {
		if _, err := store.Put(ctx, alice.ID, g.ID, score); !errors.Is(err, rating.ErrInvalidScore) {
			t.Fatal("invalid score accepted by store")
		}
		if _, err := db.ExecContext(ctx, `UPDATE ratings SET score=$1 WHERE user_id=$2`, score, bob.ID); err == nil {
			t.Fatal("invalid score accepted by database")
		}
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.Put(ctx, alice.ID, g.ID, 9); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	avg = 6
	summary(2, &avg)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.CheckReady(ctx); err == nil {
		t.Fatal("closed database ready")
	}
	request("GET", fmt.Sprintf("/api/v1/games/%d/rating", g.ID), "", "", 500)
}
