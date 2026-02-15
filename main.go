package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"strings"
	"time"
)

var (
	config   Config
	store    *Store
	notifier Notifier
)

func main() {
	config = LoadConfig()
	config.Validate()

	store = NewStore()
	notifier = NewNotifier(config)

	// Background cleanup
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			store.Cleanup()
		}
	}()

	mux := http.NewServeMux()

	// The main ForwardAuth handler
	mux.HandleFunc("/", authHandler)

	// Auth flows
	mux.HandleFunc("/auth/login", loginHandler)
	mux.HandleFunc("/auth/request-code", requestCodeHandler)
	mux.HandleFunc("/auth/verify-code", verifyCodeHandler)

	addr := ":" + config.Port
	log.Printf("Starting middleware on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

// authHandler checks for session cookie.
// If valid -> 200 OK (Traefik lets request through).
// If invalid -> serves the login page directly (or redirects to it).
func authHandler(w http.ResponseWriter, r *http.Request) {
	// 0. Whitelist /auth/ paths to prevent infinite redirect loops.
	// When valid, Traefik will forward the request to the router handling /auth/
	if uri := r.Header.Get("X-Forwarded-Uri"); strings.HasPrefix(uri, "/auth/") {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 1. Check Cookie.
	cookie, err := r.Cookie(config.CookieName)
	if err == nil && store.IsSessionValid(cookie.Value) {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 2. Invalid session -> Redirect to login.

	// Helper to handle redirect URL construction
	// We rely on X-Forwarded-Host to validly redirect the user.
	// If missing, we can't properly redirect to the correct public URL.
	forwardedHost := r.Header.Get("X-Forwarded-Host")
	forwardedProto := r.Header.Get("X-Forwarded-Proto")
	forwardedUri := r.Header.Get("X-Forwarded-Uri")

	if forwardedHost == "" {
		// Fallback for internal testing or misconfig
		// Just return 401 Unauthorized
		http.Error(w, "Unauthorized (Missing X-Forwarded-Host)", http.StatusUnauthorized)
		return
	}

	if forwardedProto == "" {
		forwardedProto = "https"
	}

	redirectURL := fmt.Sprintf("%s://%s%s", forwardedProto, forwardedHost, forwardedUri)

	// Serve the login page directly with 401 status
	w.WriteHeader(http.StatusUnauthorized)
	w.Header().Set("Content-Type", "text/html")
	loginTmpl.Execute(w, PageData{RedirectURL: redirectURL})
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	redirectURL := r.URL.Query().Get("redirect_url")
	w.Header().Set("Content-Type", "text/html")
	loginTmpl.Execute(w, PageData{RedirectURL: redirectURL})
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

	ip := getIP(r)

	// Create Code
	code := generateCode()
	store.SetCode(ip, code, config.CodeExpiration)

	// Send Code (Async or Sync? Sync is better for feedback)
	err := notifier.SendCode(code, ip)

	w.Header().Set("Content-Type", "text/html")
	if err != nil {
		log.Printf("Failed to send code to %s: %v", ip, err)
		w.WriteHeader(http.StatusInternalServerError)
		loginTmpl.Execute(w, PageData{Error: "Failed to send notification", RedirectURL: redirectURL})
		return
	}

	// Render Verify Page
	verifyTmpl.Execute(w, PageData{Message: "Code sent!", RedirectURL: redirectURL})
}

func verifyCodeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Artificial delay to prevent brute force
	time.Sleep(2 * time.Second)

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	code := r.FormValue("code")
	redirectURL := r.FormValue("redirect_url")

	ip := getIP(r)
	data := store.GetCode(ip)

	w.Header().Set("Content-Type", "text/html")

	if data == nil {
		w.WriteHeader(http.StatusUnauthorized)
		// Return to login with error
		loginTmpl.Execute(w, PageData{Error: "Code expired or not requested", RedirectURL: redirectURL})
		return
	}

	if data.Code != code {
		store.IncrementAttempts(ip)
		// Potential: limit attempts
		if data.Attempts > 5 {
			store.DeleteCode(ip)
			w.WriteHeader(http.StatusForbidden)
			loginTmpl.Execute(w, PageData{Error: "Too many attempts", RedirectURL: redirectURL})
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		verifyTmpl.Execute(w, PageData{Error: "Invalid code", RedirectURL: redirectURL})
		return
	}

	// Success!
	sessionID := generateSessionID()
	store.AddSession(sessionID, config.SessionDuration)
	store.DeleteCode(ip) // Consume code

	// Set Cookie
	http.SetCookie(w, &http.Cookie{
		Name:     config.CookieName,
		Value:    sessionID,
		Path:     "/",
		Expires:  time.Now().Add(config.SessionDuration),
		HttpOnly: true,
		Secure:   true, // Assuming https
		SameSite: http.SameSiteLaxMode,
	})

	if redirectURL != "" {
		http.Redirect(w, r, redirectURL, http.StatusFound)
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Authenticated. You may now close this window."))
	}
}

// Helpers

func getIP(r *http.Request) string {
	// If behind Traefik, X-Forwarded-For is reliable if strictly configured.
	// But Traefik also sets X-Real-Ip.
	if xrip := r.Header.Get("X-Real-Ip"); xrip != "" {
		return xrip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// First IP in list
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

func generateCode() string {
	// 6 digit numeric code
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return fmt.Sprintf("%06d", n)
}

func generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
