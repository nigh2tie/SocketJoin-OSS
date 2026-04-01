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
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *Store) CreateVote(ctx context.Context, pollID uuid.UUID, optionIDs []uuid.UUID, visitorID string, nickname string) error {
	req := voteInsertRequest{
		PollID:    pollID,
		OptionIDs: optionIDs,
		VisitorID: visitorID,
		Nickname:  nickname,
		Result:    make(chan error, 1),
	}

	select {
	case s.voteInsertCh <- req:
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-req.Result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Store) runVoteBatcher() {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("runVoteBatcher panic recovered", "panic", r)
			go s.runVoteBatcher()
		}
	}()

	const maxBatchSize = 200
	const maxWait = 10 * time.Millisecond

	for {
		first, ok := <-s.voteInsertCh
		if !ok {
			return
		}
		batch := []voteInsertRequest{first}

		timer := time.NewTimer(maxWait)
	collect:
		for len(batch) < maxBatchSize {
			select {
			case req := <-s.voteInsertCh:
				batch = append(batch, req)
			case <-timer.C:
				break collect
			}
		}

		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}

		s.processVoteBatch(batch)
	}
}

func (s *Store) processVoteBatch(batch []voteInsertRequest) {
	if len(batch) == 0 {
		return
	}

	// Dedup by (poll_id, visitor_id) within this batch
	seen := make(map[voteIdentity]struct{}, len(batch))
	unique := make([]voteInsertRequest, 0, len(batch))
	for _, req := range batch {
		key := voteIdentity{PollID: req.PollID, VisitorID: req.VisitorID}
		if _, exists := seen[key]; exists {
			req.Result <- ErrAlreadyVoted
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, req)
	}
	if len(unique) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	statusByPoll, err := s.fetchPollStatuses(ctx, unique)
	if err != nil {
		for _, req := range unique {
			req.Result <- err
		}
		return
	}

	valid := make([]voteInsertRequest, 0, len(unique))
	for _, req := range unique {
		status, exists := statusByPoll[req.PollID]
		if !exists {
			req.Result <- ErrPollNotFound
			continue
		}
		if status != "open" {
			req.Result <- ErrPollNotOpen
			continue
		}
		valid = append(valid, req)
	}
	if len(valid) == 0 {
		return
	}

	// Insert submissions and votes atomically in one transaction.
	// If votes insert fails, the transaction rolls back so vote_submissions
	// is also rolled back, leaving the voter free to retry.
	submitted, err := s.bulkInsertSubmissionsAndVotes(ctx, valid)
	if err != nil {
		for _, req := range valid {
			req.Result <- err
		}
		return
	}

	for _, req := range valid {
		key := voteIdentity{PollID: req.PollID, VisitorID: req.VisitorID}
		if _, ok := submitted[key]; ok {
			req.Result <- nil
		} else {
			req.Result <- ErrAlreadyVoted
		}
	}
}

func (s *Store) fetchPollStatuses(ctx context.Context, batch []voteInsertRequest) (map[uuid.UUID]string, error) {
	ids := make([]uuid.UUID, 0, len(batch))
	seen := make(map[uuid.UUID]struct{}, len(batch))
	for _, req := range batch {
		if _, ok := seen[req.PollID]; ok {
			continue
		}
		seen[req.PollID] = struct{}{}
		ids = append(ids, req.PollID)
	}
	if len(ids) == 0 {
		return map[uuid.UUID]string{}, nil
	}

	args := make([]interface{}, 0, len(ids))
	placeholders := make([]string, 0, len(ids))
	for i, id := range ids {
		args = append(args, id)
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+1))
	}

	query := fmt.Sprintf(
		"SELECT id, status FROM polls WHERE id IN (%s)",
		strings.Join(placeholders, ", "),
	)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	statusByPoll := make(map[uuid.UUID]string, len(ids))
	for rows.Next() {
		var id uuid.UUID
		var status string
		if err := rows.Scan(&id, &status); err != nil {
			return nil, err
		}
		statusByPoll[id] = status
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return statusByPoll, nil
}

func (s *Store) bulkInsertSubmissionsAndVotes(ctx context.Context, batch []voteInsertRequest) (map[voteIdentity]struct{}, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	inserted := make(map[voteIdentity]struct{}, len(batch))

	// 1. Insert submissions (detect duplicates)
	subStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO vote_submissions (poll_id, visitor_id)
		VALUES ($1, $2)
		ON CONFLICT (poll_id, visitor_id) DO NOTHING
		RETURNING poll_id
	`)
	if err != nil {
		return nil, err
	}
	defer subStmt.Close()

	for _, req := range batch {
		key := voteIdentity{PollID: req.PollID, VisitorID: req.VisitorID}
		// If duplicate in the same batch, skip
		if _, exists := inserted[key]; exists {
			continue
		}

		var returnedPoll uuid.UUID
		err := subStmt.QueryRowContext(ctx, req.PollID, req.VisitorID).Scan(&returnedPoll)
		if err == nil {
			// Successfully inserted
			inserted[key] = struct{}{}
		} else if err.Error() == "sql: no rows in result set" {
			// Conflict, DO NOTHING triggered (already voted previously)
			continue
		} else {
			return nil, err
		}
	}

	if len(inserted) == 0 {
		return inserted, tx.Commit()
	}

	// 2. Insert option votes only for newly submitted visitors
	voteStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO votes (poll_id, option_id, visitor_id, nickname)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT DO NOTHING
	`)
	if err != nil {
		return nil, err
	}
	defer voteStmt.Close()

	for _, req := range batch {
		key := voteIdentity{PollID: req.PollID, VisitorID: req.VisitorID}
		if _, ok := inserted[key]; !ok {
			continue
		}
		for _, optID := range req.OptionIDs {
			if _, err := voteStmt.ExecContext(ctx, req.PollID, optID, req.VisitorID, req.Nickname); err != nil {
				return nil, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return inserted, nil
}

func (s *Store) GetVoteCountsFromDB(ctx context.Context, pollID uuid.UUID) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT option_id, COUNT(*) FROM votes WHERE poll_id = $1 GROUP BY option_id`,
		pollID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int64)
	for rows.Next() {
		var optionID uuid.UUID
		var count int64
		if err := rows.Scan(&optionID, &count); err != nil {
			return nil, err
		}
		counts[optionID.String()] = count
	}
	return counts, rows.Err()
}

func (s *Store) GetVisitorVotes(ctx context.Context, eventID uuid.UUID, visitorID string) ([]VisitorVote, error) {
	query := `
		SELECT
			v.poll_id, p.title, p.status, p.is_quiz,
			v.option_id, o.label, o.is_correct, v.created_at
		FROM votes v
		JOIN polls p ON v.poll_id = p.id
		JOIN options o ON v.option_id = o.id
		WHERE p.event_id = $1 AND v.visitor_id = $2
		ORDER BY v.created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, eventID, visitorID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []VisitorVote
	for rows.Next() {
		var vv VisitorVote
		if err := rows.Scan(
			&vv.PollID, &vv.PollTitle, &vv.PollStatus, &vv.OptionIsQuiz,
			&vv.OptionID, &vv.OptionLabel, &vv.OptionIsCorrect, &vv.CreatedAt,
		); err != nil {
			return nil, err
		}

		if vv.PollStatus != "closed" && vv.OptionIsQuiz {
			vv.OptionIsCorrect = false
		}

		history = append(history, vv)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if history == nil {
		history = []VisitorVote{}
	}

	return history, nil
}
