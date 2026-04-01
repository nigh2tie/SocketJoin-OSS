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

package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/nigh2tie/SocketJoin-OSS/internal/store"
)

type ImportError struct {
	Row    int    `json:"row"`
	Reason string `json:"reason"`
}

const MaxImportRows = 500

func (s *PollService) BulkCreatePolls(ctx context.Context, eventID uuid.UUID, inputs []store.BulkPollInput) error {
	for _, inp := range inputs {
		if len(inp.Options) < 2 {
			return ErrPollNeedsTwoOptions
		}
		if s.CheckNGWord(inp.Title) {
			return ErrNGWord
		}
		for _, opt := range inp.Options {
			if s.CheckNGWord(opt.Label) {
				return ErrNGWord
			}
		}
		if inp.IsQuiz {
			hasCorrect := false
			for _, opt := range inp.Options {
				if opt.IsCorrect {
					hasCorrect = true
					break
				}
			}
			if !hasCorrect {
				return ErrQuizNeedsCorrectOption
			}
		}
	}
	return s.pg.BulkCreatePolls(ctx, eventID, inputs)
}

func (s *PollService) ParseCSVImport(data []byte) ([]store.BulkPollInput, []ImportError, error) {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}

	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	filtered := make([][]string, 0, len(records))
	for _, row := range records {
		if len(row) == 0 {
			continue
		}
		first := strings.TrimSpace(row[0])
		if first == "" {
			empty := true
			for _, cell := range row {
				if strings.TrimSpace(cell) != "" {
					empty = false
					break
				}
			}
			if empty {
				continue
			}
		}
		if strings.HasPrefix(first, "#") {
			continue
		}
		filtered = append(filtered, row)
	}

	if len(filtered) < 2 {
		return nil, nil, fmt.Errorf("CSV must have a header row and at least one data row")
	}

	headers := filtered[0]
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

	dataRows := filtered[1:]
	if len(dataRows) > MaxImportRows {
		return nil, nil, fmt.Errorf("too many rows (max %d)", MaxImportRows)
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
		if s.CheckNGWord(title) {
			importErrors = append(importErrors, ImportError{Row: rowNum, Reason: "poll_title contains prohibited words"})
			continue
		}

		pollTypeStr := get("poll_type")
		if pollTypeStr != "survey" && pollTypeStr != "quiz" {
			importErrors = append(importErrors, ImportError{Row: rowNum, Reason: "poll_type must be 'survey' or 'quiz'"})
			continue
		}
		isQuiz := pollTypeStr == "quiz"

		optLabels := make([]string, 0, 8)
		optInvalid := false
		for n := 1; n <= 8; n++ {
			label := get(fmt.Sprintf("option_%d", n))
			if label == "" {
				break
			}
			if len([]rune(label)) > 255 {
				importErrors = append(importErrors, ImportError{Row: rowNum, Reason: fmt.Sprintf("option_%d exceeds 255 characters", n)})
				optInvalid = true
				break
			}
			if s.CheckNGWord(label) {
				importErrors = append(importErrors, ImportError{Row: rowNum, Reason: fmt.Sprintf("option_%d contains prohibited words", n)})
				optInvalid = true
				break
			}
			optLabels = append(optLabels, label)
		}
		if optInvalid {
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
