package telegram

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mattstrayer/shove/internal/services"
)

type telegramMessage struct {
	Method string `json:"method"`
	// This is intentionally kept as a raw message so that this can be fed 1:1
	// to the API.
	Payload       json.RawMessage `json:"payload"`
	parsedPayload telegramPayload
}

type telegramPayload struct {
	ChatID  string `json:"chat_id"`
	Text    string `json:"text,omitempty"`
	Caption string `json:"caption,omitempty"`
	Photo   string `json:"photo,omitempty"`
}

func (msg telegramMessage) GetSquashKey() string {
	// TODO: This should include method (`sendMessage`)
	return msg.parsedPayload.ChatID
}

func (tg *TelegramService) ConvertMessage(data []byte) (services.ServiceMessage, error) {
	var msg telegramMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	if !strings.HasPrefix(msg.Method, "send") {
		return nil, fmt.Errorf("invalid method: %s", msg.Method)
	}
	// Telegram documents the chat_id as: "Integer or String", we're assuming string.
	if err := json.Unmarshal(msg.Payload, &msg.parsedPayload); err != nil {
		return nil, err
	}
	if msg.parsedPayload.ChatID == "" {
		return nil, errors.New("missing `chat_id`")
	}
	return msg, nil
}

// Validate ...
func (tg *TelegramService) Validate(data []byte) error {
	_, err := tg.ConvertMessage(data)
	return err
}

func concatText(builder *strings.Builder, text string) {
	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return
	}
	texts := builder.String()
	if len(texts) == 0 || strings.HasSuffix(texts, "\n\n") {
		// No newlines needed
	} else if strings.HasSuffix(texts, "\n") {
		builder.WriteString("\n")
	} else {
		builder.WriteString("\n\n")
	}
	builder.WriteString(text)
}

func squashMessages(msgs []telegramMessage) (dmsg telegramMessage, err error) {
	if len(msgs) == 0 {
		err = errors.New("need at least one message to digest")
		return
	}
	dmsg = msgs[0]
	var texts strings.Builder
	var captions strings.Builder
	for _, msg := range msgs {
		if msg.Method != dmsg.Method {
			err = errors.New("cannot digest mix of methods")
			return
		}
		if msg.parsedPayload.ChatID != dmsg.parsedPayload.ChatID {
			err = errors.New("different `chat_id` seen while digesting")
			return
		}
		concatText(&texts, msg.parsedPayload.Text)
		concatText(&captions, msg.parsedPayload.Caption)
	}
	dmsg.parsedPayload.Text = texts.String()
	dmsg.parsedPayload.Caption = captions.String()
	dmsg.Payload, err = json.Marshal(&dmsg.parsedPayload)
	return
}
