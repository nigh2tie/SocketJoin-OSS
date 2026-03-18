-- This assumes a simple rollback where we just drop event_id and add back poll_id.
-- Data will be lost upon rollback, which is standard for structural changes.
ALTER TABLE embed_tokens
  DROP CONSTRAINT IF EXISTS embed_tokens_event_id_fkey,
  DROP COLUMN IF EXISTS event_id;

ALTER TABLE embed_tokens
  ADD COLUMN poll_id UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE;

TRUNCATE TABLE embed_tokens;
