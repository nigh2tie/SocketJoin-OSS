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
	"crypto/subtle"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nigh2tie/SocketJoin-OSS/internal/hub"
	"github.com/nigh2tie/SocketJoin-OSS/internal/service"
	"github.com/nigh2tie/SocketJoin-OSS/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Server struct {
	Router      *chi.Mux
	Pg          *store.Store
	Redis       *store.RedisStore
	Hub         *hub.Hub
	PollService *service.PollService
}

const (
	csrfCookieName = "csrf_token"
	csrfHeaderName = "X-CSRF-Token"
)

// defaultNGWords は NG_WORDS 環境変数が未設定でも常に適用される基本禁止ワードリスト。
// NG_WORDS 環境変数でこのリストを拡張できる。
var defaultNGWords = []string{"ngword", "badword", "spam"}

func NewServer(pg *store.Store, redis *store.RedisStore, h *hub.Hub) *Server {
	ngWords := append([]string{}, defaultNGWords...)
	if raw := os.Getenv("NG_WORDS"); raw != "" {
		for _, w := range strings.Split(raw, ",") {
			if trimmed := strings.TrimSpace(strings.ToLower(w)); trimmed != "" {
				ngWords = append(ngWords, trimmed)
			}
		}
	}
	slog.Info("NG word check active",
		"total", len(ngWords),
		"built_in", len(defaultNGWords),
		"extra", len(ngWords)-len(defaultNGWords))

	pollSvc := service.NewPollService(pg, redis, ngWords)

	s := &Server{
		Router:      chi.NewRouter(),
		Pg:          pg,
		Redis:       redis,
		Hub:         h,
		PollService: pollSvc,
	}
	s.routes()
	go s.listenToRedis()
	return s
}

