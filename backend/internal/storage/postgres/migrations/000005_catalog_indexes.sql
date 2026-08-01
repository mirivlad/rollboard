CREATE INDEX games_owner_updated_at_idx
    ON games (owner_user_id, updated_at DESC, id DESC);

CREATE INDEX game_versions_game_published_at_idx
    ON game_versions (game_id, published_at DESC, id DESC);
