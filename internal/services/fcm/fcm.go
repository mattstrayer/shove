package fcm

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"log/slog"

	firebase "firebase.google.com/go/v4"
	errorutils "firebase.google.com/go/v4/errorutils"
	"firebase.google.com/go/v4/messaging"
	"github.com/mattstrayer/shove/internal/services"
)

// FCM ...
type FCM struct {
	client *messaging.Client
	log    *slog.Logger
}

// NewFCM ...
func NewFCM(log *slog.Logger) (fcm *FCM, err error) {
	app, err := firebase.NewApp(context.Background(), nil)
	if err != nil {
		fcm.log.Error("error initializing Firebase app", "error", err)
		return nil, err
	}

	ctx := context.Background()
	client, err := app.Messaging(ctx)
	if err != nil {
		fcm.log.Error("error getting FCM Messaging client", "error", err)
	}

	fcm = &FCM{
		client: client,
		log:    log,
	}
	return
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
	msg := smsg.(fcmMessage)
	startedAt := time.Now()

	message := messaging.Message{}
	err := json.Unmarshal(msg.rawData, &message)
	if err != nil {
		fcm.log.Error("error unmarshalling message", "error", err)
		return services.PushStatusHardFail
	}

	message.Token = msg.To

	var success bool

	// Send a message to the device corresponding to the provided
	// registration token.
	response, err := fcm.client.Send(context.Background(), &message)

	fcm.log.Info("Sending", "response", response, "error", err)
	if err != nil {
		fcm.log.Error("sending failed", "error", err)

		// Only define conditions where we need to hard fail.
		// all others will be temp failed by default
		// https://github.com/firebase/firebase-admin-go/blob/master/internal/errors.go#L68
		if errorutils.IsInvalidArgument(err) {
			return services.PushStatusHardFail
		}

		if errorutils.IsDataLoss(err) {
			return services.PushStatusHardFail
		}

		if errorutils.IsNotFound(err) {
			// you should remove the registration ID from your
			// server database because the application was
			// uninstalled from the device or it does not have a
			// broadcast receiver configured to receive
			// com.google.android.c2dm.intent.RECEIVE intents.
			fc.TokenInvalid(fcm.ID(), msg.To)
			return services.PushStatusHardFail
		}

		return services.PushStatusTempFail
	}

	duration := time.Since(startedAt)

	defer func() {
		fc.CountPush(fcm.ID(), success, duration)
	}()

	fcm.log.Info("Pushed", "duration", duration)

	success = true
	return services.PushStatusSuccess
}
