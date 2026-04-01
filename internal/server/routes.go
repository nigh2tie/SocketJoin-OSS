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
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nigh2tie/SocketJoin-OSS/internal/hub"
)

func (s *Server) routes() {
	s.Router.Use(middleware.RequestID)
	s.Router.Use(middleware.Logger)
	s.Router.Use(middleware.Recoverer)
	s.Router.Use(middleware.RealIP)

	// Custom middlewares
	s.Router.Use(s.securityHeadersMiddleware)
	s.Router.Use(s.corsMiddleware())
	s.Router.Use(s.rateLimitMiddleware)
	s.Router.Use(s.visitorIDMiddleware)
	s.Router.Use(s.csrfMiddleware)

	// Health checks
	s.Router.Get("/health", s.handleHealth)
	s.Router.Get("/ready", s.handleReady)

	// WS
	s.Router.Get("/ws/event/{eventID}", func(w http.ResponseWriter, r *http.Request) {
		eventID := chi.URLParam(r, "eventID")
		hub.ServeWs(s.Hub, w, r, eventID)
	})

	s.Router.Route("/api", func(r chi.Router) {
		r.Get("/csrf", func(w http.ResponseWriter, r *http.Request) {
			token := s.ensureCSRFToken(w, r)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"token": token})
		})

		// Events
		r.Post("/events", s.handleCreateEvent)
		r.Get("/events/{id}", s.handleGetEvent)
		r.Put("/events/{id}", s.handleEditEvent)
		r.Delete("/events/{id}", s.handleDeleteEvent)
		r.Put("/events/{id}/active_poll", s.handleSwitchPoll)
		r.Get("/events/{id}/history", s.handleGetHistory)
		r.Get("/events/{id}/ranking", s.handleGetRanking)

		// Visitor moderation
		r.Post("/events/{id}/visitors/{visitor_id}/ban", s.handleBanVisitor)
		r.Delete("/events/{id}/visitors/{visitor_id}/ban", s.handleUnbanVisitor)

		// Moderators
		r.Get("/events/{id}/moderators", s.handleGetModerators)
		r.Post("/events/{id}/moderators", s.handleCreateModerator)
		r.Delete("/events/{id}/moderators/{mod_id}", s.handleDeleteModerator)
		r.Post("/moderator/login", s.handleModeratorLogin)

		// Q&A
		r.Get("/events/{id}/questions", s.handleGetQuestions)
		r.Post("/events/{id}/questions", s.handleCreateQuestion)
		r.Post("/events/{id}/questions/{qid}/upvote", s.handleToggleUpvote)
		r.Patch("/events/{id}/questions/{qid}/status", s.handleUpdateQuestionStatus)

		// Polls
		r.Post("/events/{id}/polls", s.handleCreatePoll)
		r.Get("/events/{id}/polls", s.handleGetPolls)
		r.Put("/events/{id}/polls/{poll_id}/close", s.handleClosePoll)
		r.Post("/events/{id}/polls/{poll_id}/reset", s.handleResetPoll)

		// CSV
		r.Get("/events/{id}/polls/export", s.handleExportCSV)
		r.Post("/events/{id}/polls/import", s.handleImportCSV)
		r.Get("/polls/template.csv", s.handleCSVTemplate)

		// Single Poll
		r.Get("/poll/{id}", s.handleGetPoll)
		r.Post("/poll/{id}/vote", s.handleVote)
		r.Get("/poll/{id}/result", s.handleGetResult)

		// Embed
		r.Post("/events/{id}/embed_token", s.handleCreateEmbedToken)
		r.Get("/embed/verify/{id}", s.handleEmbedJoin)
	})

	// Embed Specific Delivery
	s.Router.Get("/embed/join/{id}", s.handleEmbedJoinPage)

	// Serve Static Files (SPA)
	s.setupStaticFiles()
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := s.Pg.Ping(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "database unavailable"})
		return
	}
	if err := s.Redis.Ping(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "redis unavailable"})
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) setupStaticFiles() {
	workDir, _ := os.Getwd()
	buildDir := filepath.Join(workDir, "view/build")

	if _, err := os.Stat(buildDir); !os.IsNotExist(err) {
		s.Router.Handle("/_app/*", http.StripPrefix("/", http.FileServer(http.Dir(buildDir))))
		s.Router.Handle("/robots.txt", http.FileServer(http.Dir(buildDir)))

		s.Router.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, filepath.Join(buildDir, "index.html"))
		})

		s.Router.NotFound(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api") {
				http.NotFound(w, r)
				return
			}
			http.ServeFile(w, r, filepath.Join(buildDir, "index.html"))
		})
	} else {
		s.Router.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("SocketJoin API (Frontend not built)"))
		})
	}
}
