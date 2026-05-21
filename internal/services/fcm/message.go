package fcm

import (
	"encoding/json"
	"errors"

	"github.com/mattstrayer/shove/internal/services"
)

type fcmMessage struct {
	To              string   `json:"to"`
	RegistrationIDs []string `json:"registration_ids"`
	CorrelationID   string   `json:"correlation_id,omitempty"`
	rawData         []byte
}

func (fcmMessage) GetSquashKey() string {
	panic("not implemented")
}

func (fcm *FCM) ConvertMessage(data []byte) (smsg services.ServiceMessage, err error) {
	var msg fcmMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	if len(msg.RegistrationIDs) >= 1000 {
		return nil, errors.New("too many tokens")
	}
	if msg.To == "" && len(msg.RegistrationIDs) == 0 {
		return nil, errors.New("no token specified")
	}
	if msg.To != "" && len(msg.RegistrationIDs) > 0 {
		return nil, errors.New("both to/registration_ids specified")
	}
	// Strip correlation_id from the raw bytes that get forwarded to FCM.
	// We re-marshal a generic map after deleting the key. This is cheap
	// (one push per message) and makes the strip behavior explicit rather
	// than relying on messaging.Message's silent unknown-field drop.
	if msg.CorrelationID != "" {
		var generic map[string]json.RawMessage
		if err := json.Unmarshal(data, &generic); err == nil {
			delete(generic, "correlation_id")
			if stripped, mErr := json.Marshal(generic); mErr == nil {
				data = stripped
			}
		}
	}
	msg.rawData = data
	return msg, nil
}

// Validate ...
func (fcm *FCM) Validate(data []byte) error {
	_, err := fcm.ConvertMessage(data)
	return err
}
