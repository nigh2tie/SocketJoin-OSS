-- 削除済み poll を参照している orphan を先に NULL に戻す
UPDATE events
SET current_poll_id = NULL
WHERE current_poll_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM polls WHERE polls.id = events.current_poll_id);

ALTER TABLE events
    ADD CONSTRAINT fk_events_current_poll
    FOREIGN KEY (current_poll_id) REFERENCES polls(id) ON DELETE SET NULL;
