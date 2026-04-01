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

package server

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/nigh2tie/SocketJoin-OSS/internal/store"
	"golang.org/x/crypto/bcrypt"
)

func ownerCookieName(eventID uuid.UUID) string {
	return "owner_" + eventID.String()
}

func moderatorCookieName(eventID uuid.UUID) string {
	return "moderator_" + eventID.String()
}

func (s *Server) isEventOwner(r *http.Request, event *store.Event) bool {
	cookie, err := r.Cookie(ownerCookieName(event.ID))
	if err != nil {
		return false
	}
	if err := bcrypt.CompareHashAndPassword([]byte(event.OwnerToken), []byte(cookie.Value)); err != nil {
		return false
	}
	return true
}

func (s *Server) isEventModerator(r *http.Request, eventID uuid.UUID) bool {
	cookie, err := r.Cookie(moderatorCookieName(eventID))
	if err != nil || cookie.Value == "" {
		return false
	}
	mod, err := s.Pg.AuthenticateModeratorByToken(r.Context(), cookie.Value)
	if err != nil {
		return false
	}
	return mod.EventID == eventID
}

func (s *Server) authenticateEventOwner(w http.ResponseWriter, r *http.Request, event *store.Event) bool {
	if !s.isEventOwner(r, event) {
		s.jsonError(w, r, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func (s *Server) authenticateEventOwnerOrModerator(w http.ResponseWriter, r *http.Request, event *store.Event) bool {
	if s.isEventOwner(r, event) || s.isEventModerator(r, event.ID) {
		return true
	}
	s.jsonError(w, r, "unauthorized", http.StatusUnauthorized)
	return false
}

func (s *Server) checkBan(ctx context.Context, eventID uuid.UUID, visitorID string) (bool, error) {
	return s.Redis.IsBanned(ctx, eventID, visitorID)
}
