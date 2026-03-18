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

package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nigh2tie/SocketJoin-OSS/internal/hub"
	"github.com/nigh2tie/SocketJoin-OSS/internal/server"
	"github.com/nigh2tie/SocketJoin-OSS/internal/store"
)

func main() {
	// Initialize structured logging (JSON for Cloud Logging compatibility)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://user:password@localhost:5432/socketjoin?sslmode=disable"
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
	}

	// Connect to Postgres
	pg, err := store.NewStore(dbURL)
	if err != nil {
		slog.Error("Failed to connect to postgres", "error", err)
		os.Exit(1)
	}

	// Connect to Redis
	rdb, err := store.NewRedisStore(redisURL)
	if err != nil {
		slog.Error("Failed to connect to redis", "error", err)
		os.Exit(1)
	}

	// Initialize Hub
	h := hub.NewHub()
	go h.Run()

	// Initialize Server
	srv := server.NewServer(pg, rdb, h)

	// Start Data Retention Policy (Background Job)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			acquired, err := rdb.AcquireCleanupLock(context.Background(), 23*time.Hour)
			if err != nil {
				slog.Error("Data retention lock error", "error", err)
				continue
			}
			if !acquired {
				slog.Info("Data retention cleanup skipped (another instance is running)")
				continue
			}
			slog.Info("Running data retention cleanup...")
			deleted, err := pg.DeleteOldPolls(context.Background(), 90)
			if err != nil {
				slog.Error("Data retention cleanup failed", "error", err)
			} else {
				slog.Info("Data retention cleanup completed", "deleted", deleted)
			}
		}
	}()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	httpSrv := &http.Server{
		Addr:    ":" + port,
		Handler: srv.Router,
	}

	// Graceful shutdown on SIGTERM / SIGINT
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go func() {
		slog.Info("Starting server", "port", port)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Shutdown error", "error", err)
	}
	slog.Info("Server stopped")
}
