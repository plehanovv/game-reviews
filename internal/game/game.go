package game

type Game struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	ReleaseYear int    `json:"release_year"`
}
