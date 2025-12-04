package queue

import "context"

// TokenFeedback represents feedback about an invalid or replaced device token.
type TokenFeedback struct {
	Service     string `json:"service"`
	Token       string `json:"token"`
	Replacement string `json:"replacement_token,omitempty"`
	Reason      string `json:"reason"`
	Timestamp   int64  `json:"timestamp"`
}

// FeedbackStore defines the interface for storing and retrieving device token feedback.
// Implementations can use in-memory storage, Redis, or other backends.
type FeedbackStore interface {
	// Push adds a feedback entry to the store.
	Push(ctx context.Context, feedback TokenFeedback) error

	// Pop retrieves and removes up to limit feedback entries from the store.
	// Returns an empty slice if no feedback is available.
	Pop(ctx context.Context, limit int) ([]TokenFeedback, error)

	// Peek retrieves up to limit feedback entries without removing them.
	// Useful for inspection or when external systems consume directly from Redis.
	Peek(ctx context.Context, limit int) ([]TokenFeedback, error)

	// Len returns the number of feedback entries in the store.
	Len(ctx context.Context) (int64, error)

	// Close releases any resources held by the store.
	Close() error
}

// FeedbackStoreFactory creates FeedbackStore instances.
type FeedbackStoreFactory interface {
	NewFeedbackStore() (FeedbackStore, error)
}

