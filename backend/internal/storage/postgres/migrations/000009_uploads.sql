-- Uploaded images had no record anywhere: the file went to disk, the name was
-- returned, and nothing knew whose it was or how much room it took. Any signed
-- in account could therefore upload unique images until the disk filled, and
-- nothing could ever be deleted because nothing knew what to delete.
--
-- One row per (image, owner) rather than per image, because the name is the
-- content hash: two accounts uploading the same picture share one file on disk
-- and are each charged for it. The file is removed when the last row for it is.
CREATE TABLE IF NOT EXISTS uploads (
    name          TEXT NOT NULL,
    owner_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    size_bytes    BIGINT NOT NULL CHECK (size_bytes > 0),
    content_type  TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (name, owner_user_id)
);

-- The two questions asked on every upload: what does this account already hold,
-- and what does the deployment hold in total.
CREATE INDEX IF NOT EXISTS uploads_owner_idx ON uploads (owner_user_id);
