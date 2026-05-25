package fcm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"log/slog"

	firebase "firebase.google.com/go/v4"
	errorutils "firebase.google.com/go/v4/errorutils"
	"firebase.google.com/go/v4/messaging"
	"github.com/mattstrayer/shove/internal/services"
	"google.golang.org/api/option"
)

// FCM ...
type FCM struct {
	client  *messaging.Client
	log     *slog.Logger
	emitter ResultEmitter
}

// SetResultEmitter wires a ResultEmitter to receive per-send result envelopes.
func (fcm *FCM) SetResultEmitter(e ResultEmitter) { fcm.emitter = e }

// fcmSender abstracts *messaging.Client for testing.
type fcmSender interface {
	Send(ctx context.Context, m *messaging.Message) (string, error)
}

// NewFCM creates a new FCM service using GOOGLE_APPLICATION_CREDENTIALS file path
func NewFCM(log *slog.Logger) (fcm *FCM, err error) {
	app, err := firebase.NewApp(context.Background(), nil)
	if err != nil {
		log.Error("error initializing Firebase app", "error", err)
		return nil, err
	}

	ctx := context.Background()
	client, err := app.Messaging(ctx)
	if err != nil {
		log.Error("error getting FCM Messaging client", "error", err)
		return nil, err
	}

	fcm = &FCM{
		client: client,
		log:    log,
	}
	return
}

// NewFCMFromJSON creates a new FCM service from JSON credentials bytes
func NewFCMFromJSON(credentialsJSON []byte, log *slog.Logger) (fcm *FCM, err error) {
	opt := option.WithCredentialsJSON(credentialsJSON)
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		log.Error("error initializing Firebase app", "error", err)
		return nil, err
	}

	ctx := context.Background()
	client, err := app.Messaging(ctx)
	if err != nil {
		log.Error("error getting FCM Messaging client", "error", err)
		return nil, err
	}

	fcm = &FCM{
		client: client,
		log:    log,
	}
	return
}

// NewFCMFromBase64 creates a new FCM service from base64-encoded JSON credentials
func NewFCMFromBase64(credentialsBase64 string, log *slog.Logger) (fcm *FCM, err error) {
	// Remove any whitespace/newlines
	credentialsBase64 = strings.TrimSpace(credentialsBase64)
	credentialsBase64 = strings.ReplaceAll(credentialsBase64, "\n", "")
	credentialsBase64 = strings.ReplaceAll(credentialsBase64, " ", "")

	credentialsJSON, err := base64.StdEncoding.DecodeString(credentialsBase64)
	if err != nil {
		return nil, err
	}

	return NewFCMFromJSON(credentialsJSON, log)
}

func (fcm *FCM) Logger() *slog.Logger {
	return fcm.log
}

// ID ...
func (fcm *FCM) ID() string {
	return "fcm"
}

// String ...
func (fcm *FCM) String() string {
	return "FCM"
}

func (fcm *FCM) NewClient() (services.PumpClient, error) {

	client := &http.Client{
		Timeout: time.Duration(15 * time.Second),
		Transport: &http.Transport{
			MaxIdleConns:    5,
			IdleConnTimeout: 30 * time.Second,
		},
	}
	return client, nil
}

type fcmResponse struct {
	Success int `json:"success"`
	Failure int `json:"failure"`
	Results []struct {
		MessageID      string `json:"message_id"`
		RegistrationID string `json:"registration_id"`
		Error          string `json:"error"`
	} `json:"results"`
}

func (fcm *FCM) SquashAndPushMessage(services.PumpClient, []services.ServiceMessage, services.FeedbackCollector) services.PushStatus {
	panic("not implemented")
}

func (fcm *FCM) PushMessage(pclient services.PumpClient, smsg services.ServiceMessage, fc services.FeedbackCollector) services.PushStatus {
	return fcm.sendWithClient(fcm.client, smsg.(fcmMessage), fc)
}

