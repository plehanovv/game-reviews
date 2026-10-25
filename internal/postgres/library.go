package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"game-reviews/internal/game"
	"game-reviews/internal/library"
	"github.com/jackc/pgx/v5/pgconn"
)

type Library struct{ db *sql.DB }

func NewLibrary(db *sql.DB) *Library { return &Library{db: db} }

func (s *Library) Put(ctx context.Context, userID, gameID int64, status library.Status) (library.Entry, error) {
	if !status.Valid() {
		return library.Entry{}, library.ErrInvalidStatus
	}
	var entry library.Entry
	err := s.db.QueryRowContext(ctx, `INSERT INTO library_entries(user_id,game_id,status) VALUES ($1,$2,$3)
ON CONFLICT (user_id,game_id) DO UPDATE SET status=EXCLUDED.status,
updated_at=CASE WHEN library_entries.status IS DISTINCT FROM EXCLUDED.status THEN now() ELSE library_entries.updated_at END
RETURNING game_id,status,created_at,updated_at`, userID, gameID, status).Scan(&entry.GameID, &entry.Status, &entry.CreatedAt, &entry.UpdatedAt)
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) && pgerr.Code == "23503" && pgerr.ConstraintName == "library_entries_game_id_fkey" {
		return entry, game.ErrNotFound
	}
	if err != nil {
		return entry, fmt.Errorf("put library entry: %w", err)
	}
	return entry, nil
}

func (s *Library) List(ctx context.Context, userID int64, options library.ListOptions) ([]library.Entry, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT game_id,status,created_at,updated_at FROM library_entries WHERE user_id=$1 AND game_id>$2 ORDER BY game_id LIMIT $3`, userID, options.AfterGameID, options.Limit)
	if err != nil {
		return nil, fmt.Errorf("list library: %w", err)
	}
	defer rows.Close()
	items := make([]library.Entry, 0)
	for rows.Next() {
		var entry library.Entry
		if err := rows.Scan(&entry.GameID, &entry.Status, &entry.CreatedAt, &entry.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, entry)
	}
	return items, rows.Err()
}

func (s *Library) Delete(ctx context.Context, userID, gameID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM library_entries WHERE user_id=$1 AND game_id=$2`, userID, gameID)
	return err
}

func (s *Library) CheckReady(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT user_id,game_id,status,created_at,updated_at FROM library_entries LIMIT 0`)
	if err != nil {
		return err
	}
	return rows.Close()
}
