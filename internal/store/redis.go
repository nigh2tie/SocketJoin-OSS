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

package store

import (
	"context"
	"fmt"

	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(redisURL string) (*RedisStore, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opts)
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return &RedisStore{client: client}, nil
}

// rateLimitScript は固定窓レート制限を原子的に実装する Lua スクリプト。
var rateLimitScript = redis.NewScript(`
    local count = redis.call('INCR', KEYS[1])
    if count == 1 then
        redis.call('EXPIRE', KEYS[1], ARGV[1])
    end
    return count
`)

func (r *RedisStore) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *RedisStore) RateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	val, err := rateLimitScript.Run(ctx, r.client, []string{key}, int(window.Seconds())).Int64()
	if err != nil {
		return false, err
	}
	return val <= int64(limit), nil
}

func (r *RedisStore) IncrementVote(ctx context.Context, pollID, optionID uuid.UUID) error {
	key := fmt.Sprintf("poll:%s:votes", pollID)
	pipe := r.client.Pipeline()
	pipe.HIncrBy(ctx, key, optionID.String(), 1)
	pipe.Expire(ctx, key, 90*24*time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}

func (r *RedisStore) GetVoteCounts(ctx context.Context, pollID uuid.UUID) (map[string]int64, error) {
	key := fmt.Sprintf("poll:%s:votes", pollID)
	res, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int64)
	for k, v := range res {
		var count int64
		fmt.Sscanf(v, "%d", &count)
		counts[k] = count
	}
	return counts, nil
}

func (r *RedisStore) IsBanned(ctx context.Context, eventID uuid.UUID, visitorID string) (bool, error) {
	key := fmt.Sprintf("ban:%s:%s", eventID, visitorID)
	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

func (r *RedisStore) AddBan(ctx context.Context, eventID uuid.UUID, visitorID string, ttl time.Duration) error {
	key := fmt.Sprintf("ban:%s:%s", eventID, visitorID)
	return r.client.Set(ctx, key, 1, ttl).Err()
}

func (r *RedisStore) RemoveBan(ctx context.Context, eventID uuid.UUID, visitorID string) error {
	key := fmt.Sprintf("ban:%s:%s", eventID, visitorID)
	return r.client.Del(ctx, key).Err()
}

// AcquireCleanupLock tries to acquire a distributed lock for the data-retention
// cleanup job. Returns true if the lock was acquired (this instance should run
// the job). The lock expires after ttl, preventing duplicate runs across instances.
func (r *RedisStore) AcquireCleanupLock(ctx context.Context, ttl time.Duration) (bool, error) {
	ok, err := r.client.SetNX(ctx, "cleanup:lock", 1, ttl).Result()
	return ok, err
}

// ResetPollVotes deletes the Redis vote hash for a poll so counts restart at 0.
func (r *RedisStore) ResetPollVotes(ctx context.Context, pollID uuid.UUID) error {
	key := fmt.Sprintf("poll:%s:votes", pollID)
	return r.client.Del(ctx, key).Err()
}

// PublishEventMessage publishes a message to a room-specific Redis channel
func (r *RedisStore) PublishEventMessage(ctx context.Context, eventID string, message []byte) error {
	channel := fmt.Sprintf("event:%s:channel", eventID)
	return r.client.Publish(ctx, channel, string(message)).Err()
}

// SubscribeToEvents subscribes to all event channels and returns a pubsub object.
// Use pubsub.Channel() to receive messages.
func (r *RedisStore) SubscribeToEvents(ctx context.Context) *redis.PubSub {
	return r.client.PSubscribe(ctx, "event:*:channel")
}
