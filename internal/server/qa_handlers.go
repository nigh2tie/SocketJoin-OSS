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
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nigh2tie/SocketJoin-OSS/internal/store"
)

func (s *Server) handleGetModerators(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
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

	mods, err := s.Pg.GetModeratorsByEvent(r.Context(), eventID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to get moderators", "error", err)
		s.jsonError(w, r, "failed to get moderators", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mods)
}

type CreateModeratorRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleCreateModerator(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
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
	var req CreateModeratorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, r, "invalid request", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		s.jsonError(w, r, "name is required", http.StatusBadRequest)
		return
	}

	mod, err := s.Pg.CreateModerator(r.Context(), eventID, req.Name)
	if err != nil {
		if errors.Is(err, store.ErrModeratorLimitReached) {
			s.jsonError(w, r, err.Error(), http.StatusBadRequest)
			return
		}
		slog.ErrorContext(r.Context(), "Failed to create moderator", "error", err)
		s.jsonError(w, r, "failed to create moderator", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mod)
}

func (s *Server) handleDeleteModerator(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		s.jsonError(w, r, "invalid event id", http.StatusBadRequest)
		return
	}
	modID, err := uuid.Parse(chi.URLParam(r, "mod_id"))
	if err != nil {
		s.jsonError(w, r, "invalid moderator id", http.StatusBadRequest)
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

	if err := s.Pg.DeleteModerator(r.Context(), modID, eventID); err != nil {
		slog.ErrorContext(r.Context(), "Failed to delete moderator", "error", err)
		s.jsonError(w, r, "failed to delete moderator", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

type ModeratorLoginRequest struct {
	Token string `json:"token"`
}

func (s *Server) handleModeratorLogin(w http.ResponseWriter, r *http.Request) {
	var req ModeratorLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, r, "invalid request", http.StatusBadRequest)
		return
	}

	mod, err := s.Pg.AuthenticateModeratorByToken(r.Context(), req.Token)
	if err != nil {
		s.jsonError(w, r, "unauthorized", http.StatusUnauthorized)
		return
	}

	secureCookie := strings.EqualFold(os.Getenv("APP_ENV"), "production")
	http.SetCookie(w, &http.Cookie{
		Name:     moderatorCookieName(mod.EventID),
		Value:    req.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400 * 30,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"event_id": mod.EventID.String()})
}

func (s *Server) handleGetQuestions(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		s.jsonError(w, r, "invalid event id", http.StatusBadRequest)
		return
	}

	visitorID := ""
	if cookie, err := r.Cookie("visitor_id"); err == nil {
		visitorID = cookie.Value
	}

	questions, err := s.Pg.GetQuestionsByEvent(r.Context(), eventID, visitorID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to get questions", "error", err)
		s.jsonError(w, r, "failed to get questions", http.StatusInternalServerError)
		return
	}

	if questions == nil {
		questions = []store.Question{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(questions)
}

type CreateQuestionRequest struct {
	Content string `json:"content"`
}

func (s *Server) handleCreateQuestion(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		s.jsonError(w, r, "invalid event id", http.StatusBadRequest)
		return
	}

	visitorID := ""
	if cookie, err := r.Cookie("visitor_id"); err == nil {
		visitorID = cookie.Value
	}
	if visitorID == "" {
		s.jsonError(w, r, "no visitor id", http.StatusForbidden)
		return
	}

	banned, err := s.checkBan(r.Context(), eventID, visitorID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to check ban status", "error", err)
		s.jsonError(w, r, "internal check error", http.StatusInternalServerError)
		return
	}
	if banned {
		s.jsonError(w, r, "banned", http.StatusForbidden)
		return
	}

	var req CreateQuestionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, r, "invalid request", http.StatusBadRequest)
		return
	}

	if s.PollService.CheckNGWord(req.Content) {
		s.jsonError(w, r, "content contains prohibited words", http.StatusBadRequest)
		return
	}

	q, err := s.Pg.CreateQuestion(r.Context(), eventID, visitorID, req.Content)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to create question", "error", err)
		s.jsonError(w, r, "failed to create question", http.StatusInternalServerError)
		return
	}

	q.VisitorID = ""
	s.broadcastEventUpdate(eventID.String(), "qa.question_created", q)

	w.WriteHeader(http.StatusCreated)
}

