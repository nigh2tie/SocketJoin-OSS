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

type VoteRequest struct {
	OptionID  string   `json:"option_id"`
	OptionIDs []string `json:"option_ids"`
	Nickname  string   `json:"nickname"`
}

func (s *Server) handleVote(w http.ResponseWriter, r *http.Request) {
	pollIDStr := chi.URLParam(r, "id")
	pollID, err := uuid.Parse(pollIDStr)
	if err != nil {
		s.jsonError(w, r, "invalid poll id", http.StatusBadRequest)
		return
	}

	var req VoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, r, "invalid body", http.StatusBadRequest)
		return
	}

	rawIDs := req.OptionIDs
	if len(rawIDs) == 0 && req.OptionID != "" {
		rawIDs = []string{req.OptionID}
	}
	if len(rawIDs) == 0 {
		s.jsonError(w, r, "option_ids is required", http.StatusBadRequest)
		return
	}

	optionIDs := make([]uuid.UUID, 0, len(rawIDs))
	for _, raw := range rawIDs {
		oid, err := uuid.Parse(raw)
		if err != nil {
			s.jsonError(w, r, "invalid option id: "+raw, http.StatusBadRequest)
			return
		}
		optionIDs = append(optionIDs, oid)
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

	// Delegate business logic and orchestration to Service
	err = s.PollService.Vote(r.Context(), pollID, visitorID, optionIDs, req.Nickname)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPollNotFound), errors.Is(err, service.ErrEventNotFound):
			s.jsonError(w, r, err.Error(), http.StatusNotFound)
		case errors.Is(err, service.ErrInvalidOptions), errors.Is(err, service.ErrTooManyOptions),
			errors.Is(err, service.ErrNicknameRequired), errors.Is(err, service.ErrNGWord):
			s.jsonError(w, r, err.Error(), http.StatusBadRequest)
		case errors.Is(err, service.ErrBanned):
			s.jsonError(w, r, err.Error(), http.StatusForbidden)
		case errors.Is(err, store.ErrPollNotOpen), errors.Is(err, store.ErrAlreadyVoted):
			s.jsonError(w, r, err.Error(), http.StatusConflict)
		default:
			slog.ErrorContext(r.Context(), "PollService.Vote failed", "error", err)
			s.jsonError(w, r, "failed to submit vote", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleGetResult(w http.ResponseWriter, r *http.Request) {
	pollIDStr := chi.URLParam(r, "id")
	pollID, err := uuid.Parse(pollIDStr)
	if err != nil {
		s.jsonError(w, r, "invalid poll id", http.StatusBadRequest)
		return
	}

	counts, err := s.Redis.GetVoteCounts(r.Context(), pollID)
	if err != nil || len(counts) == 0 {
		counts, err = s.Pg.GetVoteCountsFromDB(r.Context(), pollID)
		if err != nil {
			slog.ErrorContext(r.Context(), "Failed to get vote counts", "error", err)
			s.jsonError(w, r, "failed to fetch results", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(counts)
}
