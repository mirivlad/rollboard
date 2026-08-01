CREATE TABLE games (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    version INTEGER NOT NULL CHECK (version > 0),
    definition_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    game_id TEXT NOT NULL REFERENCES games(id),
    game_version INTEGER NOT NULL CHECK (game_version > 0),
    session_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX sessions_game_id_idx ON sessions (game_id);
