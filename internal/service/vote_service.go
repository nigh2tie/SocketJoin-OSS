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
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
)

func (s *PollService) Vote(ctx context.Context, pollID uuid.UUID, visitorID string, optionIDs []uuid.UUID, nickname string) error {
	// 1. Fetch Poll (Business Check)
	poll, err := s.pg.GetPoll(ctx, pollID)
	if err != nil {
		return ErrPollNotFound
	}

	// 2. Validate max selections
	if len(optionIDs) > poll.MaxSelections {
		return fmt.Errorf("%w (max %d)", ErrTooManyOptions, poll.MaxSelections)
	}

	// 3. Validate option belonging
	optionSet := make(map[uuid.UUID]struct{}, len(poll.Options))
	for _, opt := range poll.Options {
		optionSet[opt.ID] = struct{}{}
	}
	for _, oid := range optionIDs {
		if _, ok := optionSet[oid]; !ok {
			return ErrInvalidOptions
		}
	}

	// 4. Fetch Event & Policy Apply
	event, err := s.pg.GetEvent(ctx, poll.EventID)
	if err != nil {
		return ErrEventNotFound
	}

	switch event.NicknamePolicy {
	case "required":
		if strings.TrimSpace(nickname) == "" {
			return ErrNicknameRequired
		}
	case "hidden":
		nickname = ""
	}

	// 5. NG word check
	if s.CheckNGWord(nickname) {
		return ErrNGWord
	}

	// 6. BAN check
	banned, err := s.redis.IsBanned(ctx, poll.EventID, visitorID)
	if err != nil {
		slog.ErrorContext(ctx, "Ban check failed", "error", err)
		return ErrInternal
	}
	if banned {
		return ErrBanned
	}

	// 7. Execute Vote (Write to Batcher)
	// Batcher is an infra optimization in Store, we call it via Store.CreateVote.
	err = s.pg.CreateVote(ctx, pollID, optionIDs, visitorID, nickname)
	if err != nil {
		// Pass-through store specific errors like ErrAlreadyVoted or ErrPollNotOpen
		return err
	}

	// 8. Redis Counter Update (Orchestration)
	for _, oid := range optionIDs {
		if err := s.redis.IncrementVote(ctx, pollID, oid); err != nil {
			slog.ErrorContext(ctx, "Redis counter update failed", "error", err, "poll_id", pollID)
			// Non-fatal, continue to broadcast
		}
	}

	// 9. Broadcast update (Orchestration)
	// Since broadcasting to WS is a global side effect, Service orchestrates it.
	s.broadcastEventUpdate(ctx, poll.EventID.String(), "poll.updated", pollID)

	return nil
}
