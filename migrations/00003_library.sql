-- +goose Up
CREATE TABLE library_entries (
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    game_id bigint NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    status text NOT NULL CHECK (status IN ('planned', 'playing', 'completed', 'on_hold', 'dropped')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, game_id)
);
CREATE INDEX library_entries_game_id_idx ON library_entries(game_id);

-- +goose Down
DROP TABLE library_entries;
