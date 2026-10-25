package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"game-reviews/internal/postgres"
	"game-reviews/migrations"
)

func main() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	db, err := postgres.Open(ctx, url)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	defer db.Close()
	if err := migrations.Up(ctx, db); err != nil {
		slog.Error("migration failed; check database schema and migration files")
		os.Exit(1)
	}
	slog.Info("database migrations applied")
}
