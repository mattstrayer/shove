package redis

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/mattstrayer/shove/internal/queue"
	"github.com/redis/go-redis/v9"
)

const feedbackKey = "shove:feedback"

// FeedbackStore is a Redis-backed implementation of queue.FeedbackStore.
// Feedback is persisted to Redis and survives server restarts.
// External systems can consume feedback directly from Redis using the key "shove:feedback".
type FeedbackStore struct {
	client *redis.Client
}

// NewFeedbackStore creates a new Redis-backed feedback store using an existing client.
func NewFeedbackStore(client *redis.Client) *FeedbackStore {
	return &FeedbackStore{client: client}
}

// NewFeedbackStoreFromURL creates a new Redis-backed feedback store from a Redis URL.
func NewFeedbackStoreFromURL(redisURL string) (*FeedbackStore, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	opt.PoolSize = 10
	opt.MinIdleConns = 2
	opt.PoolTimeout = time.Second * 30

	client := redis.NewClient(opt)

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	slog.Info("Redis feedback store connected", "key", feedbackKey)
	return &FeedbackStore{client: client}, nil
}

// Push adds a feedback entry to the Redis list.
// Uses LPUSH so newest entries are at the head of the list.
func (s *FeedbackStore) Push(ctx context.Context, feedback queue.TokenFeedback) error {
	data, err := json.Marshal(feedback)
	if err != nil {
		return err
	}
	return s.client.LPush(ctx, feedbackKey, data).Err()
}

// Pop retrieves and removes up to limit feedback entries from the store.
// Removes from the tail (oldest entries first - FIFO order).
func (s *FeedbackStore) Pop(ctx context.Context, limit int) ([]queue.TokenFeedback, error) {
	if limit <= 0 {
		limit = 100
	}

	pipe := s.client.Pipeline()
	lrangeCmd := pipe.LRange(ctx, feedbackKey, -int64(limit), -1)
	pipe.LTrim(ctx, feedbackKey, 0, -int64(limit+1))
	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}

	items, err := lrangeCmd.Result()
	if err != nil {
		return nil, err
	}

	result := make([]queue.TokenFeedback, 0, len(items))
	for _, item := range items {
		var feedback queue.TokenFeedback
		if err := json.Unmarshal([]byte(item), &feedback); err != nil {
			slog.Warn("Failed to unmarshal feedback entry", "error", err)
			continue
		}
		result = append(result, feedback)
	}

	return result, nil
}

// Peek retrieves up to limit feedback entries without removing them.
// Returns oldest entries first (from tail of list).
func (s *FeedbackStore) Peek(ctx context.Context, limit int) ([]queue.TokenFeedback, error) {
	if limit <= 0 {
		limit = 100
	}

	items, err := s.client.LRange(ctx, feedbackKey, -int64(limit), -1).Result()
	if err != nil {
		return nil, err
	}

	result := make([]queue.TokenFeedback, 0, len(items))
	for _, item := range items {
		var feedback queue.TokenFeedback
		if err := json.Unmarshal([]byte(item), &feedback); err != nil {
			slog.Warn("Failed to unmarshal feedback entry", "error", err)
			continue
		}
		result = append(result, feedback)
	}

	return result, nil
}

// Len returns the number of feedback entries in the store.
func (s *FeedbackStore) Len(ctx context.Context) (int64, error) {
	return s.client.LLen(ctx, feedbackKey).Result()
}

// Close closes the Redis client connection.
func (s *FeedbackStore) Close() error {
	return s.client.Close()
}

// Ensure FeedbackStore implements queue.FeedbackStore
var _ queue.FeedbackStore = (*FeedbackStore)(nil)

