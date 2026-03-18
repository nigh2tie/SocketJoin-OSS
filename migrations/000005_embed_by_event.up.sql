-- Clear existing tokens as they are tied to polls, which is incompatible with the new event-level architecture
TRUNCATE TABLE embed_tokens;

ALTER TABLE embed_tokens
  DROP CONSTRAINT IF EXISTS embed_tokens_poll_id_fkey,
  DROP COLUMN IF EXISTS poll_id;

ALTER TABLE embed_tokens
  ADD COLUMN event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE;
