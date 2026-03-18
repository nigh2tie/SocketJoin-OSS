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
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

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

func (s *Server) handleCSVTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="poll_template.csv"`)

	cw := csv.NewWriter(w)
	cw.Write([]string{"poll_title", "poll_type", "max_selections", "option_1", "option_2", "option_3", "option_4", "option_5", "option_6", "option_7", "option_8", "points", "correct_options"})
	cw.Write([]string{"好きな色は？", "survey", "1", "赤", "青", "緑", "", "", "", "", "", "", ""})
	cw.Write([]string{"日本の首都は？", "quiz", "1", "東京", "大阪", "京都", "", "", "", "", "", "10", "1"})
	cw.Write([]string{"正しいものをすべて選べ", "quiz", "2", "地球は丸い", "空は赤い", "水は液体", "", "", "", "", "", "20", "1|3"})
	cw.Flush()
}

type ImportError struct {
	Row    int    `json:"row"`
	Reason string `json:"reason"`
}

type ImportResult struct {
	Success int           `json:"success"`
	Failed  int           `json:"failed"`
	Errors  []ImportError `json:"errors,omitempty"`
	DryRun  bool          `json:"dry_run"`
}

const maxImportRows = 500

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
	parsed, importErrors, fatalErr := parseCSVImport(buf.Bytes())
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
		if err := s.Pg.BulkCreatePolls(r.Context(), eventID, parsed); err != nil {
			slog.ErrorContext(r.Context(), "CSV Import: failed to insert polls", "error", err)
			s.jsonError(w, r, "failed to insert polls", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func parseCSVImport(data []byte) ([]store.BulkPollInput, []ImportError, error) {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}

	reader := csv.NewReader(bytes.NewReader(data))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(records) < 2 {
		return nil, nil, fmt.Errorf("CSV must have a header row and at least one data row")
	}

	headers := records[0]
	colIdx := make(map[string]int)
	for i, h := range headers {
		colIdx[strings.TrimSpace(h)] = i
	}

	requiredCols := []string{"poll_title", "poll_type", "option_1", "option_2"}
	for _, col := range requiredCols {
		if _, ok := colIdx[col]; !ok {
			return nil, nil, fmt.Errorf("missing required column: %s", col)
		}
	}

	dataRows := records[1:]
	if len(dataRows) > maxImportRows {
		return nil, nil, fmt.Errorf("too many rows (max %d)", maxImportRows)
	}

	var parsed []store.BulkPollInput
	var importErrors []ImportError

	for i, row := range dataRows {
		rowNum := i + 2

		get := func(col string) string {
			idx, ok := colIdx[col]
			if !ok || idx >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[idx])
		}

		title := get("poll_title")
		if title == "" {
			importErrors = append(importErrors, ImportError{Row: rowNum, Reason: "poll_title is empty"})
			continue
		}
		if len([]rune(title)) > 255 {
			importErrors = append(importErrors, ImportError{Row: rowNum, Reason: "poll_title exceeds 255 characters"})
			continue
		}

		pollTypeStr := get("poll_type")
		if pollTypeStr != "survey" && pollTypeStr != "quiz" {
			importErrors = append(importErrors, ImportError{Row: rowNum, Reason: "poll_type must be 'survey' or 'quiz'"})
			continue
		}
		isQuiz := pollTypeStr == "quiz"

		optLabels := make([]string, 0, 8)
		tooLong := false
		for n := 1; n <= 8; n++ {
			label := get(fmt.Sprintf("option_%d", n))
			if label == "" {
				break
			}
			if len([]rune(label)) > 255 {
				importErrors = append(importErrors, ImportError{Row: rowNum, Reason: fmt.Sprintf("option_%d exceeds 255 characters", n)})
				tooLong = true
				break
			}
			optLabels = append(optLabels, label)
		}
		if tooLong {
			continue
		}
		if len(optLabels) < 2 {
			importErrors = append(importErrors, ImportError{Row: rowNum, Reason: "at least 2 non-empty options required"})
			continue
		}

		correctIdxSet := make(map[int]struct{})
		if isQuiz {
			co := get("correct_options")
			if co == "" {
				importErrors = append(importErrors, ImportError{Row: rowNum, Reason: "correct_options is required for quiz"})
				continue
			}
			parts := strings.Split(co, "|")
			valid := true
			for _, p := range parts {
				n, err := strconv.Atoi(strings.TrimSpace(p))
				if err != nil || n < 1 || n > len(optLabels) {
					importErrors = append(importErrors, ImportError{Row: rowNum, Reason: fmt.Sprintf("correct_options value '%s' is out of range (1-%d)", p, len(optLabels))})
					valid = false
					break
				}
				correctIdxSet[n] = struct{}{}
			}
			if !valid {
				continue
			}
			if len(correctIdxSet) == 0 {
				importErrors = append(importErrors, ImportError{Row: rowNum, Reason: "correct_options must have at least one valid value"})
				continue
			}
		}

		points := 0
		if isQuiz {
			if pStr := get("points"); pStr != "" {
				if p, err := strconv.Atoi(pStr); err == nil && p >= 0 {
					points = p
				}
			}
		}

		maxSel := 1
		if mStr := get("max_selections"); mStr != "" {
			if m, err := strconv.Atoi(mStr); err == nil && m >= 1 {
				maxSel = m
			}
		}
		if maxSel > len(optLabels) {
			maxSel = len(optLabels)
		}

		opts := make([]store.Option, len(optLabels))
		for j, label := range optLabels {
			_, isCorrect := correctIdxSet[j+1]
			opts[j] = store.Option{Label: label, IsCorrect: isCorrect}
		}

		parsed = append(parsed, store.BulkPollInput{
			Title:         title,
			IsQuiz:        isQuiz,
			Points:        points,
			MaxSelections: maxSel,
			Options:       opts,
		})
	}

	return parsed, importErrors, nil
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
