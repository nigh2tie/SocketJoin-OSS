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
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func (s *Store) CreateQuestion(ctx context.Context, eventID uuid.UUID, visitorID, content string) (*Question, error) {
	q := &Question{
		ID:        uuid.New(),
		EventID:   eventID,
		VisitorID: visitorID,
		Content:   content,
		Status:    "active",
		CreatedAt: time.Now(),
		Upvotes:   0,
		IsUpvoted: false,
	}

	_, err := s.db.ExecContext(ctx, "INSERT INTO questions (id, event_id, visitor_id, content, status, created_at) VALUES ($1, $2, $3, $4, $5, $6)",
		q.ID, q.EventID, q.VisitorID, q.Content, q.Status, q.CreatedAt)
	if err != nil {
		return nil, err
	}
	return q, nil
}

func (s *Store) GetQuestionsByEvent(ctx context.Context, eventID uuid.UUID, visitorID string) ([]Question, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT q.id, q.event_id, '' AS visitor_id, q.content, q.status, q.created_at,
		       (SELECT COUNT(*) FROM question_upvotes qu WHERE qu.question_id = q.id) as upvotes,
		       EXISTS(SELECT 1 FROM question_upvotes qu WHERE qu.question_id = q.id AND qu.visitor_id = $1) as is_upvoted
		FROM questions q
		WHERE q.event_id = $2 AND q.status != 'archived'
		ORDER BY upvotes DESC, q.created_at ASC
	`, visitorID, eventID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var questions []Question
	for rows.Next() {
		var q Question
		if err := rows.Scan(&q.ID, &q.EventID, &q.VisitorID, &q.Content, &q.Status, &q.CreatedAt, &q.Upvotes, &q.IsUpvoted); err != nil {
			return nil, err
		}
		questions = append(questions, q)
	}
	return questions, nil
}

func (s *Store) ToggleQuestionUpvote(ctx context.Context, eventID uuid.UUID, questionID uuid.UUID, visitorID string) (bool, error) {
	// Returns true if an upvote was added, false if it was removed
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM questions WHERE id = $1 AND event_id = $2)", questionID, eventID).Scan(&exists); err != nil {
		return false, err
	}
	if !exists {
		return false, ErrQuestionNotFound
	}

	res, err := tx.ExecContext(ctx, "DELETE FROM question_upvotes WHERE question_id = $1 AND visitor_id = $2", questionID, visitorID)
	if err != nil {
		return false, err
	}
	deleted, _ := res.RowsAffected()
	if deleted > 0 {
		if err := tx.Commit(); err != nil {
			return false, err
		}
		return false, nil // Removed
	}

	_, err = tx.ExecContext(ctx, "INSERT INTO question_upvotes (question_id, visitor_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", questionID, visitorID)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil // Added
}

func (s *Store) UpdateQuestionStatus(ctx context.Context, eventID uuid.UUID, questionID uuid.UUID, status string) error {
	res, err := s.db.ExecContext(ctx, "UPDATE questions SET status = $1 WHERE id = $2 AND event_id = $3", status, questionID, eventID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrQuestionNotFound
	}
	return nil
}
