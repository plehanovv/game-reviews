package library

import (
	"errors"
	"time"
)

var ErrInvalidStatus = errors.New("invalid library status")

type Status string

const (
	Planned   Status = "planned"
	Playing   Status = "playing"
	Completed Status = "completed"
	OnHold    Status = "on_hold"
	Dropped   Status = "dropped"
)

func (s Status) Valid() bool {
	switch s {
	case Planned, Playing, Completed, OnHold, Dropped:
		return true
	}
	return false
}

type Entry struct {
	GameID    int64     `json:"game_id"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ListOptions struct {
	AfterGameID int64
	Limit       int
}

func (o ListOptions) Validate() error {
	if o.AfterGameID < 0 || o.Limit < 1 || o.Limit > 101 {
		return errors.New("invalid library list options")
	}
	return nil
}
