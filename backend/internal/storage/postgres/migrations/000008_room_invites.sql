-- Sharing a room previously meant sending someone its UUID by hand. An invite
-- token is a separate capability so it can be rotated without changing the
-- room's identity, and so the room ID never has to travel through a chat
-- message to be useful.
ALTER TABLE rooms ADD COLUMN invite_token TEXT;

-- Existing rooms get a token so the column can be made mandatory. base64 is
-- translated to the URL-safe alphabet because these end up in links.
UPDATE rooms
SET invite_token = replace(replace(replace(encode(gen_random_bytes(24), 'base64'), '+', '-'), '/', '_'), '=', '')
WHERE invite_token IS NULL;

ALTER TABLE rooms ALTER COLUMN invite_token SET NOT NULL;
ALTER TABLE rooms ADD CONSTRAINT rooms_invite_token_key UNIQUE (invite_token);
