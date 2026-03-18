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
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type CreateEventRequest struct {
	Title          string `json:"title"`
	NicknamePolicy string `json:"nickname_policy"`
}

func (s *Server) handleCreateEvent(w http.ResponseWriter, r *http.Request) {
	var req CreateEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, r, "invalid body", http.StatusBadRequest)
		return
	}

	policy := req.NicknamePolicy
	if policy != "hidden" && policy != "required" {
		policy = "optional"
	}

	event, err := s.Pg.CreateEvent(r.Context(), req.Title, policy)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to create event", "error", err)
		s.jsonError(w, r, "failed to create event", http.StatusInternalServerError)
		return
	}

	secureCookie := strings.EqualFold(os.Getenv("APP_ENV"), "production")
	http.SetCookie(w, &http.Cookie{
		Name:     ownerCookieName(event.ID),
		Value:    event.OwnerToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400 * 30,
	})

	event.OwnerToken = ""
	event.Role = "host"
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(event)
}

func (s *Server) handleGetEvent(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		s.jsonError(w, r, "invalid id", http.StatusBadRequest)
		return
	}

	event, err := s.Pg.GetEvent(r.Context(), id)
	if err != nil {
		s.jsonError(w, r, "event not found", http.StatusNotFound)
		return
	}

	if s.isEventOwner(r, event) {
		event.Role = "host"
	} else if s.isEventModerator(r, event.ID) {
		event.Role = "moderator"
	} else {
		event.Role = "visitor"
	}

	event.OwnerToken = ""
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(event)
}

type EditEventRequest struct {
	Title string `json:"title"`
}

func (s *Server) handleEditEvent(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	eventID, err := uuid.Parse(idStr)
	if err != nil {
		s.jsonError(w, r, "invalid event id", http.StatusBadRequest)
		return
	}

	event, err := s.Pg.GetEvent(r.Context(), eventID)
	if err != nil {
		s.jsonError(w, r, "event not found", http.StatusNotFound)
		return
	}
	if !s.authenticateEventOwner(w, r, event) {
		return
	}

	var req EditEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, r, "invalid body", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Title) == "" {
		s.jsonError(w, r, "title is required", http.StatusBadRequest)
		return
	}

	if err := s.Pg.UpdateEventTitle(r.Context(), eventID, req.Title); err != nil {
		slog.ErrorContext(r.Context(), "Failed to update event title", "error", err)
		s.jsonError(w, r, "failed to update event", http.StatusInternalServerError)
		return
	}

	s.broadcastEventUpdate(eventID.String(), "event.updated", map[string]interface{}{
		"title": req.Title,
	})

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleDeleteEvent(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	eventID, err := uuid.Parse(idStr)
	if err != nil {
		s.jsonError(w, r, "invalid event id", http.StatusBadRequest)
		return
	}

	event, err := s.Pg.GetEvent(r.Context(), eventID)
	if err != nil {
		s.jsonError(w, r, "event not found", http.StatusNotFound)
		return
	}
	if !s.authenticateEventOwner(w, r, event) {
		return
	}

	if err := s.Pg.DeleteEvent(r.Context(), eventID); err != nil {
		slog.ErrorContext(r.Context(), "Failed to delete event", "error", err)
		s.jsonError(w, r, "failed to delete event", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetHistory(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	eventID, err := uuid.Parse(idStr)
	if err != nil {
		s.jsonError(w, r, "invalid event id", http.StatusBadRequest)
		return
	}

	visitorID := ""
	cookie, err := r.Cookie("visitor_id")
	if err == nil {
		visitorID = cookie.Value
	}
	if visitorID == "" {
		s.jsonError(w, r, "no visitor id", http.StatusForbidden)
		return
	}

	banned, err := s.checkBan(r.Context(), eventID, visitorID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to check ban status", "error", err)
		s.jsonError(w, r, "internal error", http.StatusInternalServerError)
		return
	}
	if banned {
		s.jsonError(w, r, "You are banned", http.StatusForbidden)
		return
	}

	history, err := s.Pg.GetVisitorVotes(r.Context(), eventID, visitorID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to get history", "error", err)
		s.jsonError(w, r, "failed to get history", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

func (s *Server) handleGetRanking(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	eventID, err := uuid.Parse(idStr)
	if err != nil {
		s.jsonError(w, r, "invalid event id", http.StatusBadRequest)
		return
	}

	limit := 10
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if n, err := strconv.Atoi(lStr); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	ranking, err := s.Pg.GetRanking(r.Context(), eventID, limit)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to get ranking", "error", err)
		s.jsonError(w, r, "failed to get ranking", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ranking)
}

type BanRequest struct {
	DurationHours int `json:"duration_hours"`
}

func (s *Server) handleBanVisitor(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		s.jsonError(w, r, "invalid event id", http.StatusBadRequest)
		return
	}
	visitorID := chi.URLParam(r, "visitor_id")
	if visitorID == "" {
		s.jsonError(w, r, "visitor_id required", http.StatusBadRequest)
		return
	}

	event, err := s.Pg.GetEvent(r.Context(), eventID)
	if err != nil {
		s.jsonError(w, r, "event not found", http.StatusNotFound)
		return
	}
	if !s.authenticateEventOwner(w, r, event) {
		return
	}

	var req BanRequest
	if decErr := json.NewDecoder(r.Body).Decode(&req); decErr != nil && !errors.Is(decErr, io.EOF) {
		s.jsonError(w, r, "invalid JSON body", http.StatusBadRequest)
		return
	}
	ttl := time.Duration(req.DurationHours) * time.Hour
	if ttl <= 0 {
		ttl = 30 * 24 * time.Hour // default: 30 days
	}

	if err := s.Redis.AddBan(r.Context(), eventID, visitorID, ttl); err != nil {
		slog.ErrorContext(r.Context(), "Failed to ban visitor", "error", err)
		s.jsonError(w, r, "failed to ban visitor", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUnbanVisitor(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		s.jsonError(w, r, "invalid event id", http.StatusBadRequest)
		return
	}
	visitorID := chi.URLParam(r, "visitor_id")
	if visitorID == "" {
		s.jsonError(w, r, "visitor_id required", http.StatusBadRequest)
		return
	}

	event, err := s.Pg.GetEvent(r.Context(), eventID)
	if err != nil {
		s.jsonError(w, r, "event not found", http.StatusNotFound)
		return
	}
	if !s.authenticateEventOwner(w, r, event) {
		return
	}

	if err := s.Redis.RemoveBan(r.Context(), eventID, visitorID); err != nil {
		slog.ErrorContext(r.Context(), "Failed to unban visitor", "error", err)
		s.jsonError(w, r, "failed to unban visitor", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
