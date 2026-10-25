-- +goose Up
CREATE TABLE games (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title text NOT NULL CHECK (length(btrim(title)) BETWEEN 1 AND 300),
    release_year integer NOT NULL CHECK (release_year BETWEEN 1950 AND 2100),
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE games;