func (fcm *FCM) sendWithClient(sender fcmSender, msg fcmMessage, fc services.FeedbackCollector) services.PushStatus {
	startedAt := time.Now()

	message := messaging.Message{}
	if err := json.Unmarshal(msg.rawData, &message); err != nil {
		fcm.log.Error("error unmarshalling message", "error", err)
		fcm.emitFailure(msg, "unmarshal_error", err.Error(), 0, time.Since(startedAt))
		return services.PushStatusHardFail
	}
	message.Token = msg.To

	gatewayID, err := sender.Send(context.Background(), &message)
	duration := time.Since(startedAt)
	fcm.log.Info("Sending", "gateway_id", gatewayID, "error", err)

	if err != nil {
		fcm.log.Error("sending failed", "error", err)

		status := ResultStatusFailed
		var pushStatus services.PushStatus

		// Only define conditions where we need to hard fail.
		// all others will be temp failed by default
		// https://github.com/firebase/firebase-admin-go/blob/master/internal/errors.go#L68
		switch {
		case errorutils.IsNotFound(err):
			// you should remove the registration ID from your
			// server database because the application was
			// uninstalled from the device or it does not have a
			// broadcast receiver configured to receive
			// com.google.android.c2dm.intent.RECEIVE intents.
			fc.TokenInvalid(fcm.ID(), msg.To)
			status = ResultStatusInvalid
			pushStatus = services.PushStatusHardFail
		case errorutils.IsInvalidArgument(err):
			status = ResultStatusInvalid
			pushStatus = services.PushStatusHardFail
		case errorutils.IsDataLoss(err):
			pushStatus = services.PushStatusHardFail
		default:
			pushStatus = services.PushStatusTempFail
		}

		fc.CountPush(fcm.ID(), false, duration)
		fcm.emitResult(ResultEnvelope{
			CorrelationID: msg.CorrelationID,
			Service:       fcm.ID(),
			Status:        status,
			ErrorCode:     errorCodeFor(err),
			ErrorMessage:  err.Error(),
			LatencyMS:     duration.Milliseconds(),
			Timestamp:     time.Now().Unix(),
		})
		return pushStatus
	}

	fc.CountPush(fcm.ID(), true, duration)
	fcm.log.Info("Pushed", "duration", duration)
	fcm.emitResult(ResultEnvelope{
		CorrelationID: msg.CorrelationID,
		Service:       fcm.ID(),
		Status:        ResultStatusDelivered,
		GatewayID:     gatewayID,
		HTTPStatus:    http.StatusOK,
		LatencyMS:     duration.Milliseconds(),
		Timestamp:     time.Now().Unix(),
	})
	return services.PushStatusSuccess
}

func errorCodeFor(err error) string {
	switch {
	case errorutils.IsNotFound(err):
		return "UNREGISTERED"
	case errorutils.IsInvalidArgument(err):
		return "INVALID_ARGUMENT"
	case errorutils.IsDataLoss(err):
		return "DATA_LOSS"
	}
	return "INTERNAL"
}

func (fcm *FCM) emitResult(env ResultEnvelope) {
	if fcm.emitter == nil || env.CorrelationID == "" {
		return
	}
	if err := fcm.emitter.Emit(context.Background(), env); err != nil {
		fcm.log.Error("Failed to emit result", "error", err, "correlation_id", env.CorrelationID)
	}
}

func (fcm *FCM) emitFailure(msg fcmMessage, code, message string, httpStatus int, dur time.Duration) {
	fcm.emitResult(ResultEnvelope{
		CorrelationID: msg.CorrelationID,
		Service:       fcm.ID(),
		Status:        ResultStatusFailed,
		ErrorCode:     code,
		ErrorMessage:  message,
		HTTPStatus:    httpStatus,
		LatencyMS:     dur.Milliseconds(),
		Timestamp:     time.Now().Unix(),
	})
}
