package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/lucasvieira/traefik-auth-code-middleware/internal/notification"
	"github.com/lucasvieira/traefik-auth-code-middleware/internal/notification/discord"
	"github.com/lucasvieira/traefik-auth-code-middleware/internal/notification/telegram"
	"github.com/lucasvieira/traefik-auth-code-middleware/internal/store"
	"github.com/lucasvieira/traefik-auth-code-middleware/internal/templates"
	"github.com/lucasvieira/traefik-auth-code-middleware/internal/utils"

	"github.com/urfave/cli/v2"
)

var (
	st       *store.Store
	notifier notification.Notifier

	// Flags
	port            string
	cookieName      string
	codeExpiration  time.Duration
	sessionDuration time.Duration
)

func main() {
	app := &cli.App{
		Name:  "traefik-auth-code-middleware",
		Usage: "Middleware for Traefik to authenticate users via code sent to Telegram/Discord",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "port",
				Value:       "8080",
				EnvVars:     []string{"PORT"},
				Destination: &port,
			},
			&cli.StringFlag{
				Name:    "telegram-bot-token",
				EnvVars: []string{"TELEGRAM_BOT_TOKEN"},
			},
			&cli.StringFlag{
				Name:    "telegram-chat-id",
				EnvVars: []string{"TELEGRAM_CHAT_ID"},
			},
			&cli.StringFlag{
				Name:    "discord-webhook-url",
				EnvVars: []string{"DISCORD_WEBHOOK_URL"},
			},
			&cli.DurationFlag{
				Name:        "code-expiration",
				Value:       5 * time.Minute,
				EnvVars:     []string{"CODE_EXPIRATION"},
				Destination: &codeExpiration,
			},
			&cli.DurationFlag{
				Name:        "session-duration",
				Value:       24 * time.Hour,
				EnvVars:     []string{"SESSION_DURATION"},
				Destination: &sessionDuration,
			},
			&cli.StringFlag{
				Name:        "cookie-name",
				Value:       "traefik_auth_code",
				EnvVars:     []string{"COOKIE_NAME"},
				Destination: &cookieName,
			},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run(c *cli.Context) error {
	st = store.NewStore()

	// Background cleanup
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			st.Cleanup()
		}
	}()

	// Setup Notifier
	telegramToken := c.String("telegram-bot-token")
	telegramChatID := c.String("telegram-chat-id")
	discordWebhook := c.String("discord-webhook-url")

	if telegramToken != "" && telegramChatID != "" {
		log.Println("Using Telegram Notifier")
		notifier = telegram.New(telegramToken, telegramChatID)
	} else if discordWebhook != "" {
		log.Println("Using Discord Notifier")
		notifier = discord.New(discordWebhook)
	} else {
		log.Println("WARNING: No notification channel configured. Codes will be logged.")
		notifier = &logNotifier{}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", authHandler)
	mux.HandleFunc("/login", loginHandler)
	mux.HandleFunc("/request-code", requestCodeHandler)
	mux.HandleFunc("/verify-code", verifyCodeHandler)

	addr := ":" + port
	log.Printf("Starting middleware on %s", addr)
	return http.ListenAndServe(addr, mux)
}

type logNotifier struct{}

func (l *logNotifier) SendCode(code, ip string) error {
	log.Printf("CODE GENERATED for %s: %s", ip, code)
	return nil
}

// Handlers

