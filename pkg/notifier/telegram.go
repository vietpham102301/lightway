package notifier

import (
	"context"
	"fmt"
	"net/http"

	"github.com/vietpham102301/lightway/pkg/httpclient"
)

// TelegramNotifier sends notifications via the Telegram Bot API.
// It implements the Notifier interface.
type TelegramNotifier struct {
	Token  string
	ChatID string
	Client *httpclient.Client
}

var _ Notifier = (*TelegramNotifier)(nil)

func NewTelegramNotifier(client *httpclient.Client, token, chatID string) *TelegramNotifier {
	return &TelegramNotifier{
		Token:  token,
		ChatID: chatID,
		Client: client,
	}
}

// Send implements the Notifier interface.
func (t *TelegramNotifier) Send(message string) error {
	if t.Token == "" || t.ChatID == "" {
		return fmt.Errorf("telegram token or chat id is empty")
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.Token)

	payload := map[string]string{
		"chat_id": t.ChatID,
		"text":    message,
	}

	_, err := t.Client.RequestBytes(context.Background(), http.MethodPost, url, payload, nil)
	if err != nil {
		return fmt.Errorf("failed to send telegram request: %w", err)
	}

	return nil
}
