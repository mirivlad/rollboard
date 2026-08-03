-- Hotseat playtest sessions belong to the account that started them.
-- The column stays nullable so pre-existing prototype rows survive the
-- migration; owner-scoped reads match on owner_user_id and therefore never
-- return those unowned rows.
ALTER TABLE sessions ADD COLUMN owner_user_id UUID REFERENCES users(id);

CREATE INDEX sessions_owner_user_id_idx ON sessions (owner_user_id);
