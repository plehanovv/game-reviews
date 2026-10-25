package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"game-reviews/internal/account"
	"github.com/jackc/pgx/v5/pgconn"
)

type Accounts struct{ db *sql.DB }

func NewAccounts(db *sql.DB) *Accounts { return &Accounts{db: db} }

// SetRole is used by the local administration command, not by public HTTP routes.
func (s *Accounts) SetRole(ctx context.Context, username, role string) error {
	if role != "admin" && role != "user" {
		return errors.New("invalid role")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE users SET role=$1 WHERE username=$2`, role, username)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return errors.New("user not found")
	}
	return nil
}

func (s *Accounts) CreateUser(ctx context.Context, username, passwordHash string) (account.User, error) {
	var u account.User
	err := s.db.QueryRowContext(ctx, `INSERT INTO users(username, password_hash) VALUES ($1, $2) RETURNING id, username, role`, username, passwordHash).Scan(&u.ID, &u.Username, &u.Role)
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) && pgerr.Code == "23505" && pgerr.ConstraintName == "users_username_key" {
		return u, account.ErrUsernameTaken
	}
	return u, err
}

func (s *Accounts) FindUser(ctx context.Context, username string) (account.User, string, error) {
	var u account.User
	var hash string
	err := s.db.QueryRowContext(ctx, `SELECT id, username, role, password_hash FROM users WHERE username=$1`, username).Scan(&u.ID, &u.Username, &u.Role, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return u, "", account.ErrUnauthenticated
	}
	return u, hash, err
}

func (s *Accounts) CreateSession(ctx context.Context, hash []byte, userID int64, expires time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO sessions(token_hash, user_id, expires_at) VALUES ($1,$2,$3)`, hash, userID, expires)
	return err
}

func (s *Accounts) SessionUser(ctx context.Context, hash []byte) (account.User, error) {
	var u account.User
	err := s.db.QueryRowContext(ctx, `SELECT u.id, u.username, u.role FROM users u JOIN sessions s ON s.user_id=u.id WHERE s.token_hash=$1 AND s.expires_at > now()`, hash).Scan(&u.ID, &u.Username, &u.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return u, account.ErrUnauthenticated
	}
	return u, err
}

func (s *Accounts) DeleteSession(ctx context.Context, hash []byte) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash=$1`, hash)
	return err
}

func (s *Accounts) CheckReady(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT u.id,u.username,u.password_hash,u.role,s.token_hash,s.expires_at FROM users u LEFT JOIN sessions s ON s.user_id=u.id LIMIT 0`)
	if err != nil {
		return err
	}
	return rows.Close()
}
