package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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
// Note: For ForwardAuth, if we return 200, request proceeds.
// If we return 401/403, Traefik blocks.
// If we want to show a login page, we can either:
//  1. Redirect to an external auth service (classic forward auth).
//  2. Serve the login page directly on the unauthorized request path?
//     No, that would mess up the content type.
//     Better approach: Redirect to /auth/login.
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

	if forwardedHost == "" {
		// Fallback for internal testing or misconfig
		// Just return 401 Unauthorized
		http.Error(w, "Unauthorized (Missing X-Forwarded-Host)", http.StatusUnauthorized)
		return
	}

	if forwardedProto == "" {
		forwardedProto = "https"
	}

	// Redirect to /auth/login on the same host
	// redirectURL := fmt.Sprintf("%s://%s/auth/login", forwardedProto, forwardedHost)
	// http.Redirect(w, r, redirectURL, http.StatusFound)

	// Serve the login page directly with 401 status
	w.WriteHeader(http.StatusUnauthorized)
	w.Header().Set("Content-Type", "text/html")
	loginTmpl.Execute(w, nil)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	loginTmpl.Execute(w, nil)
}

func requestCodeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ip := getIP(r)

	// Create Code
	code := generateCode()
	store.SetCode(ip, code, config.CodeExpiration)

	// Send Code (Async or Sync? Sync is better for feedback)
	err := notifier.SendCode(code, ip)

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		log.Printf("Failed to send code to %s: %v", ip, err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to send notification"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Code sent"})
}

type VerifyRequest struct {
	Code string `json:"code"`
}

func verifyCodeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Artificial delay to prevent brute force
	time.Sleep(2 * time.Second)

	var req VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	ip := getIP(r)
	data := store.GetCode(ip)

	w.Header().Set("Content-Type", "application/json")

	if data == nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Code expired or not requested"})
		return
	}

	if data.Code != req.Code {
		store.IncrementAttempts(ip)
		// Potential: limit attempts
		if data.Attempts > 5 {
			store.DeleteCode(ip)
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "Too many attempts"})
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid code"})
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

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Authenticated"})
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
