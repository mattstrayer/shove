package apns

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"time"

	"github.com/mattstrayer/shove/internal/services"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/token"
	"golang.org/x/exp/slog"
)

// APNS ...
type APNS struct {
	production bool
	log        *slog.Logger
	keyID      string
	teamID     string
	authKey    *ecdsa.PrivateKey
}

// NewAPNS ...
func NewAPNS(authKeyPath, keyID, teamID string, production bool, log *slog.Logger) (apns *APNS, err error) {
	// Read the auth key file
	authKeyBytes, err := ioutil.ReadFile(authKeyPath)
	if err != nil {
		return nil, err
	}

	// Parse the auth key
	block, _ := pem.Decode(authKeyBytes)
	if block == nil {
		return nil, err
	}

	parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	authKey, ok := parsedKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, err
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

func (apns *APNS) PushMessage(pclient services.PumpClient, smsg services.ServiceMessage, fc services.FeedbackCollector) (status services.PushStatus) {
	client := pclient.(*apns2.Client)
	notif := smsg.(apnsNotification)
	t := time.Now()
	resp, err := client.Push(notif.notification)
	duration := time.Now().Sub(t)
	sent := false
	if err != nil {
		apns.log.Error("Push message failed", "error", err)
		status = services.PushStatusTempFail
	} else {
		reason := resp.Reason
		if reason == "" {
			reason = "OK"
		}
		apns.log.Info("Pushed", "reason", reason, "duration", duration)
		sent = resp.Sent()
		if resp.Reason == apns2.ReasonBadDeviceToken || resp.Reason == apns2.ReasonUnregistered {
			fc.TokenInvalid(apns.ID(), notif.notification.DeviceToken)
		}
		retry := resp.StatusCode >= 500
		if sent {
			status = services.PushStatusSuccess
		} else if retry {
			status = services.PushStatusTempFail
		} else {
			status = services.PushStatusHardFail
		}
	}
	fc.CountPush(apns.ID(), sent, duration)
	return
}
