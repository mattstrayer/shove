package fcm

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"firebase.google.com/go/v4/messaging"
)

type fakeEmitter struct{ emitted []ResultEnvelope }

func (f *fakeEmitter) Emit(_ context.Context, r ResultEnvelope) error {
	f.emitted = append(f.emitted, r)
	return nil
}

type fakeFC struct{ invalid []string }

func (f *fakeFC) TokenInvalid(_, t string)                    { f.invalid = append(f.invalid, t) }
func (f *fakeFC) ReplaceToken(_, _, _ string)                 {}
func (f *fakeFC) CountPush(_ string, _ bool, _ time.Duration) {}

type fakeSender struct {
	id  string
	err error
}

func (f *fakeSender) Send(_ context.Context, _ *messaging.Message) (string, error) {
	return f.id, f.err
}

func TestEmitResultFromFCMResponse(t *testing.T) {
	cases := []struct {
		name       string
		sender     *fakeSender
		corr       string
		wantStatus ResultStatus
		wantEmit   bool
	}{
		{
			name:       "delivered",
			sender:     &fakeSender{id: "projects/p/messages/abc"},
			corr:       "uuid-1",
			wantStatus: ResultStatusDelivered,
			wantEmit:   true,
		},
		{
			name:       "failed transport",
			sender:     &fakeSender{err: errors.New("connection refused")},
			corr:       "uuid-2",
			wantStatus: ResultStatusFailed,
			wantEmit:   true,
		},
		{
			name:     "no correlation id - no emit",
			sender:   &fakeSender{id: "x"},
			corr:     "",
			wantEmit: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fe := &fakeEmitter{}
			fc := &fakeFC{}
			f := &FCM{
				log:     slog.New(slog.NewTextHandler(os.Stderr, nil)),
				emitter: fe,
			}
			msg := fcmMessage{
				To:            "tok",
				CorrelationID: tc.corr,
				rawData:       []byte(`{"to":"tok"}`),
			}
			f.sendWithClient(tc.sender, msg, fc)
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
			if got.Service != "fcm" {
				t.Fatalf("service = %q", got.Service)
			}
		})
	}
}
