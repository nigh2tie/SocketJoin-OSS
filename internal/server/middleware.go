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
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-chi/cors"
	"github.com/google/uuid"
)

func (s *Server) securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if !strings.HasPrefix(r.URL.Path, "/embed/") {
			w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) corsMiddleware() func(http.Handler) http.Handler {
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	return cors.Handler(cors.Options{
		AllowOriginFunc: func(r *http.Request, origin string) bool {
			originURL, err := url.Parse(origin)
			if err == nil && sameOriginHost(originURL.Host, r.Host) {
				return true
			}
			if err == nil {
				if allowedURL, err := url.Parse(frontendURL); err == nil && sameOriginHost(originURL.Host, allowedURL.Host) {
					return true
				}
			}
			return strings.EqualFold(origin, frontendURL)
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}

func sameOriginHost(a, b string) bool {
	hostA, portA := normalizeHostPort(a)
	hostB, portB := normalizeHostPort(b)
	return portA == portB && hostA == hostB
}

func normalizeHostPort(raw string) (string, string) {
	if raw == "" {
		return "", ""
	}

	host := raw
	port := ""
	if parsed, err := url.Parse("//" + raw); err == nil {
		host = parsed.Hostname()
		port = parsed.Port()
	}

	if host == "localhost" {
		host = "127.0.0.1"
	} else if addr, err := netip.ParseAddr(host); err == nil && addr.IsLoopback() {
		host = "127.0.0.1"
	}

	return strings.ToLower(host), port
}

func (s *Server) rateLimitMiddleware(next http.Handler) http.Handler {
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
}

func (s *Server) visitorIDMiddleware(next http.Handler) http.Handler {
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
}
