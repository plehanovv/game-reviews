package game

import (
	"context"
	"errors"
	"testing"
)

func TestCatalogOwnsItsData(t *testing.T) {
	want := Game{ID: 1, Title: "Example", ReleaseYear: 2020}
	source := []Game{want}
	catalog := NewCatalog(source)
	source[0].Title = "changed input"
	listed, err := catalog.List(context.Background(), ListOptions{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	listed[0].Title = "changed output"
	got, err := catalog.Get(context.Background(), 1)
	if err != nil || got != want {
		t.Fatalf("catalog was modified: %+v, %v", got, err)
	}
	if _, err := catalog.Get(context.Background(), 2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
