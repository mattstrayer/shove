package apns_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	apns2 "github.com/sideshow/apns2"

	queueredis "github.com/mattstrayer/shove/internal/queue/redis"
	"github.com/mattstrayer/shove/internal/services/apns"
)

// adapter bridges apns.ResultEmitter -> queueredis.ResultEmitter.
type adapter struct{ inner *queueredis.ResultEmitter }

func (a adapter) Emit(ctx context.Context, r apns.ResultEnvelope) error {
	return a.inner.Emit(ctx, queueredis.Result{
		CorrelationID: r.CorrelationID,
		Service:       r.Service,
		Status:        queueredis.ResultStatus(r.Status),
		GatewayID:     r.GatewayID,
		HTTPStatus:    r.HTTPStatus,
		ErrorCode:     r.ErrorCode,
		ErrorMessage:  r.ErrorMessage,
		LatencyMS:     r.LatencyMS,
		Timestamp:     r.Timestamp,
	})
}

type stubClient struct {
	resp *apns2.Response
	err  error
}

func (s *stubClient) Push(_ *apns2.Notification) (*apns2.Response, error) {
	return s.resp, s.err
}

type noopFC struct{}

func (noopFC) TokenInvalid(_, _ string)                    {}
func (noopFC) ReplaceToken(_, _, _ string)                 {}
func (noopFC) CountPush(_ string, _ bool, _ time.Duration) {}

func TestAPNS_EndToEnd_ResultsEmitted(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatal(err)
	}
	defer mr.Close()
	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	emitter := queueredis.NewResultEmitter(client)

	cases := []struct {
		name       string
		corr       string
		resp       *apns2.Response
		wantStatus queueredis.ResultStatus
	}{
		{"delivered", "corr-1", &apns2.Response{StatusCode: 200, ApnsID: "gw-1"}, queueredis.ResultStatusDelivered},
		{"invalid", "corr-2", &apns2.Response{StatusCode: 410, Reason: apns2.ReasonUnregistered}, queueredis.ResultStatusInvalid},
		{"failed", "corr-3", &apns2.Response{StatusCode: 503, Reason: "ServiceUnavailable"}, queueredis.ResultStatusFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mr.FlushAll()
			a := apns.NewAPNSForTest(slog.New(slog.NewTextHandler(os.Stderr, nil)))
			a.SetResultEmitter(adapter{inner: emitter})

			body, _ := json.Marshal(map[string]any{
				"token":          "tok",
				"correlation_id": tc.corr,
				"headers":        map[string]any{"apns-topic": "app.vandal.app"},
				"payload":        map[string]any{"aps": map[string]any{"alert": "hi"}},
			})
			smsg, err := a.ConvertMessage(body)
			if err != nil {
				t.Fatalf("ConvertMessage: %v", err)
			}

			apns.PushForTest(a, &stubClient{resp: tc.resp}, smsg, noopFC{})

			items, err := client.LRange(context.Background(), "shove:results", 0, -1).Result()
			if err != nil {
				t.Fatalf("LRange: %v", err)
			}
			if len(items) != 1 {
				t.Fatalf("expected 1 result, got %d", len(items))
			}
			var got queueredis.Result
			if err := json.Unmarshal([]byte(items[0]), &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got.Status != tc.wantStatus {
				t.Fatalf("status = %q want %q", got.Status, tc.wantStatus)
			}
			if got.CorrelationID != tc.corr {
				t.Fatalf("correlation_id = %q want %q", got.CorrelationID, tc.corr)
			}
		})
	}
}
