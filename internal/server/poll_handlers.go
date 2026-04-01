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
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nigh2tie/SocketJoin-OSS/internal/service"
	"github.com/nigh2tie/SocketJoin-OSS/internal/store"
)

type SwitchPollRequest struct {
	PollID string `json:"poll_id"`
}

func (s *Server) handleSwitchPoll(w http.ResponseWriter, r *http.Request) {
	eventIDStr := chi.URLParam(r, "id")
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		s.jsonError(w, r, "invalid event id", http.StatusBadRequest)
		return
	}

	var req SwitchPollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, r, "invalid body", http.StatusBadRequest)
		return
	}

	event, err := s.Pg.GetEvent(r.Context(), eventID)
	if err != nil {
		s.jsonError(w, r, "event not found", http.StatusNotFound)
		return
	}
	if !s.authenticateEventOwnerOrModerator(w, r, event) {
		return
	}

	pollID, err := uuid.Parse(req.PollID)
	if err != nil {
		s.jsonError(w, r, "invalid poll id", http.StatusBadRequest)
		return
	}

	targetPoll, err := s.Pg.GetPoll(r.Context(), pollID)
	if err != nil || targetPoll.EventID != eventID {
		s.jsonError(w, r, "poll not found in this event", http.StatusNotFound)
		return
	}

	if err := s.Pg.UpdateEventCurrentPoll(r.Context(), eventID, pollID); err != nil {
		slog.ErrorContext(r.Context(), "Failed to update current poll", "error", err)
		s.jsonError(w, r, "failed to switch poll", http.StatusInternalServerError)
		return
	}

	s.broadcastEventUpdate(eventID.String(), "event.updated", map[string]interface{}{
		"current_poll_id": pollID,
		"poll":            targetPoll,
	})

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleClosePoll(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		s.jsonError(w, r, "invalid event id", http.StatusBadRequest)
		return
	}
	pollID, err := uuid.Parse(chi.URLParam(r, "poll_id"))
	if err != nil {
		s.jsonError(w, r, "invalid poll id", http.StatusBadRequest)
		return
	}

	event, err := s.Pg.GetEvent(r.Context(), eventID)
	if err != nil {
		s.jsonError(w, r, "event not found", http.StatusNotFound)
		return
	}
	if !s.authenticateEventOwnerOrModerator(w, r, event) {
		return
	}

	if err := s.PollService.ClosePoll(r.Context(), eventID, pollID); err != nil {
		if errors.Is(err, service.ErrPollNotFound) {
			s.jsonError(w, r, err.Error(), http.StatusNotFound)
			return
		}
		slog.ErrorContext(r.Context(), "PollService.ClosePoll failed", "error", err)
		s.jsonError(w, r, "failed to close poll", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleResetPoll(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		s.jsonError(w, r, "invalid event id", http.StatusBadRequest)
		return
	}
	pollID, err := uuid.Parse(chi.URLParam(r, "poll_id"))
	if err != nil {
		s.jsonError(w, r, "invalid poll id", http.StatusBadRequest)
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

	if err := s.PollService.ResetPoll(r.Context(), eventID, pollID); err != nil {
		if errors.Is(err, service.ErrPollNotFound) {
			s.jsonError(w, r, err.Error(), http.StatusNotFound)
			return
		}
		slog.ErrorContext(r.Context(), "PollService.ResetPoll failed", "error", err)
		s.jsonError(w, r, "failed to reset poll", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleGetPolls(w http.ResponseWriter, r *http.Request) {
	eventIDStr := chi.URLParam(r, "id")
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		s.jsonError(w, r, "invalid event id", http.StatusBadRequest)
		return
	}

	event, err := s.Pg.GetEvent(r.Context(), eventID)
	if err != nil {
		s.jsonError(w, r, "event not found", http.StatusNotFound)
		return
	}
	if !s.authenticateEventOwnerOrModerator(w, r, event) {
		return
	}

	polls, err := s.Pg.GetPollsForEvent(r.Context(), eventID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to get polls", "error", err)
		s.jsonError(w, r, "failed to fetch polls", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(polls)
}

type CreatePollOptionRequest struct {
	Label     string `json:"label"`
	IsCorrect bool   `json:"is_correct"`
}

type CreatePollRequest struct {
	Title         string                    `json:"title"`
	IsQuiz        bool                      `json:"is_quiz"`
	Points        int                       `json:"points"`
	MaxSelections int                       `json:"max_selections"`
	Options       []CreatePollOptionRequest `json:"options"`
}

func (s *Server) handleCreatePoll(w http.ResponseWriter, r *http.Request) {
	eventIDStr := chi.URLParam(r, "id")
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		s.jsonError(w, r, "invalid event id", http.StatusBadRequest)
		return
	}

	var req CreatePollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, r, "invalid body", http.StatusBadRequest)
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

	storeOptions := make([]store.Option, len(req.Options))
	for i, o := range req.Options {
		storeOptions[i] = store.Option{
			Label:     o.Label,
			IsCorrect: o.IsCorrect,
		}
	}

	poll, err := s.PollService.CreatePoll(r.Context(), eventID, req.Title, req.IsQuiz, req.Points, req.MaxSelections, storeOptions)
	if err != nil {
		if errors.Is(err, service.ErrNGWord) || errors.Is(err, service.ErrPollNeedsTwoOptions) || errors.Is(err, service.ErrQuizNeedsCorrectOption) {
			s.jsonError(w, r, err.Error(), http.StatusBadRequest)
			return
		}
		slog.ErrorContext(r.Context(), "PollService.CreatePoll failed", "error", err)
		s.jsonError(w, r, "failed to create poll", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(poll)
}

func (s *Server) handleGetPoll(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		s.jsonError(w, r, "invalid id", http.StatusBadRequest)
		return
	}

	poll, err := s.Pg.GetPoll(r.Context(), id)
	if err != nil {
		s.jsonError(w, r, "poll not found", http.StatusNotFound)
		return
	}

	if poll.IsQuiz && poll.Status != "closed" {
		for i := range poll.Options {
			poll.Options[i].IsCorrect = false
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(poll)
}
