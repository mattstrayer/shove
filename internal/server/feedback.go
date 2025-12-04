package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/mattstrayer/shove/internal/queue"
)

const defaultFeedbackLimit = 1000

// handleFeedback retrieves and removes feedback entries (pop behavior).
// Query params:
//   - limit: max number of entries to return (default 1000)
func (s *Server) handleFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Invalid request method.", http.StatusMethodNotAllowed)
		return
	}

	limit := defaultFeedbackLimit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	feedback, err := s.feedbackStore.Pop(ctx, limit)
	if err != nil {
		slog.Error("Failed to retrieve feedback", "error", err)
		http.Error(w, "Failed to retrieve feedback", http.StatusInternalServerError)
		return
	}

	j, err := json.Marshal(struct {
		Feedback []queue.TokenFeedback `json:"feedback"`
	}{Feedback: feedback})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(j)
}

// handleFeedbackPeek retrieves feedback entries without removing them.
// Query params:
//   - limit: max number of entries to return (default 1000)
func (s *Server) handleFeedbackPeek(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Invalid request method.", http.StatusMethodNotAllowed)
		return
	}

	limit := defaultFeedbackLimit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	feedback, err := s.feedbackStore.Peek(ctx, limit)
	if err != nil {
		slog.Error("Failed to peek feedback", "error", err)
		http.Error(w, "Failed to retrieve feedback", http.StatusInternalServerError)
		return
	}

	count, _ := s.feedbackStore.Len(ctx)

	j, err := json.Marshal(struct {
		Feedback []queue.TokenFeedback `json:"feedback"`
		Total    int64                 `json:"total"`
	}{Feedback: feedback, Total: count})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(j)
}

// TokenInvalid records that a device token is no longer valid.
func (s *Server) TokenInvalid(serviceID, token string) {
	feedback := queue.TokenFeedback{
		Service:   serviceID,
		Token:     token,
		Reason:    "invalid",
		Timestamp: time.Now().Unix(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.feedbackStore.Push(ctx, feedback); err != nil {
		slog.Error("Failed to store invalid token feedback", "error", err, "service", serviceID, "token", token)
		return
	}
	slog.Info("Invalid token", "service", serviceID, "token", token)
}

// ReplaceToken records that a device token should be replaced with a new one.
func (s *Server) ReplaceToken(serviceID, token, replacement string) {
	feedback := queue.TokenFeedback{
		Service:     serviceID,
		Token:       token,
		Replacement: replacement,
		Reason:      "replaced",
		Timestamp:   time.Now().Unix(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.feedbackStore.Push(ctx, feedback); err != nil {
		slog.Error("Failed to store token replacement feedback", "error", err, "service", serviceID)
		return
	}
	slog.Info("Token replaced", "service", serviceID)
}
