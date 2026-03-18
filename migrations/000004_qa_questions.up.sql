CREATE TABLE IF NOT EXISTS questions (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    visitor_id TEXT NOT NULL,
    content TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'active', -- active, answered, archived
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_questions_event_id ON questions(event_id);
CREATE INDEX idx_questions_status ON questions(status);

CREATE TABLE IF NOT EXISTS question_upvotes (
    question_id UUID NOT NULL REFERENCES questions(id) ON DELETE CASCADE,
    visitor_id TEXT NOT NULL,
    PRIMARY KEY (question_id, visitor_id)
);
