package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"strings"
	"time"

	"game-reviews/internal/postgres"
)

func main() {
	username := flag.String("username", "", "Existing username")
	role := flag.String("role", "", "Role to assign: admin or user")
	flag.Parse()
	name := strings.ToLower(strings.TrimSpace(*username))
	if name == "" || (*role != "admin" && *role != "user") || flag.NArg() != 0 {
		slog.Error("usage: go run ./cmd/admin -username NAME -role admin|user")
		os.Exit(1)
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := postgres.Open(ctx, url)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	defer db.Close()
	if err := postgres.NewAccounts(db).SetRole(ctx, name, *role); err != nil {
		slog.Error("role update failed; verify username and database schema")
		os.Exit(1)
	}
	slog.Info("user role updated", "username", name, "role", *role)
}
