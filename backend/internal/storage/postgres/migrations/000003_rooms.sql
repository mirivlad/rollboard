CREATE TABLE rooms (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_version_id UUID NOT NULL REFERENCES game_versions(id),
    host_user_id UUID NOT NULL REFERENCES users(id),
    title TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 120),
    max_players SMALLINT NOT NULL CHECK (max_players BETWEEN 2 AND 8),
    status TEXT NOT NULL DEFAULT 'lobby' CHECK (status IN ('lobby', 'active', 'finished')),
    sequence BIGINT NOT NULL DEFAULT 0 CHECK (sequence >= 0),
    session_json JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE room_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    actor_kind TEXT NOT NULL CHECK (actor_kind IN ('user', 'guest')),
    actor_id UUID NOT NULL,
    display_name TEXT NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 64),
    muted_at TIMESTAMPTZ,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (room_id, actor_kind, actor_id)
);

CREATE INDEX room_members_room_id_idx ON room_members (room_id, joined_at);

CREATE TABLE room_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    member_id UUID NOT NULL REFERENCES room_members(id) ON DELETE CASCADE,
    body TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 1000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX room_messages_room_id_created_at_idx ON room_messages (room_id, created_at, id);
