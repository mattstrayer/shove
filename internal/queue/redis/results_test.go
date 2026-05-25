package redis

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/redis/go-redis/v9"
)

func newMiniRedis(t *testing.T) *redis.Client {
	t.Helper()
	s, err := startMiniredis()
	if err != nil {
		t.Skipf("miniredis unavailable: %v", err)
	}
	t.Cleanup(s.close)
	return redis.NewClient(&redis.Options{Addr: s.addr()})
}

func TestResultEmitter_EmitWritesEnvelope(t *testing.T) {
	client := newMiniRedis(t)
	emitter := NewResultEmitter(client)
	ctx := context.Background()

	res := Result{
		CorrelationID: "9b3f-uuid",
		Service:       "apns",
		Status:        ResultStatusDelivered,
		GatewayID:     "abc-apns-id",
		HTTPStatus:    200,
		LatencyMS:     87,
		Timestamp:     1716200000,
	}
	if err := emitter.Emit(ctx, res); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	got, err := client.LRange(ctx, "shove:results", 0, -1).Result()
	if err != nil {
		t.Fatalf("LRange: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	var decoded Result
	if err := json.Unmarshal([]byte(got[0]), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded != res {
		t.Fatalf("round-trip mismatch:\n got  %+v\n want %+v", decoded, res)
	}
}

func TestResultEmitter_EmitOmitsEmptyOptionalFields(t *testing.T) {
	client := newMiniRedis(t)
	emitter := NewResultEmitter(client)
	ctx := context.Background()

	if err := emitter.Emit(ctx, Result{
		CorrelationID: "uuid",
		Service:       "fcm",
		Status:        ResultStatusDelivered,
		Timestamp:     1,
	}); err != nil {
		t.Fatalf("Emit: %v", err)
	}
	got, _ := client.LRange(ctx, "shove:results", 0, -1).Result()
	for _, field := range []string{"gateway_id", "error_code", "error_message", "http_status", "latency_ms"} {
		if bytesContains(got[0], field) {
			t.Fatalf("expected %q to be omitted; payload: %s", field, got[0])
		}
	}
}

func bytesContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
