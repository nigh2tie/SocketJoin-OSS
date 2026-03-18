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
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

func (s *Store) CreateEmbedToken(ctx context.Context, eventID uuid.UUID, allowedOrigins []string) (*EmbedToken, error) {
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"*"}
	}

	token := uuid.New().String()
	et := &EmbedToken{
		Token:          token,
		EventID:        eventID,
		AllowedOrigins: allowedOrigins,
		CreatedAt:      time.Now(),
	}

	_, err := s.db.ExecContext(ctx, "INSERT INTO embed_tokens (token, event_id, allowed_origins) VALUES ($1, $2, $3)",
		et.Token, et.EventID, et.AllowedOrigins)
	if err != nil {
		return nil, err
	}
	return et, nil
}

func (s *Store) GetEmbedToken(ctx context.Context, token string) (*EmbedToken, error) {
	et := &EmbedToken{}
	var allowedOriginsStr string

	err := s.db.QueryRowContext(ctx, "SELECT token, event_id, allowed_origins::text, created_at FROM embed_tokens WHERE token = $1", token).
		Scan(&et.Token, &et.EventID, &allowedOriginsStr, &et.CreatedAt)

	if err != nil {
		return nil, err
	}

	allowedOriginsStr = strings.TrimLeft(allowedOriginsStr, "{")
	allowedOriginsStr = strings.TrimRight(allowedOriginsStr, "}")
	if allowedOriginsStr != "" {
		parts := strings.Split(allowedOriginsStr, ",")
		for _, p := range parts {
			p = strings.Trim(p, "\"")
			et.AllowedOrigins = append(et.AllowedOrigins, p)
		}
	} else {
		et.AllowedOrigins = []string{}
	}

	return et, nil
}

func (s *Store) CreateModerator(ctx context.Context, eventID uuid.UUID, name string) (*Moderator, error) {
	modID := uuid.New()
	secret := uuid.New().String()

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	tokenHash := string(hashBytes)

	rawToken := modID.String() + "_" + secret

	mod := &Moderator{
		ID:        modID,
		EventID:   eventID,
		Name:      name,
		Token:     rawToken,
		CreatedAt: time.Now(),
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "SELECT id FROM events WHERE id = $1 FOR UPDATE", eventID); err != nil {
		return nil, err
	}

	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM event_moderators WHERE event_id = $1", eventID).Scan(&count); err != nil {
		return nil, err
	}
	if count >= 3 {
		return nil, ErrModeratorLimitReached
	}

	_, err = tx.ExecContext(ctx, "INSERT INTO event_moderators (id, event_id, name, token_hash, created_at) VALUES ($1, $2, $3, $4, $5)",
		mod.ID, mod.EventID, mod.Name, tokenHash, mod.CreatedAt)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return mod, nil
}

func (s *Store) GetModeratorsByEvent(ctx context.Context, eventID uuid.UUID) ([]Moderator, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, event_id, name, created_at FROM event_moderators WHERE event_id = $1 ORDER BY created_at ASC", eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mods []Moderator
	for rows.Next() {
		var m Moderator
		if err := rows.Scan(&m.ID, &m.EventID, &m.Name, &m.CreatedAt); err != nil {
			return nil, err
		}
		mods = append(mods, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if mods == nil {
		mods = []Moderator{}
	}
	return mods, nil
}

func (s *Store) DeleteModerator(ctx context.Context, modID uuid.UUID, eventID uuid.UUID) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM event_moderators WHERE id = $1 AND event_id = $2", modID, eventID)
	return err
}

func (s *Store) AuthenticateModeratorByToken(ctx context.Context, token string) (*Moderator, error) {
	parts := strings.SplitN(token, "_", 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid token format")
	}
	modID, err := uuid.Parse(parts[0])
	if err != nil {
		return nil, errors.New("invalid token format")
	}
	secret := parts[1]

	var m Moderator
	var hash string
	err = s.db.QueryRowContext(ctx, "SELECT id, event_id, name, token_hash, created_at FROM event_moderators WHERE id = $1", modID).
		Scan(&m.ID, &m.EventID, &m.Name, &hash, &m.CreatedAt)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret)); err != nil {
		return nil, errors.New("unauthorized")
	}

	return &m, nil
}
