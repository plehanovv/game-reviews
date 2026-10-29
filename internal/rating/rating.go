package rating

import (
	"errors"
	"time"
)

var ErrInvalidScore = errors.New("score must be between 1 and 10")
var ErrNotFound = errors.New("rating not found")

func ValidScore(score int) bool { return score >= 1 && score <= 10 }

type Rating struct {
	GameID    int64     `json:"game_id"`
	Score     int       `json:"score"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Summary struct {
	GameID  int64    `json:"game_id"`
	Count   int64    `json:"count"`
	Average *float64 `json:"average"`
}
