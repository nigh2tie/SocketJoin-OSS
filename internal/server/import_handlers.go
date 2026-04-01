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
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nigh2tie/SocketJoin-OSS/internal/service"
)

func (s *Server) handleCSVTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="poll_template.csv"`)

	cw := csv.NewWriter(w)
	cw.Write([]string{"# SocketJoin CSV import template"})
	cw.Write([]string{"# poll_type: survey または quiz"})
	cw.Write([]string{"# max_selections: 同時に選べる最大数。未指定時は 1"})
	cw.Write([]string{"# correct_options: quiz のみ必須。option_1 を 1 として、複数正解は 1|3 のように指定"})
	cw.Write([]string{"# option は左から詰めて記入し、使わない列は空欄のままにしてください"})
	cw.Write([]string{"poll_title", "poll_type", "max_selections", "option_1", "option_2", "option_3", "option_4", "option_5", "option_6", "option_7", "option_8", "points", "correct_options"})
	cw.Write([]string{"好きな色は？", "survey", "1", "赤", "青", "緑", "", "", "", "", "", "", ""})
	cw.Write([]string{"2 + 2 は？", "quiz", "1", "3", "4", "5", "", "", "", "", "", "10", "2"})
	cw.Write([]string{"正しいものをすべて選べ", "quiz", "2", "地球は丸い", "空は赤い", "水は液体", "", "", "", "", "", "20", "1|3"})
	cw.Flush()
}

type ImportResult struct {
	Success int                   `json:"success"`
	Failed  int                   `json:"failed"`
	Errors  []service.ImportError `json:"errors,omitempty"`
	DryRun  bool                  `json:"dry_run"`
}

func (s *Server) handleImportCSV(w http.ResponseWriter, r *http.Request) {
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
	if !s.authenticateEventOwner(w, r, event) {
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		s.jsonError(w, r, "failed to parse form", http.StatusBadRequest)
		return
	}

	dryRun := r.FormValue("dry_run") == "true"

	file, _, err := r.FormFile("file")
	if err != nil {
		s.jsonError(w, r, "file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(file)
	parsed, importErrors, fatalErr := s.PollService.ParseCSVImport(buf.Bytes())
	if fatalErr != nil {
		s.jsonError(w, r, fatalErr.Error(), http.StatusBadRequest)
		return
	}

	result := ImportResult{
		DryRun:  dryRun,
		Failed:  len(importErrors),
		Errors:  importErrors,
		Success: len(parsed),
	}

	if !dryRun && len(importErrors) == 0 && len(parsed) > 0 {
		if err := s.PollService.BulkCreatePolls(r.Context(), eventID, parsed); err != nil {
			slog.ErrorContext(r.Context(), "CSV Import: failed to insert polls", "error", err)
			s.jsonError(w, r, "failed to insert polls", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
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
	if !s.authenticateEventOwner(w, r, event) {
		return
	}

	rows, err := s.Pg.GetPollsForExport(r.Context(), eventID)
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to get polls for export", "error", err)
		s.jsonError(w, r, "failed to export polls", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="results.csv"`)

	cw := csv.NewWriter(w)
	cw.Write([]string{"poll_title", "poll_type", "option_label", "is_correct", "vote_count", "total_voters", "vote_rate(%)"})

	for _, row := range rows {
		pollType := "survey"
		if row.PollType == "quiz" {
			pollType = "quiz"
		}
		isCorrect := ""
		if row.IsCorrect {
			isCorrect = "TRUE"
		}
		rate := ""
		if row.TotalVoters > 0 {
			rate = fmt.Sprintf("%.1f", float64(row.VoteCount)/float64(row.TotalVoters)*100)
		}
		cw.Write([]string{
			row.PollTitle,
			pollType,
			row.OptionLabel,
			isCorrect,
			strconv.FormatInt(row.VoteCount, 10),
			strconv.FormatInt(row.TotalVoters, 10),
			rate,
		})
	}
	cw.Flush()
}
