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
)

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
		if v.nickname == "" {
			v.nickname = "Anonymous"
		}
		voters = append(voters, v)
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
			COALESCE(NULLIF(nickname, ''), 'Anonymous') AS nickname,
			total_score
		FROM participant_scores
		WHERE event_id = $1
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
