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
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleEmbedJoinPage(w http.ResponseWriter, r *http.Request) {
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
	workDir, _ := os.Getwd()
	buildDir := filepath.Join(workDir, "view/build")
	http.ServeFile(w, r, filepath.Join(buildDir, "index.html"))
}
