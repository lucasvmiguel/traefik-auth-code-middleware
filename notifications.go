package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type Notifier interface {
	SendCode(code, ip string) error
}

// Telegram Notifier
type TelegramNotifier struct {
	BotToken string
	ChatID   string
}

func (t *TelegramNotifier) SendCode(code, ip string) error {
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

// Discord Notifier
type DiscordNotifier struct {
	WebhookURL string
}

func (d *DiscordNotifier) SendCode(code, ip string) error {
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

// Factory
func NewNotifier(cfg Config) Notifier {
	if cfg.TelegramBotToken != "" && cfg.TelegramChatID != "" {
		log.Println("Using Telegram Notifier")
		return &TelegramNotifier{
			BotToken: cfg.TelegramBotToken,
			ChatID:   cfg.TelegramChatID,
		}
	}
	if cfg.DiscordWebhookURL != "" {
		log.Println("Using Discord Notifier")
		return &DiscordNotifier{
			WebhookURL: cfg.DiscordWebhookURL,
		}
	}
	// Fallback: Log only (for testing or misconfig)
	log.Println("No notifier configured. Codes will be logged to stdout.")
	return &LogNotifier{}
}

type LogNotifier struct{}

func (l *LogNotifier) SendCode(code, ip string) error {
	log.Printf("CODE GENERATED for %s: %s", ip, code)
	return nil
}
