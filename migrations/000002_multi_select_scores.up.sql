-- 000001_schema.up.sql に統合済み。このファイルは後方互換のために存在する。
-- 各 DDL は IF NOT EXISTS / IF NOT EXISTS ガードで冪等にしてあるため、
-- 000001 を適用済みの新規DBではすべて no-op になる。

-- 複数選択: 設問ごとの最大選択数（1=単一選択）
ALTER TABLE polls ADD COLUMN IF NOT EXISTS max_selections INT NOT NULL DEFAULT 1;

-- 複数選択: votes の一意制約を (poll_id, visitor_id, option_id) に変更
DROP INDEX IF EXISTS idx_votes_poll_visitor;
CREATE UNIQUE INDEX IF NOT EXISTS idx_votes_poll_visitor_option ON votes(poll_id, visitor_id, option_id);

-- 投票送信済みトラッキング（1端末1回制御の主キー）
CREATE TABLE IF NOT EXISTS vote_submissions (
    poll_id      UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    visitor_id   TEXT NOT NULL,
    submitted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY  (poll_id, visitor_id)
);

-- クイズ参加者の累計得点
CREATE TABLE IF NOT EXISTS participant_scores (
    event_id    UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    visitor_id  TEXT NOT NULL,
    nickname    TEXT NOT NULL DEFAULT '',
    total_score INT  NOT NULL DEFAULT 0,
    PRIMARY KEY (event_id, visitor_id)
);

CREATE INDEX IF NOT EXISTS idx_participant_scores_rank
    ON participant_scores(event_id, total_score DESC);
