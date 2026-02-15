package main

import (
	"log"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port              string
	TelegramBotToken  string
	TelegramChatID    string
	DiscordWebhookURL string
	CodeExpiration    time.Duration
	SessionDuration   time.Duration
	CookieName        string
}

func LoadConfig() Config {
	return Config{
		Port:              getEnv("PORT", "8080"),
		TelegramBotToken:  getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:    getEnv("TELEGRAM_CHAT_ID", ""),
		DiscordWebhookURL: getEnv("DISCORD_WEBHOOK_URL", ""),
		CodeExpiration:    getEnvDuration("CODE_EXPIRATION", 5*time.Minute),
		SessionDuration:   getEnvDuration("SESSION_DURATION", 24*time.Hour),
		CookieName:        getEnv("COOKIE_NAME", "traefik_auth_code"),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
		log.Printf("Invalid duration for %s, using fallback: %v", key, fallback)
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	if value, ok := os.LookupEnv(key); ok {
		if i, err := strconv.ParseInt(value, 10, 64); err == nil {
			return i
		}
		log.Printf("Invalid int64 for %s, using fallback: %v", key, fallback)
	}
	return fallback
}

// Helper to check if a string is empty
func (c Config) Validate() {
	if c.TelegramBotToken == "" && c.DiscordWebhookURL == "" {
		log.Println("WARNING: No notification channel configured (Telegram or Discord). Codes cannot be sent.")
	}
	if c.TelegramBotToken != "" && c.TelegramChatID == "" {
		log.Println("WARNING: Telegram Bot Token set but no Chat ID.")
	}
}
