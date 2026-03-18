-- イベント（ホストが作成する投票セッション）
CREATE TABLE IF NOT EXISTS events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title           TEXT NOT NULL,
    owner_token     TEXT NOT NULL,                           -- bcryptハッシュ済みホスト認証トークン
    status          TEXT NOT NULL DEFAULT 'live',            -- live | closed
    current_poll_id UUID,                                    -- 現在アクティブな投票
    nickname_policy VARCHAR(20) NOT NULL DEFAULT 'optional', -- optional | required | anonymous
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 投票（アンケートまたはクイズ）
CREATE TABLE IF NOT EXISTS polls (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id       UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    title          TEXT NOT NULL,
    status         TEXT NOT NULL DEFAULT 'open',             -- open | closed
    is_quiz        BOOLEAN NOT NULL DEFAULT FALSE,
    points         INT NOT NULL DEFAULT 0,
    max_selections INT NOT NULL DEFAULT 1,                   -- 1=単一選択、2以上=複数選択
    created_at     TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_polls_event ON polls(event_id);

-- 選択肢
CREATE TABLE IF NOT EXISTS options (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    poll_id    UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    label      TEXT NOT NULL,
    item_order INT NOT NULL,
    is_correct BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 投票票（poll_id + visitor_id + option_id でユニーク：複数選択対応）
CREATE TABLE IF NOT EXISTS votes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    poll_id    UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    option_id  UUID NOT NULL REFERENCES options(id) ON DELETE CASCADE,
    visitor_id TEXT NOT NULL,
    nickname   TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

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

-- iframe埋め込みトークン
CREATE TABLE IF NOT EXISTS embed_tokens (
    token           TEXT PRIMARY KEY,
    poll_id         UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    allowed_origins TEXT[],
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
