DROP INDEX IF EXISTS idx_participant_scores_rank;
DROP TABLE IF EXISTS participant_scores;
DROP TABLE IF EXISTS vote_submissions;
DROP INDEX IF EXISTS idx_votes_poll_visitor_option;

-- 複数選択の重複行を除去してから旧ユニーク制約を復元する
DELETE FROM votes
WHERE id NOT IN (
    SELECT DISTINCT ON (poll_id, visitor_id) id
    FROM votes
    ORDER BY poll_id, visitor_id, created_at ASC
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_votes_poll_visitor ON votes(poll_id, visitor_id);

ALTER TABLE polls DROP COLUMN IF EXISTS max_selections;
