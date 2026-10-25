package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"game-reviews/internal/game"
)

type Catalog struct{ db *sql.DB }

func NewCatalog(db *sql.DB) *Catalog { return &Catalog{db: db} }

func (c *Catalog) Create(ctx context.Context, input game.CreateInput) (game.Game, error) {
	input, err := input.Validate()
	if err != nil {
		return game.Game{}, err
	}
	var item game.Game
	err = c.db.QueryRowContext(ctx, `INSERT INTO games(title, release_year) VALUES ($1,$2) RETURNING id,title,release_year`, input.Title, input.ReleaseYear).Scan(&item.ID, &item.Title, &item.ReleaseYear)
	if err != nil {
		return game.Game{}, fmt.Errorf("create game: %w", err)
	}
	return item, nil
}

func (c *Catalog) List(ctx context.Context, options game.ListOptions) ([]game.Game, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	rows, err := c.db.QueryContext(ctx, `SELECT id, title, release_year FROM games WHERE id > $1 ORDER BY id LIMIT $2`, options.AfterID, options.Limit)
	if err != nil {
		return nil, fmt.Errorf("list games: %w", err)
	}
	defer rows.Close()
	items := make([]game.Game, 0)
	for rows.Next() {
		var item game.Game
		if err := rows.Scan(&item.ID, &item.Title, &item.ReleaseYear); err != nil {
			return nil, fmt.Errorf("scan game: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read games: %w", err)
	}
	return items, nil
}

// CheckReady verifies both connectivity and the columns required by this API.
func (c *Catalog) CheckReady(ctx context.Context) error {
	rows, err := c.db.QueryContext(ctx, `SELECT id, title, release_year FROM games LIMIT 0`)
	if err != nil {
		return err
	}
	return rows.Close()
}

func (c *Catalog) Get(ctx context.Context, id int64) (game.Game, error) {
	var item game.Game
	err := c.db.QueryRowContext(ctx, `SELECT id, title, release_year FROM games WHERE id = $1`, id).Scan(&item.ID, &item.Title, &item.ReleaseYear)
	if errors.Is(err, sql.ErrNoRows) {
		return game.Game{}, game.ErrNotFound
	}
	if err != nil {
		return game.Game{}, fmt.Errorf("get game: %w", err)
	}
	return item, nil
}
