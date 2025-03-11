package fcm

import (
	"encoding/json"
	"errors"

	"github.com/mattstrayer/shove/internal/services"
)

type fcmMessage struct {
	To              string   `json:"to"`
	RegistrationIDs []string `json:"registration_ids"`
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
	msg.rawData = data
	return msg, nil
}

// Validate ...
func (fcm *FCM) Validate(data []byte) error {
	_, err := fcm.ConvertMessage(data)
	return err
}
