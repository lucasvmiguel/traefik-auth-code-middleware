package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// Notifier sends notifications to Telegram.
type Notifier struct {
	BotToken string
	ChatID   string
}

func New(botToken, chatID string) *Notifier {
	return &Notifier{
		BotToken: botToken,
		ChatID:   chatID,
	}
}

func (t *Notifier) SendCode(code, ip string) error {
	msg := fmt.Sprintf("🔐 Authorization Code: *%s*\n\nRequest from IP: `%s`", code, ip)
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.BotToken)

	w := map[string]string{
		"chat_id":    t.ChatID,
		"text":       msg,
		"parse_mode": "Markdown",
	}
	body, _ := json.Marshal(w)

	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram api error: status %d", resp.StatusCode)
	}
	return nil
}
