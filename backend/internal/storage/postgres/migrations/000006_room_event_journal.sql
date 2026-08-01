CREATE TABLE room_events (
    room_id UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    sequence BIGINT NOT NULL CHECK (sequence >= 0),
    event_type TEXT NOT NULL CHECK (char_length(event_type) BETWEEN 1 AND 64),
    payload_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (room_id, sequence)
);

CREATE TABLE room_command_receipts (
    room_id UUID NOT NULL,
    actor_kind TEXT NOT NULL CHECK (actor_kind IN ('user', 'guest')),
    actor_id UUID NOT NULL,
    command_id UUID NOT NULL,
    command_type TEXT NOT NULL CHECK (char_length(command_type) BETWEEN 1 AND 64),
    event_sequence BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (room_id, actor_kind, actor_id, command_id),
    FOREIGN KEY (room_id, event_sequence) REFERENCES room_events(room_id, sequence) ON DELETE CASCADE
);

CREATE INDEX room_events_room_sequence_idx ON room_events (room_id, sequence);