func (s *Server) listenToRedis() {
	backoff := 500 * time.Millisecond
	maxBackoff := 30 * time.Second

	for {
		slog.Info("Subscribing to Redis events...")
		pubsub := s.Redis.SubscribeToEvents(context.Background())
		
		ch := pubsub.Channel()
		// If SubscribeToEvents fails or returns a closed channel, 
		// we need to check if it's actually working.
		
		func() {
			defer pubsub.Close()
			for msg := range ch {
				// Reset backoff on successful message
				backoff = 500 * time.Millisecond
				
				// Channel name format: "event:{eventID}:channel"
				parts := strings.Split(msg.Channel, ":")
				if len(parts) >= 2 {
					eventID := parts[1]
					s.Hub.BroadcastToRoom(eventID, []byte(msg.Payload))
				}
			}
		}()

		slog.Error("Redis PubSub connection lost, reconnecting...", "retry_after", backoff)
		time.Sleep(backoff)
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (s *Server) routes() {
	s.Router.Use(middleware.RequestID)
	s.Router.Use(middleware.Logger)
	s.Router.Use(middleware.Recoverer)
	s.Router.Use(middleware.RealIP)

	// Security headers
	s.Router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			if !strings.HasPrefix(r.URL.Path, "/embed/") {
				w.Header().Set("X-Frame-Options", "SAMEORIGIN")
			}
			next.ServeHTTP(w, r)
		})
	})

	// CORS
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	s.Router.Use(cors.Handler(cors.Options{
		AllowOriginFunc: func(r *http.Request, origin string) bool {
			// Parse origin and compare host exactly to prevent suffix-bypass
			// (e.g. evil-example.com matching example.com via HasSuffix).
			parsed, err := url.Parse(origin)
			if err == nil && parsed.Host == r.Host {
				return true
			}
			// Allow explicit frontend URL
			return strings.EqualFold(origin, frontendURL)
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Rate Limit (Simple per-IP)
	s.Router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}

			if r.URL.Path == "/health" {
				next.ServeHTTP(w, r)
				return
			}

			allowed, err := s.Redis.RateLimit(r.Context(), "ratelimit:"+ip, 100, time.Minute)
			if err != nil {
				slog.ErrorContext(r.Context(), "RateLimit error", "error", err)
				s.jsonError(w, r, "service unavailable", http.StatusServiceUnavailable)
				return
			}
			if !allowed {
				s.jsonError(w, r, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// Cookie middleware for visitor_id
	s.Router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("visitor_id")
			if err != nil || cookie.Value == "" {
				uid := uuid.New().String()
				secureCookie := strings.EqualFold(os.Getenv("APP_ENV"), "production")
				http.SetCookie(w, &http.Cookie{
					Name:     "visitor_id",
					Value:    uid,
					Path:     "/",
					HttpOnly: true,
					Secure:   secureCookie,
					SameSite: http.SameSiteLaxMode,
					Expires:  time.Now().Add(365 * 24 * time.Hour),
				})
			}
			next.ServeHTTP(w, r)
		})
	})

	// CSRF for non-API unsafe methods
	s.Router.Use(s.csrfMiddleware)

	// Health checks
	s.Router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	s.Router.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		// Check Postgres
		if err := s.Pg.Ping(r.Context()); err != nil {
			slog.ErrorContext(r.Context(), "Readiness check failed: DB", "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "database unavailable"})
			return
		}

		// Check Redis
		if err := s.Redis.Ping(r.Context()); err != nil {
			slog.ErrorContext(r.Context(), "Readiness check failed: Redis", "error", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "redis unavailable"})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

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

	// Serve Static Files (SPA)
	workDir, _ := os.Getwd()
	buildDir := filepath.Join(workDir, "view/build")

	s.Router.Get("/embed/join/{id}", func(w http.ResponseWriter, r *http.Request) {
		eventIDStr := chi.URLParam(r, "id")
		tokenStr := r.URL.Query().Get("token")
		
		cspOrigins := "'none'"
		if tokenStr != "" {
			if token, err := s.Pg.GetEmbedToken(r.Context(), tokenStr); err == nil {
				if token.EventID.String() == eventIDStr {
					var origins []string
					allowsAll := false
					for _, allow := range token.AllowedOrigins {
						if allow == "*" {
							allowsAll = true
							break
						}
						origins = append(origins, allow)
					}
					if allowsAll {
						cspOrigins = "*"
					} else if len(origins) > 0 {
						cspOrigins = strings.Join(origins, " ")
					}
				}
			}
		}
		if cspOrigins != "*" {
			w.Header().Set("Content-Security-Policy", "frame-ancestors "+cspOrigins)
		}
		
		// Proxy index.html from frontend server if available
		frontURL := os.Getenv("INTERNAL_FRONTEND_URL")
		if frontURL != "" {
			resp, err := http.Get(frontURL + "/index.html")
			if err == nil {
				defer resp.Body.Close()
				w.Header().Set("Content-Type", "text/html")
				io.Copy(w, resp.Body)
				return
			}
		}
		
		// Fallback to local disk if running standalone
		http.ServeFile(w, r, filepath.Join(buildDir, "index.html"))
	})

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

// Event and Ban handlers moved to event_handlers.go

// Poll handlers moved to poll_handlers.go

// Vote handlers moved to poll_handlers.go

// CSV and Embed handlers moved to poll_handlers.go and qa_handlers.go

// --- Helpers ---

func (s *Server) jsonError(w http.ResponseWriter, r *http.Request, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func (s *Server) broadcastEventUpdate(eventID string, msgType string, payload interface{}) {
	msg, err := json.Marshal(map[string]interface{}{
		"type":    msgType,
		"payload": payload,
	})
	if err != nil {
		slog.Error("Failed to marshal event update", "type", msgType, "error", err)
		return
	}
	if err := s.Redis.PublishEventMessage(context.Background(), eventID, msg); err != nil {
		slog.Error("Failed to publish event update to Redis", "event_id", eventID, "type", msgType, "error", err)
	}
}

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

// Ban handlers moved to event_handlers.go

func (s *Server) csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := s.ensureCSRFToken(w, r)
		if !isUnsafeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/ws/") {
			next.ServeHTTP(w, r)
			return
		}

		requestToken := r.Header.Get(csrfHeaderName)
		if requestToken == "" {
			requestToken = r.FormValue("_csrf")
		}

		if requestToken == "" || subtle.ConstantTimeCompare([]byte(requestToken), []byte(token)) != 1 {
			s.jsonError(w, r, "invalid csrf token", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) ensureCSRFToken(w http.ResponseWriter, r *http.Request) string {
	if cookie, err := r.Cookie(csrfCookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	token := uuid.NewString()
	secureCookie := strings.EqualFold(os.Getenv("APP_ENV"), "production")
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		Secure:   secureCookie,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(24 * time.Hour),
	})
	return token
}

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// Moderator and Q&A handlers moved to qa_handlers.go
