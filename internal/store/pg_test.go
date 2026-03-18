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

// Package store_test contains integration tests for the PostgreSQL store.
// These tests require a live database. Set TEST_DB_URL to the connection string
// before running, e.g.:
//
//	TEST_DB_URL="postgres://user:pass@localhost:5432/socketjoin_test?sslmode=disable" go test ./internal/store/...
//
// The target database must have all migrations applied.
// Each test creates its own isolated event and cleans up via t.Cleanup
// (ON DELETE CASCADE handles all child rows automatically).
package store_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/nigh2tie/SocketJoin-OSS/internal/store"
)

// newTestStore connects to TEST_DB_URL or skips the test.
func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dbURL := os.Getenv("TEST_DB_URL")
	if dbURL == "" {
		t.Skip("TEST_DB_URL not set; skipping DB integration test")
	}
	s, err := store.NewStore(dbURL)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s
}

// createTestEvent creates a uniquely-named event and registers a cleanup that
// deletes it (cascading to all child rows) when the test finishes.
func createTestEvent(t *testing.T, s *store.Store) *store.Event {
	t.Helper()
	ctx := context.Background()
	event, err := s.CreateEvent(ctx, "test-"+uuid.NewString()[:8], "optional")
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
	t.Cleanup(func() {
		if err := s.DeleteEvent(context.Background(), event.ID); err != nil {
			t.Logf("cleanup DeleteEvent %s: %v", event.ID, err)
		}
	})
	return event
}

// --- BulkCreatePolls ---

func TestBulkCreatePolls_InsertsAll(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	event := createTestEvent(t, s)

	inputs := []store.BulkPollInput{
		{
			Title:         "Survey poll",
			IsQuiz:        false,
			MaxSelections: 1,
			Options: []store.Option{
				{Label: "Yes"},
				{Label: "No"},
			},
		},
		{
			Title:         "Quiz poll",
			IsQuiz:        true,
			Points:        10,
			MaxSelections: 1,
			Options: []store.Option{
				{Label: "Right", IsCorrect: true},
				{Label: "Wrong"},
			},
		},
	}

	if err := s.BulkCreatePolls(ctx, event.ID, inputs); err != nil {
		t.Fatalf("BulkCreatePolls: %v", err)
	}

	polls, err := s.GetPollsForEvent(ctx, event.ID)
	if err != nil {
		t.Fatalf("GetPollsForEvent: %v", err)
	}
	if len(polls) != 2 {
		t.Errorf("poll count = %d, want 2", len(polls))
	}

	// Verify titles are preserved
	titles := map[string]bool{}
	for _, p := range polls {
		titles[p.Title] = true
	}
	for _, inp := range inputs {
		if !titles[inp.Title] {
			t.Errorf("poll %q not found after bulk insert", inp.Title)
		}
	}
}

func TestBulkCreatePolls_Empty(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	event := createTestEvent(t, s)

	// An empty slice should succeed without inserting anything
	if err := s.BulkCreatePolls(ctx, event.ID, nil); err != nil {
		t.Fatalf("BulkCreatePolls(nil): %v", err)
	}

	polls, _ := s.GetPollsForEvent(ctx, event.ID)
	if len(polls) != 0 {
		t.Errorf("poll count = %d, want 0", len(polls))
	}
}

// --- ResetPollVotes ---

func TestResetPollVotes_ClearsVotesAndReopens(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	event := createTestEvent(t, s)

	// Create a survey poll
	poll, err := s.CreatePoll(ctx, event.ID, "Reset test", false, 0, 1, []store.Option{
		{Label: "A"},
		{Label: "B"},
	})
	if err != nil {
		t.Fatalf("CreatePoll: %v", err)
	}

	optA := poll.Options[0].ID

	// Cast a vote
	visitorID := uuid.NewString()
	if err := s.CreateVote(ctx, poll.ID, []uuid.UUID{optA}, visitorID, "tester"); err != nil {
		t.Fatalf("CreateVote: %v", err)
	}

	// Verify vote was recorded
	counts, err := s.GetVoteCountsFromDB(ctx, poll.ID)
	if err != nil {
		t.Fatalf("GetVoteCountsFromDB: %v", err)
	}
	if counts[optA.String()] != 1 {
		t.Errorf("vote count before reset = %d, want 1", counts[optA.String()])
	}

	// Reset
	if err := s.ResetPollVotes(ctx, poll.ID); err != nil {
		t.Fatalf("ResetPollVotes: %v", err)
	}

	// Votes cleared
	counts, err = s.GetVoteCountsFromDB(ctx, poll.ID)
	if err != nil {
		t.Fatalf("GetVoteCountsFromDB after reset: %v", err)
	}
	if len(counts) != 0 {
		t.Errorf("counts after reset = %v, want empty", counts)
	}

	// Poll is back to open — same visitor can vote again
	if err := s.CreateVote(ctx, poll.ID, []uuid.UUID{optA}, visitorID, "tester"); err != nil {
		t.Errorf("CreateVote after reset: expected success, got %v", err)
	}
}

