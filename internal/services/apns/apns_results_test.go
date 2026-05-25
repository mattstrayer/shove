package apns

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/sideshow/apns2"
)

type fakeEmitter struct{ emitted []ResultEnvelope }

func (f *fakeEmitter) Emit(ctx context.Context, r ResultEnvelope) error {
	f.emitted = append(f.emitted, r)
	return nil
}

type fakeFC struct{ invalid []string }

func (f *fakeFC) TokenInvalid(_, t string)                    { f.invalid = append(f.invalid, t) }
func (f *fakeFC) ReplaceToken(_, _, _ string)                 {}
func (f *fakeFC) CountPush(_ string, _ bool, _ time.Duration) {}

type fakeAPNSClient struct {
	resp *apns2.Response
	err  error
}

func (f *fakeAPNSClient) Push(_ *apns2.Notification) (*apns2.Response, error) {
	return f.resp, f.err
}

func TestEmitResultFromAPNSResponse(t *testing.T) {
	cases := []struct {
		name       string
		resp       *apns2.Response
		err        error
		corr       string
		wantStatus ResultStatus
		wantEmit   bool
	}{
		{
			name:       "delivered 200",
			resp:       &apns2.Response{StatusCode: 200, ApnsID: "gw-id"},
			corr:       "uuid-1",
			wantStatus: ResultStatusDelivered,
			wantEmit:   true,
		},
		{
			name:       "invalid BadDeviceToken",
			resp:       &apns2.Response{StatusCode: 400, Reason: apns2.ReasonBadDeviceToken},
			corr:       "uuid-2",
			wantStatus: ResultStatusInvalid,
			wantEmit:   true,
		},
		{
			name:       "invalid Unregistered",
			resp:       &apns2.Response{StatusCode: 410, Reason: apns2.ReasonUnregistered},
			corr:       "uuid-3",
			wantStatus: ResultStatusInvalid,
			wantEmit:   true,
		},
		{
			name:       "failed 503",
			resp:       &apns2.Response{StatusCode: 503, Reason: "ServiceUnavailable"},
			corr:       "uuid-4",
			wantStatus: ResultStatusFailed,
			wantEmit:   true,
		},
		{
			name:       "transport error",
			err:        context.DeadlineExceeded,
			corr:       "uuid-5",
			wantStatus: ResultStatusFailed,
			wantEmit:   true,
		},
		{
			name:     "no correlation id - no emit",
			resp:     &apns2.Response{StatusCode: 200, ApnsID: "gw-id"},
			corr:     "",
			wantEmit: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fe := &fakeEmitter{}
			fc := &fakeFC{}
			a := &APNS{
				log:     slog.New(slog.NewTextHandler(os.Stderr, nil)),
				emitter: fe,
			}
			notif := apnsNotification{
				notification:  &apns2.Notification{DeviceToken: "tok"},
				correlationID: tc.corr,
			}
			a.pushWithClient(&fakeAPNSClient{resp: tc.resp, err: tc.err}, notif, fc)

			if !tc.wantEmit {
				if len(fe.emitted) != 0 {
					t.Fatalf("expected no emit, got %+v", fe.emitted)
				}
				return
			}
			if len(fe.emitted) != 1 {
				t.Fatalf("expected 1 emit, got %d", len(fe.emitted))
			}
			got := fe.emitted[0]
			if got.Status != tc.wantStatus {
				t.Fatalf("status = %q want %q", got.Status, tc.wantStatus)
			}
			if got.CorrelationID != tc.corr {
				t.Fatalf("correlationID = %q want %q", got.CorrelationID, tc.corr)
			}
			if got.Service != "apns" && got.Service != "apns-sandbox" {
				t.Fatalf("unexpected service: %q", got.Service)
			}
		})
	}
}
