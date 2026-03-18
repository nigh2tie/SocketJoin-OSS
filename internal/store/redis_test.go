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

package store_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nigh2tie/SocketJoin-OSS/internal/store"
)

func newTestRedis(t *testing.T) *store.RedisStore {
	t.Helper()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Skip("TEST_REDIS_URL not set")
	}
	r, err := store.NewRedisStore(url)
	if err != nil {
		t.Fatalf("NewRedisStore: %v", err)
	}
	return r
}

// --- BAN lifecycle ---

func TestBanLifecycle(t *testing.T) {
	r := newTestRedis(t)
	ctx := context.Background()
	eventID := uuid.New()
	visitorID := uuid.New().String()

	// Initially not banned
	banned, err := r.IsBanned(ctx, eventID, visitorID)
	if err != nil {
		t.Fatalf("IsBanned: %v", err)
	}
	if banned {
		t.Fatal("expected visitor to not be banned initially")
	}

	// Ban the visitor
	if err := r.AddBan(ctx, eventID, visitorID, time.Minute); err != nil {
		t.Fatalf("AddBan: %v", err)
	}
	t.Cleanup(func() { r.RemoveBan(context.Background(), eventID, visitorID) })

	// Should now be banned
	banned, err = r.IsBanned(ctx, eventID, visitorID)
	if err != nil {
		t.Fatalf("IsBanned after ban: %v", err)
	}
	if !banned {
		t.Fatal("expected visitor to be banned after AddBan")
	}

	// Unban
	if err := r.RemoveBan(ctx, eventID, visitorID); err != nil {
		t.Fatalf("RemoveBan: %v", err)
	}

	// Should be unbanned
	banned, err = r.IsBanned(ctx, eventID, visitorID)
	if err != nil {
		t.Fatalf("IsBanned after RemoveBan: %v", err)
	}
	if banned {
		t.Fatal("expected visitor to be unbanned after RemoveBan")
	}
}

func TestBanIsEventScoped(t *testing.T) {
	r := newTestRedis(t)
	ctx := context.Background()
	event1 := uuid.New()
	event2 := uuid.New()
	visitorID := uuid.New().String()

	if err := r.AddBan(ctx, event1, visitorID, time.Minute); err != nil {
		t.Fatalf("AddBan: %v", err)
	}
	t.Cleanup(func() { r.RemoveBan(context.Background(), event1, visitorID) })

	// Banned in event1
	banned, err := r.IsBanned(ctx, event1, visitorID)
	if err != nil {
		t.Fatalf("IsBanned event1: %v", err)
	}
	if !banned {
		t.Fatal("expected banned in event1")
	}

	// Not banned in event2 (ban is event-scoped)
	banned, err = r.IsBanned(ctx, event2, visitorID)
	if err != nil {
		t.Fatalf("IsBanned event2: %v", err)
	}
	if banned {
		t.Fatal("expected NOT banned in event2: ban must be event-scoped")
	}
}

// --- AcquireCleanupLock ---

func TestAcquireCleanupLock_OnlyOneWins(t *testing.T) {
	r := newTestRedis(t)
	ctx := context.Background()

	// Use short TTL so the test cleans up quickly.
	// If another test is holding the lock this test skips gracefully.
	ok, err := r.AcquireCleanupLock(ctx, 3*time.Second)
	if err != nil {
		t.Fatalf("AcquireCleanupLock: %v", err)
	}
	if !ok {
		t.Skip("cleanup:lock already held (another test or process) — skipping")
	}

	// Second attempt must fail while lock is held
	ok2, err := r.AcquireCleanupLock(ctx, 3*time.Second)
	if err != nil {
		t.Fatalf("AcquireCleanupLock (2nd): %v", err)
	}
	if ok2 {
		t.Error("second AcquireCleanupLock succeeded but should have been rejected")
	}
}
