package game

import (
	"context"
	"errors"
	"sort"
)

var ErrNotFound = errors.New("game not found")

// Catalog is an immutable in-memory snapshot, safe for concurrent readers.
type Catalog struct{ games []Game }

func NewCatalog(games []Game) *Catalog {
	items := make([]Game, len(games))
	copy(items, games)
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return &Catalog{games: items}
}

func (c *Catalog) List(ctx context.Context, options ListOptions) ([]Game, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	items := make([]Game, 0, options.Limit)
	for _, item := range c.games {
		if item.ID > options.AfterID {
			items = append(items, item)
			if len(items) == options.Limit {
				break
			}
		}
	}
	return items, nil
}

func (c *Catalog) Get(ctx context.Context, id int64) (Game, error) {
	if err := ctx.Err(); err != nil {
		return Game{}, err
	}
	for _, item := range c.games {
		if item.ID == id {
			return item, nil
		}
	}
	return Game{}, ErrNotFound
}
