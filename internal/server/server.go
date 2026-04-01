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
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/nigh2tie/SocketJoin-OSS/internal/hub"
	"github.com/nigh2tie/SocketJoin-OSS/internal/service"
	"github.com/nigh2tie/SocketJoin-OSS/internal/store"

	"github.com/go-chi/chi/v5"
)

type Server struct {
	Router      *chi.Mux
	Pg          *store.Store
	Redis       *store.RedisStore
	Hub         *hub.Hub
	PollService *service.PollService
}

// defaultNGWords は NG_WORDS 環境変数が未設定でも常に適用される基本禁止ワードリスト。
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