// --- CloseAndScorePoll ---

func TestCloseAndScorePoll_CorrectVoterScored(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	event := createTestEvent(t, s)

	// Quiz poll: opt A = correct, opt B = incorrect, 10 points
	poll, err := s.CreatePoll(ctx, event.ID, "Quiz", true, 10, 1, []store.Option{
		{Label: "A", IsCorrect: true},
		{Label: "B", IsCorrect: false},
	})
	if err != nil {
		t.Fatalf("CreatePoll: %v", err)
	}

	optA := poll.Options[0].ID
	optB := poll.Options[1].ID

	aliceID := uuid.NewString()
	bobID := uuid.NewString()

	// Alice votes correctly (A)
	if err := s.CreateVote(ctx, poll.ID, []uuid.UUID{optA}, aliceID, "alice"); err != nil {
		t.Fatalf("CreateVote alice: %v", err)
	}
	// Bob votes incorrectly (B)
	if err := s.CreateVote(ctx, poll.ID, []uuid.UUID{optB}, bobID, "bob"); err != nil {
		t.Fatalf("CreateVote bob: %v", err)
	}

	if err := s.CloseAndScorePoll(ctx, poll.ID); err != nil {
		t.Fatalf("CloseAndScorePoll: %v", err)
	}

	ranking, err := s.GetRanking(ctx, event.ID, 10)
	if err != nil {
		t.Fatalf("GetRanking: %v", err)
	}

	if len(ranking) != 1 {
		t.Fatalf("ranking entries = %d, want 1 (only alice)", len(ranking))
	}
	if ranking[0].Nickname != "alice" {
		t.Errorf("rank-1 nickname = %q, want alice", ranking[0].Nickname)
	}
	if ranking[0].TotalScore != 10 {
		t.Errorf("rank-1 score = %d, want 10", ranking[0].TotalScore)
	}
	if ranking[0].Rank != 1 {
		t.Errorf("rank-1 rank = %d, want 1", ranking[0].Rank)
	}
}

func TestCloseAndScorePoll_IncorrectVoterNotScored(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	event := createTestEvent(t, s)

	poll, err := s.CreatePoll(ctx, event.ID, "Quiz wrong", true, 10, 1, []store.Option{
		{Label: "Right", IsCorrect: true},
		{Label: "Wrong", IsCorrect: false},
	})
	if err != nil {
		t.Fatalf("CreatePoll: %v", err)
	}

	wrongOpt := poll.Options[1].ID // "Wrong"
	if err := s.CreateVote(ctx, poll.ID, []uuid.UUID{wrongOpt}, uuid.NewString(), "charlie"); err != nil {
		t.Fatalf("CreateVote: %v", err)
	}

	if err := s.CloseAndScorePoll(ctx, poll.ID); err != nil {
		t.Fatalf("CloseAndScorePoll: %v", err)
	}

	ranking, err := s.GetRanking(ctx, event.ID, 10)
	if err != nil {
		t.Fatalf("GetRanking: %v", err)
	}
	if len(ranking) != 0 {
		t.Errorf("ranking = %v, want empty (charlie voted wrong)", ranking)
	}
}

func TestCloseAndScorePoll_Idempotent(t *testing.T) {
	// Closing an already-closed poll must not add scores a second time.
	s := newTestStore(t)
	ctx := context.Background()
	event := createTestEvent(t, s)

	poll, err := s.CreatePoll(ctx, event.ID, "Idempotent quiz", true, 10, 1, []store.Option{
		{Label: "Correct", IsCorrect: true},
		{Label: "Wrong"},
	})
	if err != nil {
		t.Fatalf("CreatePoll: %v", err)
	}

	correctOpt := poll.Options[0].ID
	if err := s.CreateVote(ctx, poll.ID, []uuid.UUID{correctOpt}, uuid.NewString(), "dave"); err != nil {
		t.Fatalf("CreateVote: %v", err)
	}

	// First close+score
	if err := s.CloseAndScorePoll(ctx, poll.ID); err != nil {
		t.Fatalf("CloseAndScorePoll (1st): %v", err)
	}

	// Second call: poll is already closed so votes cannot be cast after first close,
	// but the SQL should not error when called again on an already-closed poll.
	if err := s.CloseAndScorePoll(ctx, poll.ID); err != nil {
		t.Fatalf("CloseAndScorePoll (2nd): %v", err)
	}

	ranking, err := s.GetRanking(ctx, event.ID, 10)
	if err != nil {
		t.Fatalf("GetRanking: %v", err)
	}
	if len(ranking) != 1 {
		t.Fatalf("ranking count = %d, want 1", len(ranking))
	}
	// Score must be 10, not 20 (not double-added)
	if ranking[0].TotalScore != 10 {
		t.Errorf("score after double close = %d, want 10", ranking[0].TotalScore)
	}
}

