package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"strings"
)

// GetIP extracts the user's real IP address from the request.
// It checks X-Real-Ip, X-Forwarded-For, and finally RemoteAddr.
func GetIP(r *http.Request) string {
	if xrip := r.Header.Get("X-Real-Ip"); xrip != "" {
		return xrip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

// GenerateCode creates a 6-digit numeric code for authentication.
func GenerateCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return fmt.Sprintf("%06d", n)
}

// GenerateSessionID creates a secure random session ID.
func GenerateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
