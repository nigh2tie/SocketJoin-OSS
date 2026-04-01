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
	"golang.org/x/crypto/bcrypt"
)

func (s *Store) CreateEvent(ctx context.Context, title string, nicknamePolicy string) (*Event, error) {
	eventID := uuid.New()
	ownerToken := uuid.New().String()

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(ownerToken), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	ownerTokenHash := string(hashBytes)

	event := &Event{
		ID:             eventID,
		Title:          title,
		OwnerToken:     ownerToken, // Return unhashed to user
		Status:         "live",
		NicknamePolicy: nicknamePolicy,
		ShowQAOnScreen: true,
		CreatedAt:      time.Now(),
	}

	_, err = s.db.ExecContext(ctx, "INSERT INTO events (id, title, owner_token, status, nickname_policy, show_qa_on_screen, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		event.ID, event.Title, ownerTokenHash, event.Status, event.NicknamePolicy, event.ShowQAOnScreen, event.CreatedAt)
	if err != nil {
		return nil, err
	}
	return event, nil
}

func (s *Store) GetEvent(ctx context.Context, id uuid.UUID) (*Event, error) {
	event := &Event{}
	err := s.db.QueryRowContext(ctx, "SELECT id, title, owner_token, status, current_poll_id, nickname_policy, show_qa_on_screen, created_at FROM events WHERE id = $1", id).
		Scan(&event.ID, &event.Title, &event.OwnerToken, &event.Status, &event.CurrentPollID, &event.NicknamePolicy, &event.ShowQAOnScreen, &event.CreatedAt)
	if err != nil {
		return nil, err
	}
	return event, nil
}

func (s *Store) UpdateEventCurrentPoll(ctx context.Context, eventID uuid.UUID, pollID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, "UPDATE events SET current_poll_id = $1 WHERE id = $2", pollID, eventID)
	return err
}

func (s *Store) UpdateEventTitle(ctx context.Context, eventID uuid.UUID, title string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE events SET title = $1 WHERE id = $2", title, eventID)
	return err
}

func (s *Store) UpdateEventShowQAOnScreen(ctx context.Context, eventID uuid.UUID, show bool) error {
	_, err := s.db.ExecContext(ctx, "UPDATE events SET show_qa_on_screen = $1 WHERE id = $2", show, eventID)
	return err
}

func (s *Store) DeleteEvent(ctx context.Context, eventID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM events WHERE id = $1", eventID)
	return err
}
