package postgres

import (
	"context"
	"database/sql"
	"errors"

	"game-reviews/internal/game"
	"game-reviews/internal/rating"
	"github.com/jackc/pgx/v5/pgconn"
)

type Ratings struct{ db *sql.DB }

func NewRatings(db *sql.DB) *Ratings { return &Ratings{db: db} }

func (s *Ratings) Put(ctx context.Context, userID, gameID int64, score int) (rating.Rating, error) {
	if !rating.ValidScore(score) {
		return rating.Rating{}, rating.ErrInvalidScore
	}
	var item rating.Rating
	err := s.db.QueryRowContext(ctx, `INSERT INTO ratings(user_id,game_id,score) VALUES ($1,$2,$3)
ON CONFLICT (user_id,game_id) DO UPDATE SET score=EXCLUDED.score,
updated_at=CASE WHEN ratings.score IS DISTINCT FROM EXCLUDED.score THEN now() ELSE ratings.updated_at END
RETURNING game_id,score,created_at,updated_at`, userID, gameID, score).Scan(&item.GameID, &item.Score, &item.CreatedAt, &item.UpdatedAt)
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) && pgerr.Code == "23503" && pgerr.ConstraintName == "ratings_game_id_fkey" {
		return item, game.ErrNotFound
	}
	return item, err
}

func (s *Ratings) Get(ctx context.Context, userID, gameID int64) (rating.Rating, error) {
	var item rating.Rating
	err := s.db.QueryRowContext(ctx, `SELECT game_id,score,created_at,updated_at FROM ratings WHERE user_id=$1 AND game_id=$2`, userID, gameID).Scan(&item.GameID, &item.Score, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return item, rating.ErrNotFound
	}
	return item, err
}

func (s *Ratings) Delete(ctx context.Context, userID, gameID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM ratings WHERE user_id=$1 AND game_id=$2`, userID, gameID)
	return err
}

func (s *Ratings) Summary(ctx context.Context, gameID int64) (rating.Summary, error) {
	var summary rating.Summary
	var average sql.NullFloat64
	err := s.db.QueryRowContext(ctx, `SELECT g.id,count(r.score),round(avg(r.score),2)::double precision
FROM games g LEFT JOIN ratings r ON r.game_id=g.id WHERE g.id=$1 GROUP BY g.id`, gameID).Scan(&summary.GameID, &summary.Count, &average)
	if errors.Is(err, sql.ErrNoRows) {
		return summary, game.ErrNotFound
	}
	if average.Valid {
		summary.Average = &average.Float64
	}
	return summary, err
}

func (s *Ratings) CheckReady(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT user_id,game_id,score,created_at,updated_at FROM ratings LIMIT 0`)
	if err != nil {
		return err
	}
	return rows.Close()
}
