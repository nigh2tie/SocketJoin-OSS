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

package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/nigh2tie/SocketJoin-OSS/internal/store"
)

var (
	ErrPollNotFound          = errors.New("poll not found")
	ErrEventNotFound         = errors.New("event not found")
	ErrInvalidOptions        = errors.New("invalid options selected")
	ErrTooManyOptions        = errors.New("too many options selected")
	ErrNicknameRequired      = errors.New("nickname is required for this event")
	ErrNGWord                = errors.New("text contains prohibited words")
	ErrBanned                = errors.New("user is banned")
	ErrInternal              = errors.New("internal server error")
	ErrPollNeedsTwoOptions   = errors.New("poll must have at least 2 options")
	ErrQuizNeedsCorrectOption = errors.New("quiz must have at least one correct option")
)

type PollService struct {
	pg      *store.Store
	redis   *store.RedisStore
	ngWords []string
}

func NewPollService(pg *store.Store, redis *store.RedisStore, ngWords []string) *PollService {
	return &PollService{
		pg:      pg,
		redis:   redis,
		ngWords: ngWords,
	}
}

func (s *PollService) CheckNGWord(text string) bool {
	lower := strings.ToLower(text)
	for _, w := range s.ngWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

func (s *PollService) CreatePoll(ctx context.Context, eventID uuid.UUID, title string, isQuiz bool, points int, maxSelections int, options []store.Option) (*store.Poll, error) {
	if len(options) < 2 {
		return nil, ErrPollNeedsTwoOptions
	}

	if s.CheckNGWord(title) {
		return nil, ErrNGWord
	}
	for _, opt := range options {
		if s.CheckNGWord(opt.Label) {
			return nil, ErrNGWord
		}
	}

	if isQuiz {
		hasCorrect := false
		for _, o := range options {
			if o.IsCorrect {
				hasCorrect = true
				break
			}
		}
		if !hasCorrect {
			return nil, ErrQuizNeedsCorrectOption
		}
	}

	if maxSelections < 1 {
		maxSelections = 1
	}
	if maxSelections > len(options) {
		maxSelections = len(options)
	}

	return s.pg.CreatePoll(ctx, eventID, title, isQuiz, points, maxSelections, options)
}

func (s *PollService) ClosePoll(ctx context.Context, eventID, pollID uuid.UUID) error {
	poll, err := s.pg.GetPoll(ctx, pollID)
	if err != nil {
		return ErrPollNotFound
	}
	if poll.EventID != eventID {
		return ErrPollNotFound
	}

	if poll.Status == "open" && poll.IsQuiz && poll.Points > 0 {
		if err := s.pg.CloseAndScorePoll(ctx, pollID); err != nil {
			return err
		}
	} else {
		if err := s.pg.ClosePoll(ctx, pollID); err != nil {
			return err
		}
	}

	s.broadcastEventUpdate(ctx, eventID.String(), "poll.closed", pollID)
	return nil
}

func (s *PollService) ResetPoll(ctx context.Context, eventID, pollID uuid.UUID) error {
	poll, err := s.pg.GetPoll(ctx, pollID)
	if err != nil {
		return ErrPollNotFound
	}
	if poll.EventID != eventID {
		return ErrPollNotFound
	}

	if err := s.pg.ResetPollVotes(ctx, pollID); err != nil {
		return err
	}

	if err := s.redis.ResetPollVotes(ctx, pollID); err != nil {
		slog.ErrorContext(ctx, "Redis ResetPollVotes error", "error", err)
	}

	s.broadcastEventUpdate(ctx, eventID.String(), "poll.reset", pollID)
	return nil
}

func (s *PollService) broadcastEventUpdate(ctx context.Context, eventID string, msgType string, pollID uuid.UUID) {
	// Re-fetch or get from cache to send latest counts
	counts, err := s.redis.GetVoteCounts(ctx, pollID)
	if err != nil || len(counts) == 0 {
		counts, _ = s.pg.GetVoteCountsFromDB(ctx, pollID)
	}

	msg, err := json.Marshal(map[string]interface{}{
		"type": msgType,
		"payload": map[string]interface{}{
			"poll_id": pollID,
			"counts":  counts,
		},
	})
	if err != nil {
		return
	}

	if err := s.redis.PublishEventMessage(ctx, eventID, msg); err != nil {
		slog.ErrorContext(ctx, "Failed to publish WS update", "error", err)
	}
}
