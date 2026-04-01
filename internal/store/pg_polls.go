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

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func (s *Store) GetPollsForEvent(ctx context.Context, eventID uuid.UUID) ([]Poll, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, event_id, title, status, is_quiz, points, max_selections, created_at FROM polls WHERE event_id = $1 ORDER BY created_at ASC",
		eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var polls []Poll
	for rows.Next() {
		var p Poll
		if err := rows.Scan(&p.ID, &p.EventID, &p.Title, &p.Status, &p.IsQuiz, &p.Points, &p.MaxSelections, &p.CreatedAt); err != nil {
			return nil, err
		}
		polls = append(polls, p)
	}
	return polls, nil
}

func (s *Store) CreatePoll(ctx context.Context, eventID uuid.UUID, title string, isQuiz bool, points int, maxSelections int, options []Option) (*Poll, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	poll, err := insertPollTx(ctx, tx, eventID, title, isQuiz, points, maxSelections, options)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return poll, nil
}

func (s *Store) BulkCreatePolls(ctx context.Context, eventID uuid.UUID, inputs []BulkPollInput) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, inp := range inputs {
		if _, err := insertPollTx(ctx, tx, eventID, inp.Title, inp.IsQuiz, inp.Points, inp.MaxSelections, inp.Options); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) GetPoll(ctx context.Context, pollID uuid.UUID) (*Poll, error) {
	poll := &Poll{}
	err := s.db.QueryRowContext(ctx,
		"SELECT id, event_id, title, status, is_quiz, points, max_selections, created_at FROM polls WHERE id = $1", pollID).
		Scan(&poll.ID, &poll.EventID, &poll.Title, &poll.Status, &poll.IsQuiz, &poll.Points, &poll.MaxSelections, &poll.CreatedAt)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx,
		"SELECT id, poll_id, label, item_order, is_correct, created_at FROM options WHERE poll_id = $1 ORDER BY item_order", pollID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var opt Option
		if err := rows.Scan(&opt.ID, &opt.PollID, &opt.Label, &opt.Order, &opt.IsCorrect, &opt.CreatedAt); err != nil {
			return nil, err
		}
		poll.Options = append(poll.Options, opt)
	}

	return poll, nil
}

func (s *Store) ClosePoll(ctx context.Context, pollID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, "UPDATE polls SET status = 'closed' WHERE id = $1", pollID)
	return err
}

func (s *Store) ResetPollVotes(ctx context.Context, pollID uuid.UUID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM votes WHERE poll_id = $1", pollID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM vote_submissions WHERE poll_id = $1", pollID); err != nil {
		return err
	}
	// Reopen the poll
	if _, err := tx.ExecContext(ctx, "UPDATE polls SET status = 'open' WHERE id = $1", pollID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) DeleteOldPolls(ctx context.Context, retentionDays int) (int64, error) {
	query := `DELETE FROM polls WHERE created_at < NOW() - make_interval(days => $1)`
	res, err := s.db.ExecContext(ctx, query, retentionDays)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
