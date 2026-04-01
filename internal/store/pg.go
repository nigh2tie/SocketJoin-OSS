// SocketJoin: Real-time event interaction platform.
// Copyright (C) 2026 Q-Q
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Event struct {
	ID             uuid.UUID     `json:"id"`
	Title          string        `json:"title"`
	OwnerToken     string        `json:"owner_token,omitempty"`
	Status         string        `json:"status"` // live, closed
	CurrentPollID  uuid.NullUUID `json:"current_poll_id"`
	NicknamePolicy string        `json:"nickname_policy"` // hidden, optional, required
	ShowQAOnScreen bool          `json:"show_qa_on_screen"`
	CreatedAt      time.Time     `json:"created_at"`
	Role           string        `json:"role,omitempty"` // "host", "moderator"
}

type Moderator struct {
	ID        uuid.UUID `json:"id"`
	EventID   uuid.UUID `json:"event_id"`
	Name      string    `json:"name"`
	Token     string    `json:"token,omitempty"` // Returned only on creation
	CreatedAt time.Time `json:"created_at"`
}

type Question struct {
	ID        uuid.UUID `json:"id"`
	EventID   uuid.UUID `json:"event_id"`
	VisitorID string    `json:"visitor_id,omitempty"` // masked or ignored in public response if anonymous
	Content   string    `json:"content"`
	Status    string    `json:"status"` // "active", "answered", "archived"
	CreatedAt time.Time `json:"created_at"`

	// Virtual fields for responses
	Upvotes   int  `json:"upvotes"`
	IsUpvoted bool `json:"is_upvoted"`
}

type Poll struct {
	ID            uuid.UUID `json:"id"`
	EventID       uuid.UUID `json:"event_id"`
	Title         string    `json:"title"`
	Status        string    `json:"status"`
	IsQuiz        bool      `json:"is_quiz"`
	Points        int       `json:"points"`
	MaxSelections int       `json:"max_selections"`
	CreatedAt     time.Time `json:"created_at"`
	Options       []Option  `json:"options"`
}

type Option struct {
	ID        uuid.UUID `json:"id"`
	PollID    uuid.UUID `json:"poll_id"`
	Label     string    `json:"label"`
	Order     int       `json:"order"`
	IsCorrect bool      `json:"is_correct"`
	CreatedAt time.Time `json:"created_at"`
}

type Vote struct {
	ID        uuid.UUID `json:"id"`
	PollID    uuid.UUID `json:"poll_id"`
	OptionID  uuid.UUID `json:"option_id"`
	VisitorID string    `json:"visitor_id"`
	Nickname  string    `json:"nickname"`
	CreatedAt time.Time `json:"created_at"`
}

type RankingEntry struct {
	Rank       int    `json:"rank"`
	Nickname   string `json:"nickname"`
	TotalScore int    `json:"total_score"`
}

var (
	ErrPollNotFound          = errors.New("poll not found")
	ErrPollNotOpen           = errors.New("poll is not open")
	ErrAlreadyVoted          = errors.New("already voted")
	ErrModeratorLimitReached = errors.New("maximum of 3 moderators allowed")
	ErrQuestionNotFound      = errors.New("question not found")
)

type voteInsertRequest struct {
	PollID    uuid.UUID
	OptionIDs []uuid.UUID
	VisitorID string
	Nickname  string
	Result    chan error
}

type Store struct {
	db           *sql.DB
	voteInsertCh chan voteInsertRequest
}

func NewStore(dbURL string) (*Store, error) {
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}

	s := &Store{
		db:           db,
		voteInsertCh: make(chan voteInsertRequest, 4096),
	}

	go s.runVoteBatcher()

	return s, nil
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// BulkCreatePolls inserts all polls atomically. All succeed or none are saved.
type BulkPollInput struct {
	Title         string
	IsQuiz        bool
	Points        int
	MaxSelections int
	Options       []Option
}

