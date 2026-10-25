package game

import (
	"errors"
	"strings"
	"unicode/utf8"
)

var ErrInvalidGame = errors.New("invalid game")

type CreateInput struct {
	Title       string `json:"title"`
	ReleaseYear int    `json:"release_year"`
}

func (input CreateInput) Validate() (CreateInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	length := utf8.RuneCountInString(input.Title)
	if !utf8.ValidString(input.Title) || strings.ContainsRune(input.Title, 0) || length < 1 || length > 300 || input.ReleaseYear < 1950 || input.ReleaseYear > 2100 {
		return input, ErrInvalidGame
	}
	return input, nil
}
