package apns

import (
	"log/slog"

	"github.com/mattstrayer/shove/internal/services"
)

// NewAPNSForTest constructs an APNS struct with only the logger set, for
// tests that drive the handler with a fake apns2Pusher.
func NewAPNSForTest(log *slog.Logger) *APNS {
	return &APNS{log: log}
}

// PushForTest exposes pushWithClient to package-external tests.
func PushForTest(a *APNS, client apns2Pusher, smsg services.ServiceMessage, fc services.FeedbackCollector) services.PushStatus {
	return a.pushWithClient(client, smsg.(apnsNotification), fc)
}