func insertPollTx(ctx context.Context, tx interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
}, eventID uuid.UUID, title string, isQuiz bool, points int, maxSelections int, options []Option) (*Poll, error) {
	if maxSelections < 1 {
		maxSelections = 1
	}

	pollID := uuid.New()
	poll := &Poll{
		ID:            pollID,
		EventID:       eventID,
		Title:         title,
		Status:        "open",
		IsQuiz:        isQuiz,
		Points:        points,
		MaxSelections: maxSelections,
		CreatedAt:     time.Now(),
	}

	_, err := tx.ExecContext(ctx,
		"INSERT INTO polls (id, event_id, title, status, is_quiz, points, max_selections, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
		poll.ID, poll.EventID, poll.Title, poll.Status, poll.IsQuiz, poll.Points, poll.MaxSelections, poll.CreatedAt)
	if err != nil {
		return nil, err
	}

	for i, optData := range options {
		optID := uuid.New()
		opt := Option{
			ID:        optID,
			PollID:    pollID,
			Label:     optData.Label,
			Order:     i,
			IsCorrect: optData.IsCorrect,
			CreatedAt: time.Now(),
		}
		_, err = tx.ExecContext(ctx,
			"INSERT INTO options (id, poll_id, label, item_order, is_correct, created_at) VALUES ($1, $2, $3, $4, $5, $6)",
			opt.ID, opt.PollID, opt.Label, opt.Order, opt.IsCorrect, opt.CreatedAt)
		if err != nil {
			return nil, err
		}
		poll.Options = append(poll.Options, opt)
	}

	return poll, nil
}

type voteIdentity struct {
	PollID    uuid.UUID
	VisitorID string
}

// bulkInsertSubmissionsAndVotes atomically inserts vote_submissions and votes
// in a single transaction. If either insert fails, both are rolled back so the
// voter can retry without being permanently stuck as "already voted".

// ResetPollVotes clears vote data for a poll so it can accept new votes.
// participant_scores are preserved (not rolled back).

// addQuizScoresSQL は完全一致した参加者に得点を加算する SQL。
// $1 = poll_id。ClosePoll との原子的実行のために定数化する。
const addQuizScoresSQL = `
	WITH poll_info AS (
		SELECT p.event_id, p.points
		FROM polls p
		WHERE p.id = $1
	),
	correct_set AS (
		SELECT ARRAY_AGG(o.id ORDER BY o.id) AS correct_ids
		FROM options o
		WHERE o.poll_id = $1 AND o.is_correct = TRUE
	),
	voter_selections AS (
		SELECT v.visitor_id,
		       COALESCE(NULLIF(MAX(v.nickname), ''), 'Anonymous') AS nickname,
		       ARRAY_AGG(v.option_id ORDER BY v.option_id) AS selected_ids
		FROM votes v
		WHERE v.poll_id = $1
		GROUP BY v.visitor_id
	),
	correct_voters AS (
		SELECT vs.visitor_id, vs.nickname
		FROM voter_selections vs, correct_set cs
		WHERE vs.selected_ids = cs.correct_ids
	)
	INSERT INTO participant_scores (event_id, visitor_id, nickname, total_score)
	SELECT pi.event_id, cv.visitor_id, cv.nickname, pi.points
	FROM correct_voters cv, poll_info pi
	WHERE pi.points > 0
	ON CONFLICT (event_id, visitor_id) DO UPDATE
		SET total_score = participant_scores.total_score + EXCLUDED.total_score,
		    nickname     = CASE
		                     WHEN EXCLUDED.nickname != '' THEN EXCLUDED.nickname
		                     ELSE participant_scores.nickname
		                   END
`

// CloseAndScorePoll atomically closes a poll and computes quiz scores in one
// transaction. If scoring fails the poll remains open and can be retried.
// The method is idempotent: calling it on an already-closed poll is a no-op.

// AddQuizScores computes complete-match scores outside a close transaction.
// Kept for standalone use (e.g. recomputation); prefer CloseAndScorePoll
// when closing a poll atomically.

// GetRanking returns top N participants sorted by total score descending.

type EmbedToken struct {
	Token          string    `json:"token"`
	EventID        uuid.UUID `json:"event_id"`
	AllowedOrigins []string  `json:"allowed_origins"`
	CreatedAt      time.Time `json:"created_at"`
}

type VisitorVote struct {
	PollID          uuid.UUID `json:"poll_id"`
	PollTitle       string    `json:"poll_title"`
	PollStatus      string    `json:"poll_status"`
	OptionID        uuid.UUID `json:"option_id"`
	OptionLabel     string    `json:"option_label"`
	OptionIsCorrect bool      `json:"option_is_correct"`
	OptionIsQuiz    bool      `json:"is_quiz"`
	CreatedAt       time.Time `json:"created_at"`
}

// GetPollsForExport returns polls with vote counts for CSV export.
type PollExportRow struct {
	PollTitle   string
	PollType    string
	OptionLabel string
	OptionOrder int
	IsCorrect   bool
	VoteCount   int64
	TotalVoters int64
}

// --- Moderator Methods ---

// Q&A Functions
