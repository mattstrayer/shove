package redis

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

// resultsKey is the Redis list shove pushes per-send results onto.
const resultsKey = "shove:results"

// ResultStatus enumerates the three terminal states for a push attempt.
type ResultStatus string

const (
	ResultStatusDelivered ResultStatus = "delivered"
	ResultStatusFailed    ResultStatus = "failed"
	ResultStatusInvalid   ResultStatus = "invalid"
)

// Result is the envelope shove writes onto shove:results after each
// APNS/FCM round-trip. Shape is service-agnostic.
type Result struct {
	CorrelationID string       `json:"correlation_id"`
	Service       string       `json:"service"`
	Status        ResultStatus `json:"status"`
	GatewayID     string       `json:"gateway_id,omitempty"`
	HTTPStatus    int          `json:"http_status,omitempty"`
	ErrorCode     string       `json:"error_code,omitempty"`
	ErrorMessage  string       `json:"error_message,omitempty"`
	LatencyMS     int64        `json:"latency_ms,omitempty"`
	Timestamp     int64        `json:"timestamp"`
}

// ResultEmitter LPUSHes Result envelopes onto shove:results.
// It mirrors FeedbackStore's shape but is intentionally a separate type:
// results have a distinct schema and lifecycle from token feedback.
type ResultEmitter struct {
	client *redis.Client
}

// NewResultEmitter wraps an existing Redis client.
func NewResultEmitter(client *redis.Client) *ResultEmitter {
	return &ResultEmitter{client: client}
}

// Emit writes one result envelope. Safe to call with an empty
// correlation_id — callers (handlers) should guard, but if a zero-value
// envelope gets through we still write it so the operator notices.
func (e *ResultEmitter) Emit(ctx context.Context, r Result) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return e.client.LPush(ctx, resultsKey, data).Err()
}
