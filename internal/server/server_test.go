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
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nigh2tie/SocketJoin-OSS/internal/service"
)

// --- isUnsafeMethod ---

func TestIsUnsafeMethod(t *testing.T) {
	cases := []struct {
		method string
		want   bool
	}{
		{http.MethodPost, true},
		{http.MethodPut, true},
		{http.MethodPatch, true},
		{http.MethodDelete, true},
		{http.MethodGet, false},
		{http.MethodHead, false},
		{http.MethodOptions, false},
	}
	for _, tc := range cases {
		if got := isUnsafeMethod(tc.method); got != tc.want {
			t.Errorf("isUnsafeMethod(%q) = %v, want %v", tc.method, got, tc.want)
		}
	}
}

// --- checkNGWord ---

func TestCheckNGWord(t *testing.T) {
	s := service.NewPollService(nil, nil, defaultNGWords)
	cases := []struct {
		text string
		want bool
	}{
		{"hello world", false},
		{"NGWord test", true},
		{"contains badword here", true},
		{"SPAM message", true},
		{"clean text", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := s.CheckNGWord(tc.text); got != tc.want {
			t.Errorf("checkNGWord(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}

// --- handleCSVTemplate ---

func TestHandleCSVTemplate(t *testing.T) {
	s := &Server{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/polls/template.csv", nil)
	s.handleCSVTemplate(w, r)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/csv") {
		t.Errorf("Content-Type = %q, want text/csv", ct)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "attachment") {
		t.Errorf("Content-Disposition = %q, missing attachment", cd)
	}

	reader := csv.NewReader(resp.Body)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("csv parse: %v", err)
	}
	// 5 comment rows + header row + 3 example rows
	if len(records) != 9 {
		t.Errorf("row count = %d, want 9", len(records))
	}

	wantHeaders := []string{"poll_title", "poll_type", "option_1", "option_2", "points", "correct_options"}
	header := records[5]
	headerSet := make(map[string]struct{}, len(header))
	for _, h := range header {
		headerSet[h] = struct{}{}
	}
	for _, col := range wantHeaders {
		if _, ok := headerSet[col]; !ok {
			t.Errorf("template header missing column %q", col)
		}
	}
}

// --- parseCSVImport ---

func TestParseCSVImport(t *testing.T) {
	cases := []struct {
		name       string
		input      string
		wantPolls  int
		wantErrors int
		wantFatal  bool
	}{
		// Happy paths
		{
			name:      "survey with 2 options",
			input:     "poll_title,poll_type,option_1,option_2\nQ,survey,A,B\n",
			wantPolls: 1,
		},
		{
			name:      "survey with 4 options",
			input:     "poll_title,poll_type,option_1,option_2,option_3,option_4\nQ,survey,A,B,C,D\n",
			wantPolls: 1,
		},
		{
			name:      "quiz with single correct",
			input:     "poll_title,poll_type,option_1,option_2,points,correct_options\nQ,quiz,A,B,10,1\n",
			wantPolls: 1,
		},
		{
			name:      "quiz with multiple correct pipe-separated",
			input:     "poll_title,poll_type,option_1,option_2,option_3,points,correct_options\nQ,quiz,A,B,C,20,1|3\n",
			wantPolls: 1,
		},
		{
			name:      "BOM prefix stripped",
			input:     "\xEF\xBB\xBFpoll_title,poll_type,option_1,option_2\nQ,survey,A,B\n",
			wantPolls: 1,
		},
		{
			name:      "extra whitespace in headers trimmed",
			input:     " poll_title , poll_type , option_1 , option_2 \nQ,survey,A,B\n",
			wantPolls: 1,
		},
		{
			name:      "multiple valid rows",
			input:     "poll_title,poll_type,option_1,option_2\nQ1,survey,A,B\nQ2,survey,C,D\n",
			wantPolls: 2,
		},

		// Fatal errors (non-nil error returned)
		{
			name:      "only header row",
			input:     "poll_title,poll_type,option_1,option_2\n",
			wantFatal: true,
		},
		{
			name:      "missing required column poll_title",
			input:     "poll_type,option_1,option_2\nsurvey,A,B\n",
			wantFatal: true,
		},
		{
			name:      "missing required column option_2",
			input:     "poll_title,poll_type,option_1\nQ,survey,A\n",
			wantFatal: true,
		},

		// Per-row errors
		{
			name:       "empty poll_title",
			input:      "poll_title,poll_type,option_1,option_2\n,survey,A,B\n",
			wantErrors: 1,
		},
		{
			name:       "invalid poll_type",
			input:      "poll_title,poll_type,option_1,option_2\nQ,unknown,A,B\n",
			wantErrors: 1,
		},
		{
			name:       "only one non-empty option",
			input:      "poll_title,poll_type,option_1,option_2\nQ,survey,A,\n",
			wantErrors: 1,
		},
		{
			name:       "quiz missing correct_options column value",
			input:      "poll_title,poll_type,option_1,option_2,correct_options\nQ,quiz,A,B,\n",
			wantErrors: 1,
		},
		{
			name:       "quiz correct_options index out of range",
			input:      "poll_title,poll_type,option_1,option_2,correct_options\nQ,quiz,A,B,5\n",
			wantErrors: 1,
		},
		{
			name:       "quiz correct_options non-numeric",
			input:      "poll_title,poll_type,option_1,option_2,correct_options\nQ,quiz,A,B,abc\n",
			wantErrors: 1,
		},
		{
			name:       "quiz correct_options zero index",
			input:      "poll_title,poll_type,option_1,option_2,correct_options\nQ,quiz,A,B,0\n",
			wantErrors: 1,
		},

		// Mixed valid/invalid rows
		{
			name:       "one valid one empty-title row",
			input:      "poll_title,poll_type,option_1,option_2\nGood,survey,A,B\n,survey,C,D\n",
			wantPolls:  1,
			wantErrors: 1,
		},

		// Verify option IsCorrect mapping
		{
			name:      "quiz correct option mapped correctly",
			input:     "poll_title,poll_type,option_1,option_2,option_3,correct_options\nQ,quiz,A,B,C,2\n",
			wantPolls: 1,
		},
	}

	s := service.NewPollService(nil, nil, defaultNGWords)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			polls, errs, err := s.ParseCSVImport([]byte(tc.input))

			if tc.wantFatal {
				if err == nil {
					t.Error("expected fatal error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected fatal error: %v", err)
			}
			if len(polls) != tc.wantPolls {
				t.Errorf("parsed polls = %d, want %d", len(polls), tc.wantPolls)
			}
			if len(errs) != tc.wantErrors {
				t.Errorf("import errors = %d, want %d; errors: %v", len(errs), tc.wantErrors, errs)
			}
		})
	}
}

// TestParseCSVImport_CorrectOptionMapping verifies that is_correct is set on the
// right options when correct_options references specific 1-based option indices.
func TestParseCSVImport_CorrectOptionMapping(t *testing.T) {
	s := service.NewPollService(nil, nil, defaultNGWords)
	input := "poll_title,poll_type,option_1,option_2,option_3,correct_options\nQ,quiz,A,B,C,2\n"
	polls, errs, err := s.ParseCSVImport([]byte(input))
	if err != nil || len(errs) != 0 || len(polls) != 1 {
		t.Fatalf("unexpected: err=%v errs=%v polls=%d", err, errs, len(polls))
	}
	opts := polls[0].Options
	if len(opts) != 3 {
		t.Fatalf("option count = %d, want 3", len(opts))
	}
	// option index 2 (1-based) → opts[1]
	if opts[0].IsCorrect {
		t.Error("opts[0] (A) should NOT be correct")
	}
	if !opts[1].IsCorrect {
		t.Error("opts[1] (B) SHOULD be correct")
	}
	if opts[2].IsCorrect {
		t.Error("opts[2] (C) should NOT be correct")
	}
}

// TestParseCSVImport_MultiCorrect verifies pipe-separated correct options.
func TestParseCSVImport_MultiCorrect(t *testing.T) {
	s := service.NewPollService(nil, nil, defaultNGWords)
	input := "poll_title,poll_type,option_1,option_2,option_3,correct_options\nQ,quiz,A,B,C,1|3\n"
	polls, errs, err := s.ParseCSVImport([]byte(input))
	if err != nil || len(errs) != 0 || len(polls) != 1 {
		t.Fatalf("unexpected: err=%v errs=%v polls=%d", err, errs, len(polls))
	}
	opts := polls[0].Options
	if !opts[0].IsCorrect {
		t.Error("opts[0] (A) SHOULD be correct")
	}
	if opts[1].IsCorrect {
		t.Error("opts[1] (B) should NOT be correct")
	}
	if !opts[2].IsCorrect {
		t.Error("opts[2] (C) SHOULD be correct")
	}
}