// --- GetRanking ---

func TestGetRanking_OrderedByScore(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	event := createTestEvent(t, s)

	// Poll 1: 10 points
	poll1, err := s.CreatePoll(ctx, event.ID, "Q1", true, 10, 1, []store.Option{
		{Label: "Right", IsCorrect: true},
		{Label: "Wrong"},
	})
	if err != nil {
		t.Fatalf("CreatePoll poll1: %v", err)
	}
	// Poll 2: 5 points
	poll2, err := s.CreatePoll(ctx, event.ID, "Q2", true, 5, 1, []store.Option{
		{Label: "Right", IsCorrect: true},
		{Label: "Wrong"},
	})
	if err != nil {
		t.Fatalf("CreatePoll poll2: %v", err)
	}

	aliceID := uuid.NewString()
	bobID := uuid.NewString()

	// Alice answers both correctly → 15 points
	if err := s.CreateVote(ctx, poll1.ID, []uuid.UUID{poll1.Options[0].ID}, aliceID, "alice"); err != nil {
		t.Fatalf("alice vote poll1: %v", err)
	}
	if err := s.CreateVote(ctx, poll2.ID, []uuid.UUID{poll2.Options[0].ID}, aliceID, "alice"); err != nil {
		t.Fatalf("alice vote poll2: %v", err)
	}

	// Bob answers only poll1 correctly → 10 points
	if err := s.CreateVote(ctx, poll1.ID, []uuid.UUID{poll1.Options[0].ID}, bobID, "bob"); err != nil {
		t.Fatalf("bob vote poll1: %v", err)
	}
	if err := s.CreateVote(ctx, poll2.ID, []uuid.UUID{poll2.Options[1].ID}, bobID, "bob"); err != nil {
		t.Fatalf("bob vote poll2 (wrong): %v", err)
	}

	if err := s.CloseAndScorePoll(ctx, poll1.ID); err != nil {
		t.Fatalf("CloseAndScorePoll poll1: %v", err)
	}
	if err := s.CloseAndScorePoll(ctx, poll2.ID); err != nil {
		t.Fatalf("CloseAndScorePoll poll2: %v", err)
	}

	ranking, err := s.GetRanking(ctx, event.ID, 10)
	if err != nil {
		t.Fatalf("GetRanking: %v", err)
	}
	if len(ranking) != 2 {
		t.Fatalf("ranking count = %d, want 2", len(ranking))
	}

	// Alice should be rank 1
	if ranking[0].Nickname != "alice" || ranking[0].TotalScore != 15 || ranking[0].Rank != 1 {
		t.Errorf("rank[0] = {%s %d rank%d}, want {alice 15 rank1}", ranking[0].Nickname, ranking[0].TotalScore, ranking[0].Rank)
	}
	// Bob should be rank 2
	if ranking[1].Nickname != "bob" || ranking[1].TotalScore != 10 || ranking[1].Rank != 2 {
		t.Errorf("rank[1] = {%s %d rank%d}, want {bob 10 rank2}", ranking[1].Nickname, ranking[1].TotalScore, ranking[1].Rank)
	}
}

func TestGetRanking_LimitRespected(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	event := createTestEvent(t, s)

	poll, err := s.CreatePoll(ctx, event.ID, "Limit quiz", true, 5, 1, []store.Option{
		{Label: "Correct", IsCorrect: true},
		{Label: "Wrong"},
	})
	if err != nil {
		t.Fatalf("CreatePoll: %v", err)
	}
	correctOpt := poll.Options[0].ID

	// Create 5 correct voters
	voters := []string{"v1", "v2", "v3", "v4", "v5"}
	for _, v := range voters {
		if err := s.CreateVote(ctx, poll.ID, []uuid.UUID{correctOpt}, uuid.NewString(), v); err != nil {
			t.Fatalf("CreateVote %s: %v", v, err)
		}
	}

	if err := s.CloseAndScorePoll(ctx, poll.ID); err != nil {
		t.Fatalf("CloseAndScorePoll: %v", err)
	}

	// limit=3 should return at most 3 entries
	ranking, err := s.GetRanking(ctx, event.ID, 3)
	if err != nil {
		t.Fatalf("GetRanking: %v", err)
	}
	if len(ranking) != 3 {
		t.Errorf("ranking count = %d, want 3", len(ranking))
	}
}

// --- CreateVote duplicate prevention ---

