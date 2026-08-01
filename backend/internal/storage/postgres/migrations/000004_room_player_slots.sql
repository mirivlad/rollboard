ALTER TABLE room_members ADD COLUMN player_id TEXT;

CREATE UNIQUE INDEX room_members_room_player_id_unique
    ON room_members (room_id, player_id)
    WHERE player_id IS NOT NULL;