func authHandler(w http.ResponseWriter, r *http.Request) {
	// Whitelist paths
	uri := r.Header.Get("X-Forwarded-Uri")
	if uri == "/login" || uri == "/request-code" || uri == "/verify-code" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Check Cookie
	cookie, err := r.Cookie(cookieName)
	if err == nil && st.IsSessionValid(cookie.Value) {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Redirect to login
	forwardedHost := r.Header.Get("X-Forwarded-Host")
	forwardedProto := r.Header.Get("X-Forwarded-Proto")
	forwardedUri := r.Header.Get("X-Forwarded-Uri")

	if forwardedHost == "" {
		http.Error(w, "Unauthorized (Missing X-Forwarded-Host)", http.StatusUnauthorized)
		return
	}

	if forwardedProto == "" {
		forwardedProto = "https"
	}

	redirectURL := fmt.Sprintf("%s://%s%s", forwardedProto, forwardedHost, forwardedUri)

	w.WriteHeader(http.StatusUnauthorized)
	w.Header().Set("Content-Type", "text/html")
	if err := templates.LoginTmpl.Execute(w, templates.PageData{RedirectURL: redirectURL}); err != nil {
		log.Printf("Template error: %v", err)
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	redirectURL := r.URL.Query().Get("redirect_url")
	w.Header().Set("Content-Type", "text/html")
	templates.LoginTmpl.Execute(w, templates.PageData{RedirectURL: redirectURL})
}

func requestCodeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}
	redirectURL := r.FormValue("redirect_url")

	ip := utils.GetIP(r)

	// Rate Limiting: Check if code already exists for IP
	if existing := st.GetCode(ip); existing != nil {
		// Prevent spamming requests.
		// If code exists and is recent (e.g. < 1 min old), deny?
		// For simplicity, if a code exists, we can deny generating a new one immediately
		// or we can just overwrite it but rate limit based on time?
		// User requirement: "make sure an ip can not request multiple codes in ashort period"
		// We can add a "LastRequestedAt" to store, or just use existence of code.
		// Since code expiration is likely 5 mins, we don't want to block for 5 mins.
		// We can check if code was generated < 1 minute ago.
		// The store doesn't expose creation time directly in GetCode (it returns CodeData).
		// We can update CodeData to include CreatedAt or similar if needed.
		// For now, let's assume if a code exists, we wait.
		// Or better, update Store to handle "CanRequestCode" check.
		// Let's rely on a primitive: if code exists, reject.
		// BUT if user lost code, they need new one.
		// So we should allow after some time (e.g. 60s).
		// Let's modify Store to support this or just check ExpiresAt?
		// CodeData has ExpiresAt. CreatedAt = ExpiresAt - CodeExpiration.
		// Let's assume we want 1 min cooldown.

		// If ExpiresAt is > Now + (CodeExpiration - 1min), then it was created recently.
		// Example: Exp=5m. Created at T. Expires at T+5m.
		// If Now < T + 1m -> Now < (ExpiresAt - 4m).

		timeSinceCreation := codeExpiration - existing.ExpiresAt.Sub(time.Now())
		if timeSinceCreation < 1*time.Minute {
			w.WriteHeader(http.StatusTooManyRequests)
			templates.LoginTmpl.Execute(w, templates.PageData{Error: "Please wait before requesting a new code", RedirectURL: redirectURL})
			return
		}
	}

	code := utils.GenerateCode()
	st.SetCode(ip, code, codeExpiration)

	err := notifier.SendCode(code, ip)

	w.Header().Set("Content-Type", "text/html")
	if err != nil {
		log.Printf("Failed to send code to %s: %v", ip, err)
		w.WriteHeader(http.StatusInternalServerError)
		templates.LoginTmpl.Execute(w, templates.PageData{Error: "Failed to send notification", RedirectURL: redirectURL})
		return
	}

	templates.VerifyTmpl.Execute(w, templates.PageData{Message: "Code sent!", RedirectURL: redirectURL})
}

func verifyCodeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	time.Sleep(2 * time.Second) // Slow down brute force

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	code := r.FormValue("code")
	redirectURL := r.FormValue("redirect_url")

	ip := utils.GetIP(r)
	data := st.GetCode(ip)

	w.Header().Set("Content-Type", "text/html")

	if data == nil {
		w.WriteHeader(http.StatusUnauthorized)
		templates.LoginTmpl.Execute(w, templates.PageData{Error: "Code expired or not requested", RedirectURL: redirectURL})
		return
	}

	if data.Code != code {
		st.IncrementAttempts(ip)
		if data.Attempts > 5 {
			st.DeleteCode(ip)
			w.WriteHeader(http.StatusForbidden)
			templates.LoginTmpl.Execute(w, templates.PageData{Error: "Too many attempts", RedirectURL: redirectURL})
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		templates.VerifyTmpl.Execute(w, templates.PageData{Error: "Invalid code", RedirectURL: redirectURL})
		return
	}

	// Success
	sessionID := utils.GenerateSessionID()
	st.AddSession(sessionID, sessionDuration)
	st.DeleteCode(ip)

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    sessionID,
		Path:     "/",
		Expires:  time.Now().Add(sessionDuration),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	if redirectURL != "" {
		http.Redirect(w, r, redirectURL, http.StatusFound)
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Authenticated. You may now close this window."))
	}
}
