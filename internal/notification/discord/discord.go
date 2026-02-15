package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// Notifier sends notifications to Discord.
type Notifier struct {
	WebhookURL string
}

func New(webhookURL string) *Notifier {
	return &Notifier{
		WebhookURL: webhookURL,
	}
}

func (d *Notifier) SendCode(code, ip string) error {
	msg := map[string]string{
		"content": fmt.Sprintf("🔐 **Authorization Code**: `%s`\nRequest from IP: `%s`", code, ip),
	}
	body, _ := json.Marshal(msg)

	resp, err := http.Post(d.WebhookURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("discord webhook error: status %d", resp.StatusCode)
	}
	return nil
}