func (s *Server) handleToggleUpvote(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		s.jsonError(w, r, "invalid event id", http.StatusBadRequest)
		return
	}
	questionID, err := uuid.Parse(chi.URLParam(r, "qid"))
	if err != nil {
		s.jsonError(w, r, "invalid question id", http.StatusBadRequest)
		return
	}

	visitorID := ""
	if cookie, err := r.Cookie("visitor_id"); err == nil {
		visitorID = cookie.Value
	}
	if visitorID == "" {
		s.jsonError(w, r, "no visitor id", http.StatusForbidden)
		return
	}

	_, err = s.Pg.ToggleQuestionUpvote(r.Context(), eventID, questionID, visitorID)
	if err != nil {
		if errors.Is(err, store.ErrQuestionNotFound) {
			s.jsonError(w, r, "question not found in this event", http.StatusNotFound)
			return
		}
		slog.ErrorContext(r.Context(), "Failed to toggle upvote", "error", err)
		s.jsonError(w, r, "failed to toggle upvote", http.StatusInternalServerError)
		return
	}

	s.broadcastEventUpdate(eventID.String(), "qa.question_upvoted", map[string]interface{}{"question_id": questionID})

	w.WriteHeader(http.StatusOK)
}

type UpdateQuestionStatusRequest struct {
	Status string `json:"status"` // active, answered, archived
}

func (s *Server) handleUpdateQuestionStatus(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		s.jsonError(w, r, "invalid event id", http.StatusBadRequest)
		return
	}
	questionID, err := uuid.Parse(chi.URLParam(r, "qid"))
	if err != nil {
		s.jsonError(w, r, "invalid question id", http.StatusBadRequest)
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

	var req UpdateQuestionStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, r, "invalid request", http.StatusBadRequest)
		return
	}

	if req.Status != "active" && req.Status != "answered" && req.Status != "archived" {
		s.jsonError(w, r, "invalid status", http.StatusBadRequest)
		return
	}

	err = s.Pg.UpdateQuestionStatus(r.Context(), eventID, questionID, req.Status)
	if err != nil {
		if errors.Is(err, store.ErrQuestionNotFound) {
			s.jsonError(w, r, "question not found in this event", http.StatusNotFound)
			return
		}
		slog.ErrorContext(r.Context(), "Failed to update question status", "error", err)
		s.jsonError(w, r, "failed to update question status", http.StatusInternalServerError)
		return
	}

	s.broadcastEventUpdate(eventID.String(), "qa.question_status_updated", map[string]interface{}{
		"question_id": questionID,
		"status":      req.Status,
	})

	w.WriteHeader(http.StatusOK)
}

type CreateEmbedTokenRequest struct {
	AllowedOrigins []string `json:"allowed_origins"`
}

func (s *Server) handleCreateEmbedToken(w http.ResponseWriter, r *http.Request) {
	eventIDStr := chi.URLParam(r, "id")
	eventID, err := uuid.Parse(eventIDStr)
	if err != nil {
		s.jsonError(w, r, "invalid event id", http.StatusBadRequest)
		return
	}

	var req CreateEmbedTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, r, "invalid body", http.StatusBadRequest)
		return
	}

	eventForEmbed, err := s.Pg.GetEvent(r.Context(), eventID)
	if err != nil {
		s.jsonError(w, r, "event not found", http.StatusNotFound)
		return
	}
	if !s.authenticateEventOwner(w, r, eventForEmbed) {
		return
	}

	token, err := s.Pg.CreateEmbedToken(r.Context(), eventID, req.AllowedOrigins)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to create embed token", "error", err)
		s.jsonError(w, r, "failed to create token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(token)
}

func (s *Server) handleEmbedJoin(w http.ResponseWriter, r *http.Request) {
	eventIDStr := chi.URLParam(r, "id")
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		s.jsonError(w, r, "missing token", http.StatusForbidden)
		return
	}

	token, err := s.Pg.GetEmbedToken(r.Context(), tokenStr)
	if err != nil {
		s.jsonError(w, r, "invalid token", http.StatusForbidden)
		return
	}

	if token.EventID.String() != eventIDStr {
		s.jsonError(w, r, "token mismatch", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"allowed": true})
}
