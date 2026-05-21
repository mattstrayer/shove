package apns

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
)

func newTestAPNS(t *testing.T) *APNS {
	t.Helper()
	return &APNS{log: slog.New(slog.NewTextHandler(os.Stderr, nil))}
}

func TestConvertMessage_CorrelationID(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantCorr string
	}{
		{
			name: "with correlation_id",
			body: `{
				"token": "abc",
				"correlation_id": "9b3f-uuid",
				"headers": {"apns-topic": "app.vandal.app"},
				"payload": {"aps":{"alert":"hi"}}
			}`,
			wantCorr: "9b3f-uuid",
		},
		{
			name: "without correlation_id",
			body: `{
				"token": "abc",
				"headers": {"apns-topic": "app.vandal.app"},
				"payload": {"aps":{"alert":"hi"}}
			}`,
			wantCorr: "",
		},
	}
	a := newTestAPNS(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			smsg, err := a.ConvertMessage([]byte(tc.body))
			if err != nil {
				t.Fatalf("ConvertMessage: %v", err)
			}
			n := smsg.(apnsNotification)
			if n.correlationID != tc.wantCorr {
				t.Fatalf("correlationID = %q, want %q", n.correlationID, tc.wantCorr)
			}
		})
	}
}

func TestConvertMessage_PayloadDoesNotContainCorrelationID(t *testing.T) {
	a := newTestAPNS(t)
	body := `{
		"token": "abc",
		"correlation_id": "9b3f-uuid",
		"headers": {"apns-topic": "app.vandal.app"},
		"payload": {"aps":{"alert":"hi"}}
	}`
	smsg, err := a.ConvertMessage([]byte(body))
	if err != nil {
		t.Fatalf("ConvertMessage: %v", err)
	}
	n := smsg.(apnsNotification)
	raw, err := json.Marshal(n.notification.Payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if bytes.Contains(raw, []byte("correlation_id")) {
		t.Fatalf("forwarded payload contains correlation_id: %s", raw)
	}
}
