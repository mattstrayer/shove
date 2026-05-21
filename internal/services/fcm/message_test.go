package fcm

import (
	"bytes"
	"testing"
)

func TestConvertMessage_CorrelationID(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantCorr string
	}{
		{
			name:     "with correlation_id",
			body:     `{"to":"tok","correlation_id":"9b3f-uuid","notification":{"title":"hi"}}`,
			wantCorr: "9b3f-uuid",
		},
		{
			name:     "without correlation_id",
			body:     `{"to":"tok","notification":{"title":"hi"}}`,
			wantCorr: "",
		},
	}
	f := &FCM{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			smsg, err := f.ConvertMessage([]byte(tc.body))
			if err != nil {
				t.Fatalf("ConvertMessage: %v", err)
			}
			m := smsg.(fcmMessage)
			if m.CorrelationID != tc.wantCorr {
				t.Fatalf("CorrelationID = %q, want %q", m.CorrelationID, tc.wantCorr)
			}
		})
	}
}

func TestConvertMessage_StripsCorrelationIDFromRawData(t *testing.T) {
	f := &FCM{}
	body := `{"to":"tok","correlation_id":"9b3f-uuid","notification":{"title":"hi"}}`
	smsg, err := f.ConvertMessage([]byte(body))
	if err != nil {
		t.Fatalf("ConvertMessage: %v", err)
	}
	m := smsg.(fcmMessage)
	if bytes.Contains(m.rawData, []byte("correlation_id")) {
		t.Fatalf("rawData still contains correlation_id: %s", m.rawData)
	}
	if m.CorrelationID != "9b3f-uuid" {
		t.Fatalf("CorrelationID lost during strip: %q", m.CorrelationID)
	}
}