func TestCreateVote_AlreadyVotedReturnsError(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	event := createTestEvent(t, s)

	poll, err := s.CreatePoll(ctx, event.ID, "Dup vote test", false, 0, 1, []store.Option{
		{Label: "A"},
		{Label: "B"},
	})
	if err != nil {
		t.Fatalf("CreatePoll: %v", err)
	}
	optA := poll.Options[0].ID
	visitorID := uuid.NewString()

	if err := s.CreateVote(ctx, poll.ID, []uuid.UUID{optA}, visitorID, ""); err != nil {
		t.Fatalf("first vote: %v", err)
	}
	err = s.CreateVote(ctx, poll.ID, []uuid.UUID{optA}, visitorID, "")
	if err == nil {
		t.Error("second vote for same visitor: expected error, got nil")
	}
}

// --- Multi-select voting ---

func TestCreateVote_MultiSelect(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	event := createTestEvent(t, s)

	// Poll allows up to 2 selections
	poll, err := s.CreatePoll(ctx, event.ID, "Multi-select", false, 0, 2, []store.Option{
		{Label: "A"},
		{Label: "B"},
		{Label: "C"},
	})
	if err != nil {
		t.Fatalf("CreatePoll: %v", err)
	}

	optA := poll.Options[0].ID
	optB := poll.Options[1].ID

	// Vote for two options simultaneously
	visitorID := uuid.NewString()
	if err := s.CreateVote(ctx, poll.ID, []uuid.UUID{optA, optB}, visitorID, "multi"); err != nil {
		t.Fatalf("CreateVote multi-select: %v", err)
	}

	counts, err := s.GetVoteCountsFromDB(ctx, poll.ID)
	if err != nil {
		t.Fatalf("GetVoteCountsFromDB: %v", err)
	}
	if counts[optA.String()] != 1 {
		t.Errorf("optA count = %d, want 1", counts[optA.String()])
	}
	if counts[optB.String()] != 1 {
		t.Errorf("optB count = %d, want 1", counts[optB.String()])
	}
}

func TestCreateModerator_EnforcesLimit(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	event := createTestEvent(t, s)

	for i := 1; i <= 3; i++ {
		if _, err := s.CreateModerator(ctx, event.ID, "mod-"+uuid.NewString()[:8]); err != nil {
			t.Fatalf("CreateModerator #%d: %v", i, err)
		}
	}

	_, err := s.CreateModerator(ctx, event.ID, "overflow")
	if !errors.Is(err, store.ErrModeratorLimitReached) {
		t.Fatalf("CreateModerator #4 error = %v, want ErrModeratorLimitReached", err)
	}
}

func TestQuestionMethods_RespectEventBoundaryAndMaskVisitor(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	eventA := createTestEvent(t, s)
	eventB := createTestEvent(t, s)

	authorID := uuid.NewString()
	q, err := s.CreateQuestion(ctx, eventA.ID, authorID, "How are we protecting boundaries?")
	if err != nil {
		t.Fatalf("CreateQuestion: %v", err)
	}

	_, err = s.ToggleQuestionUpvote(ctx, eventB.ID, q.ID, uuid.NewString())
	if !errors.Is(err, store.ErrQuestionNotFound) {
		t.Fatalf("ToggleQuestionUpvote cross-event error = %v, want ErrQuestionNotFound", err)
	}

	err = s.UpdateQuestionStatus(ctx, eventB.ID, q.ID, "answered")
	if !errors.Is(err, store.ErrQuestionNotFound) {
		t.Fatalf("UpdateQuestionStatus cross-event error = %v, want ErrQuestionNotFound", err)
	}

	voterID := uuid.NewString()
	added, err := s.ToggleQuestionUpvote(ctx, eventA.ID, q.ID, voterID)
	if err != nil {
		t.Fatalf("ToggleQuestionUpvote same-event: %v", err)
	}
	if !added {
		t.Fatalf("ToggleQuestionUpvote same-event added = false, want true")
	}

	if err := s.UpdateQuestionStatus(ctx, eventA.ID, q.ID, "answered"); err != nil {
		t.Fatalf("UpdateQuestionStatus same-event: %v", err)
	}

	questions, err := s.GetQuestionsByEvent(ctx, eventA.ID, voterID)
	if err != nil {
		t.Fatalf("GetQuestionsByEvent: %v", err)
	}
	found := false
	for _, got := range questions {
		if got.ID != q.ID {
			continue
		}
		found = true
		if got.Status != "answered" {
			t.Errorf("status = %q, want answered", got.Status)
		}
		if got.Upvotes != 1 {
			t.Errorf("upvotes = %d, want 1", got.Upvotes)
		}
		if !got.IsUpvoted {
			t.Error("is_upvoted = false, want true")
		}
		if got.VisitorID != "" {
			t.Errorf("visitor_id leaked in response: %q", got.VisitorID)
		}
	}
	if !found {
		t.Fatal("question not found in eventA question list")
	}
}
