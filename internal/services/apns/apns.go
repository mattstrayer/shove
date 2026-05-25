package apns

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"log/slog"

	"github.com/mattstrayer/shove/internal/services"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/token"
)

// apns2Pusher abstracts the subset of *apns2.Client used by pushWithClient,
// enabling fake clients in tests.
type apns2Pusher interface {
	Push(*apns2.Notification) (*apns2.Response, error)
}

// APNS ...
type APNS struct {
	production bool
	log        *slog.Logger
	keyID      string
	teamID     string
	authKey    *ecdsa.PrivateKey
	emitter    ResultEmitter
}

// SetResultEmitter wires an emitter used to publish delivery results.
func (apns *APNS) SetResultEmitter(e ResultEmitter) { apns.emitter = e }

// NewAPNS creates a new APNS service from a file path
func NewAPNS(authKeyPath, keyID, teamID string, production bool, log *slog.Logger) (apns *APNS, err error) {
	authKeyBytes, err := ioutil.ReadFile(authKeyPath)
	if err != nil {
		return nil, err
	}
	return NewAPNSFromKey(authKeyBytes, keyID, teamID, production, log)
}

// NewAPNSFromKey creates a new APNS service from raw key bytes (PEM format)
func NewAPNSFromKey(authKeyBytes []byte, keyID, teamID string, production bool, log *slog.Logger) (apns *APNS, err error) {
	// Parse the auth key
	block, _ := pem.Decode(authKeyBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	authKey, ok := parsedKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not an ECDSA private key")
	}

	apns = &APNS{
		authKey:    authKey,
		keyID:      keyID,
		teamID:     teamID,
		production: production,
		log:        log,
	}
	return
}

// NewAPNSFromBase64 creates a new APNS service from a base64-encoded key
func NewAPNSFromBase64(authKeyBase64, keyID, teamID string, production bool, log *slog.Logger) (apns *APNS, err error) {
	// Remove any whitespace/newlines
	authKeyBase64 = strings.TrimSpace(authKeyBase64)
	authKeyBase64 = strings.ReplaceAll(authKeyBase64, "\n", "")
	authKeyBase64 = strings.ReplaceAll(authKeyBase64, " ", "")

	authKeyBytes, err := base64.StdEncoding.DecodeString(authKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 key: %w", err)
	}

	return NewAPNSFromKey(authKeyBytes, keyID, teamID, production, log)
}

func (apns *APNS) Logger() *slog.Logger {
	return apns.log
}

func (apns *APNS) NewClient() (pclient services.PumpClient, err error) {
	authToken := &token.Token{
		AuthKey: apns.authKey,
		KeyID:   apns.keyID,
		TeamID:  apns.teamID,
	}

	client := apns2.NewTokenClient(authToken)
	if apns.production {
		client.Production()
	} else {
		client.Development()
	}
	pclient = client
	return
}

// ID ...
func (apns *APNS) ID() string {
	if apns.production {
		return "apns"
	}
	return "apns-sandbox"

}

// String ...
func (apns *APNS) String() string {
	if apns.production {
		return "APNS"
	}
	return "APNS-sandbox"
}

func (apns *APNS) SquashAndPushMessage(client services.PumpClient, smsgs []services.ServiceMessage, fc services.FeedbackCollector) services.PushStatus {
	panic("not implemented")
}

func (apns *APNS) PushMessage(pclient services.PumpClient, smsg services.ServiceMessage, fc services.FeedbackCollector) services.PushStatus {
	client := pclient.(*apns2.Client)
	return apns.pushWithClient(client, smsg.(apnsNotification), fc)
}

func (apns *APNS) pushWithClient(client apns2Pusher, notif apnsNotification, fc services.FeedbackCollector) services.PushStatus {
	t := time.Now()
	resp, err := client.Push(notif.notification)
	duration := time.Since(t)
	sent := false
	var status services.PushStatus
	var envelope ResultEnvelope
	envelope.CorrelationID = notif.correlationID
	envelope.Service = apns.ID()
	envelope.LatencyMS = duration.Milliseconds()
	envelope.Timestamp = time.Now().Unix()

	if err != nil {
		apns.log.Error("Push message failed", "error", err)
		status = services.PushStatusTempFail
		envelope.Status = ResultStatusFailed
		envelope.ErrorMessage = err.Error()
	} else {
		reason := resp.Reason
		if reason == "" {
			reason = "OK"
		}
		apns.log.Info("Pushed", "reason", reason, "duration", duration)
		sent = resp.Sent()
		envelope.GatewayID = resp.ApnsID
		envelope.HTTPStatus = resp.StatusCode
		envelope.ErrorCode = resp.Reason

		if resp.Reason == apns2.ReasonBadDeviceToken || resp.Reason == apns2.ReasonUnregistered {
			fc.TokenInvalid(apns.ID(), notif.notification.DeviceToken)
			envelope.Status = ResultStatusInvalid
			status = services.PushStatusHardFail
		} else if sent {
			envelope.Status = ResultStatusDelivered
			status = services.PushStatusSuccess
		} else if resp.StatusCode >= 500 {
			envelope.Status = ResultStatusFailed
			status = services.PushStatusTempFail
		} else {
			envelope.Status = ResultStatusFailed
			status = services.PushStatusHardFail
		}
	}
	fc.CountPush(apns.ID(), sent, duration)
	apns.emitResult(envelope)
	return status
}

func (apns *APNS) emitResult(env ResultEnvelope) {
	if apns.emitter == nil || env.CorrelationID == "" {
		return
	}
	if err := apns.emitter.Emit(context.Background(), env); err != nil {
		apns.log.Error("Failed to emit result", "error", err, "correlation_id", env.CorrelationID)
	}
}
