-- +goose Up
CREATE TABLE ratings (
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    game_id bigint NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    score smallint NOT NULL CHECK (score BETWEEN 1 AND 10),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, game_id)
);
CREATE INDEX ratings_game_id_idx ON ratings(game_id);

-- +goose Down
DROP TABLE ratings;
