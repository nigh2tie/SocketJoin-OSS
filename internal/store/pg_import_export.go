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
