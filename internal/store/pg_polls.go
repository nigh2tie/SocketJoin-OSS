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
		first := <-s.voteInsertCh
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

func (s *Store) CloseAndScorePoll(ctx context.Context, pollID uuid.UUID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Only transition open → closed. If already closed, 0 rows are affected and
	// scoring is skipped, making repeated calls safe.
	res, err := tx.ExecContext(ctx, "UPDATE polls SET status = 'closed' WHERE id = $1 AND status = 'open'", pollID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		if _, err := tx.ExecContext(ctx, addQuizScoresSQL, pollID); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) AddQuizScores(ctx context.Context, pollID uuid.UUID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Get poll points
	var points int
	var eventID uuid.UUID
	err = tx.QueryRowContext(ctx, "SELECT event_id, points FROM polls WHERE id = $1", pollID).Scan(&eventID, &points)
	if err != nil {
		return err
	}

	if points <= 0 {
		return tx.Commit() // Nothing to add
	}

	// 2. Get correct options
	rows, err := tx.QueryContext(ctx, "SELECT id FROM options WHERE poll_id = $1 AND is_correct = TRUE ORDER BY id", pollID)
	if err != nil {
		return err
	}
	var correctIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		correctIDs = append(correctIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	// 3. Get all voter selections
	voterRows, err := tx.QueryContext(ctx, `
		SELECT visitor_id, MAX(nickname)
		FROM votes 
		WHERE poll_id = $1 
		GROUP BY visitor_id
	`, pollID)
	if err != nil {
		return err
	}
	
	type voter struct {
		id       string
		nickname string
	}
	var voters []voter
	for voterRows.Next() {
		var v voter
		if err := voterRows.Scan(&v.id, &v.nickname); err != nil {
			voterRows.Close()
			return err
		}
		if v.nickname != "" && v.nickname != "Anonymous" {
			voters = append(voters, v)
		}
	}
	voterRows.Close()
	if err := voterRows.Err(); err != nil {
		return err
	}

	// 4. For each voter, get their selected options and compare
	// To minimize queries, we can get all votes for the poll and group them
	allVotesRows, err := tx.QueryContext(ctx, "SELECT visitor_id, option_id FROM votes WHERE poll_id = $1 ORDER BY visitor_id, option_id", pollID)
	if err != nil {
		return err
	}
	
	selections := make(map[string][]uuid.UUID)
	for allVotesRows.Next() {
		var vID string
		var optID uuid.UUID
		if err := allVotesRows.Scan(&vID, &optID); err != nil {
			allVotesRows.Close()
			return err
		}
		selections[vID] = append(selections[vID], optID)
	}
	allVotesRows.Close()
	if err := allVotesRows.Err(); err != nil {
		return err
	}

	// 5. Update scores
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO participant_scores (event_id, visitor_id, nickname, total_score)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (event_id, visitor_id) DO UPDATE
		SET total_score = participant_scores.total_score + EXCLUDED.total_score,
		    nickname = CASE WHEN EXCLUDED.nickname != '' THEN EXCLUDED.nickname ELSE participant_scores.nickname END
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, v := range voters {
		sels := selections[v.id]
		if len(sels) != len(correctIDs) {
			continue
		}
		// Slices are ordered since we used ORDER BY in SQL
		match := true
		for i, id := range sels {
			if id != correctIDs[i] {
				match = false
				break
			}
		}
		
		if match {
			if _, err := stmt.ExecContext(ctx, eventID, v.id, v.nickname, points); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func (s *Store) GetRanking(ctx context.Context, eventID uuid.UUID, limit int) ([]RankingEntry, error) {
	if limit <= 0 {
		limit = 10
	}
	query := `
		SELECT
			RANK() OVER (ORDER BY total_score DESC) AS rank,
			nickname,
			total_score
		FROM participant_scores
		WHERE event_id = $1
		  AND nickname != ''
		  AND nickname != 'Anonymous'
		ORDER BY total_score DESC
		LIMIT $2
	`
	rows, err := s.db.QueryContext(ctx, query, eventID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []RankingEntry
	for rows.Next() {
		var e RankingEntry
		if err := rows.Scan(&e.Rank, &e.Nickname, &e.TotalScore); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []RankingEntry{}
	}
	return entries, nil
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

func (s *Store) DeleteOldPolls(ctx context.Context, retentionDays int) (int64, error) {
	query := `DELETE FROM polls WHERE created_at < NOW() - make_interval(days => $1)`
	res, err := s.db.ExecContext(ctx, query, retentionDays)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
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

func (s *Store) GetPollsForExport(ctx context.Context, eventID uuid.UUID) ([]PollExportRow, error) {
	query := `
		SELECT
			p.title,
			CASE WHEN p.is_quiz THEN 'quiz' ELSE 'survey' END AS poll_type,
			o.label,
			o.item_order,
			o.is_correct,
			COALESCE((SELECT COUNT(*) FROM votes v WHERE v.option_id = o.id), 0) AS vote_count,
			COALESCE((SELECT COUNT(DISTINCT vs.visitor_id) FROM vote_submissions vs WHERE vs.poll_id = p.id), 0) AS total_voters
		FROM polls p
		JOIN options o ON o.poll_id = p.id
		WHERE p.event_id = $1
		ORDER BY p.created_at ASC, o.item_order ASC
	`
	rows, err := s.db.QueryContext(ctx, query, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []PollExportRow
	for rows.Next() {
		var r PollExportRow
		if err := rows.Scan(&r.PollTitle, &r.PollType, &r.OptionLabel, &r.OptionOrder, &r.IsCorrect, &r.VoteCount, &r.TotalVoters); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
