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

// GenerateCode creates a numeric code of the specified length.
func GenerateCode(length int) string {
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(length)), nil)
	n, _ := rand.Int(rand.Reader, max)
	format := fmt.Sprintf("%%0%dd", length)
	return fmt.Sprintf(format, n)
}

// GenerateSessionID creates a secure random session ID.
func GenerateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
