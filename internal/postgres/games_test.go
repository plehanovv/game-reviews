package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"game-reviews/internal/game"
	"game-reviews/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func TestCatalogIntegration(t *testing.T) {
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
	schema := fmt.Sprintf("catalog_test_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
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
	if err := NewCatalog(db).CheckReady(ctx); err == nil {
		t.Fatal("unmigrated database reported ready")
	}
	for i := 0; i < 2; i++ {
		if err := migrations.Up(ctx, db); err != nil {
			t.Fatal(err)
		}
	}
	catalog := NewCatalog(db)
	if err := catalog.CheckReady(ctx); err != nil {
		t.Fatal(err)
	}
	items, err := catalog.List(ctx, game.ListOptions{Limit: 20})
	if err != nil || items == nil || len(items) != 0 {
		t.Fatalf("empty catalog: %v, %v", items, err)
	}
	var id int64
	if err := db.QueryRowContext(ctx, `INSERT INTO games(title, release_year) VALUES ($1, $2) RETURNING id`, "Test game", 2020).Scan(&id); err != nil {
		t.Fatal(err)
	}
	got, err := catalog.Get(ctx, id)
	if err != nil || got.Title != "Test game" || got.ReleaseYear != 2020 {
		t.Fatalf("get: %+v, %v", got, err)
	}
	items, err = catalog.List(ctx, game.ListOptions{Limit: 20})
	if err != nil || len(items) != 1 || items[0] != got {
		t.Fatalf("list: %+v, %v", items, err)
	}
	if _, err := catalog.Get(ctx, id+1); !errors.Is(err, game.ErrNotFound) {
		t.Fatalf("missing game: %v", err)
	}
	for _, title := range []any{"", "   ", nil} {
		if _, err := db.ExecContext(ctx, `INSERT INTO games(title, release_year) VALUES ($1, 2020)`, title); err == nil {
			t.Fatal("invalid title accepted")
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO games(title, release_year) VALUES ('Bad year', 0)`); err == nil {
		t.Fatal("invalid year accepted")
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, err := catalog.List(canceled, game.ListOptions{Limit: 20}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
	// A fresh connection sees the same committed data.
	var secondID, thirdID int64
	if err := db.QueryRowContext(ctx, `INSERT INTO games(title, release_year) VALUES ('Second', 2021) RETURNING id`).Scan(&secondID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO games(title, release_year) VALUES ('Third', 2022) RETURNING id`).Scan(&thirdID); err != nil {
		t.Fatal(err)
	}
	page, err := catalog.List(ctx, game.ListOptions{AfterID: id, Limit: 1})
	if err != nil || len(page) != 1 || page[0].ID != secondID {
		t.Fatalf("first page: %+v, %v", page, err)
	}
	page, err = catalog.List(ctx, game.ListOptions{AfterID: secondID, Limit: 1})
	if err != nil || len(page) != 1 || page[0].ID != thirdID {
		t.Fatalf("second page: %+v, %v", page, err)
	}
	page, err = catalog.List(ctx, game.ListOptions{AfterID: thirdID, Limit: 1})
	if err != nil || len(page) != 0 || page == nil {
		t.Fatalf("last page: %+v, %v", page, err)
	}
	other := stdlib.OpenDB(*cfg)
	defer other.Close()
	if _, err := NewCatalog(other).Get(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := catalog.CheckReady(ctx); err == nil {
		t.Fatal("closed database reported ready")
	}
	if _, err := catalog.Get(ctx, id); err == nil || errors.Is(err, game.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("database failure must not become 404: %v", err)
	}
}
