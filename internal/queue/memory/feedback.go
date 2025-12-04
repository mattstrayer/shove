package memory

import (
	"context"
	"sync"

	"github.com/mattstrayer/shove/internal/queue"
)

// FeedbackStore is an in-memory implementation of queue.FeedbackStore.
// Feedback is lost on server restart. Use Redis-backed store for persistence.
type FeedbackStore struct {
	mu       sync.Mutex
	feedback []queue.TokenFeedback
}

// NewFeedbackStore creates a new in-memory feedback store.
func NewFeedbackStore() *FeedbackStore {
	return &FeedbackStore{
		feedback: make([]queue.TokenFeedback, 0),
	}
}

// Push adds a feedback entry to the in-memory store.
func (s *FeedbackStore) Push(_ context.Context, feedback queue.TokenFeedback) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.feedback = append(s.feedback, feedback)
	return nil
}

// Pop retrieves and removes up to limit feedback entries from the store.
func (s *FeedbackStore) Pop(_ context.Context, limit int) ([]queue.TokenFeedback, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.feedback) == 0 {
		return []queue.TokenFeedback{}, nil
	}

	count := limit
	if count > len(s.feedback) || count <= 0 {
		count = len(s.feedback)
	}

	result := make([]queue.TokenFeedback, count)
	copy(result, s.feedback[:count])
	s.feedback = s.feedback[count:]

	return result, nil
}

// Peek retrieves up to limit feedback entries without removing them.
func (s *FeedbackStore) Peek(_ context.Context, limit int) ([]queue.TokenFeedback, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.feedback) == 0 {
		return []queue.TokenFeedback{}, nil
	}

	count := limit
	if count > len(s.feedback) || count <= 0 {
		count = len(s.feedback)
	}

	result := make([]queue.TokenFeedback, count)
	copy(result, s.feedback[:count])

	return result, nil
}

// Len returns the number of feedback entries in the store.
func (s *FeedbackStore) Len(_ context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return int64(len(s.feedback)), nil
}

// Close is a no-op for in-memory store.
func (s *FeedbackStore) Close() error {
	return nil
}

// Ensure FeedbackStore implements queue.FeedbackStore
var _ queue.FeedbackStore = (*FeedbackStore)(nil)

