package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lucasvieira/traefik-auth-code-middleware/internal/store"
)

// Mock Notifier
type MockNotifier struct {
	LastCode string
	LastIP   string
}

func (m *MockNotifier) SendCode(code, ip string) error {
	m.LastCode = code
	m.LastIP = ip
	return nil
}

func TestAuthHandler(t *testing.T) {
	// Setup
	st = store.NewStore()
	cookieName = "test_cookie"

	// Case 1: No Cookie -> 401 Unauthorized (HTML Login Page)
	req := httptest.NewRequest("GET", "/", nil)
	// Add X-Forwarded headers as expected by our handler logic
	req.Header.Set("X-Forwarded-Host", "example.com")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Uri", "/protected")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(authHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusUnauthorized)
	}

	if rr.Header().Get("Content-Type") != "text/html" {
		t.Errorf("handler returned wrong content type: got %v want text/html", rr.Header().Get("Content-Type"))
	}

	// Check if redirect_url is present in the HTML (naive check)
	if !bytes.Contains(rr.Body.Bytes(), []byte("https://example.com/protected")) {
		t.Error("HTML does not contain the expected redirect_url")
	}

	// Case 2: Valid Session -> 200 OK
	sessionID := "valid_session"
	st.AddSession(sessionID, 1*time.Minute)

	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-Host", "example.com")
	req.AddCookie(&http.Cookie{Name: "test_cookie", Value: sessionID})

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code for valid session: got %v want %v", status, http.StatusOK)
	}
}

func TestRequestCodeHandler(t *testing.T) {
	st = store.NewStore()
	codeExpiration = 1 * time.Minute
	codeLength = 6
	mockNotifier := &MockNotifier{}
	notifier = mockNotifier

	// Form Data
	req := httptest.NewRequest("POST", "/_auth_code/request-code", strings.NewReader("redirect_url=http://example.com"))
	req.Header.Set("X-Forwarded-For", "127.0.0.1") // Ensure IP is set for rate limiting check
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(requestCodeHandler)

	handler.ServeHTTP(rr, req)

	// It should verify success implies rendering the Verify Page (likely 200 OK with HTML)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	if rr.Header().Get("Content-Type") != "text/html" {
		t.Errorf("handler returned wrong content type: got %v want text/html", rr.Header().Get("Content-Type"))
	}

	// Verify code was generated and "sent"
	if mockNotifier.LastCode == "" {
		t.Error("Expected code to be sent via notifier")
	}

	// Check for correct maxlength in HTML
	if !strings.Contains(rr.Body.String(), `maxlength="6"`) {
		t.Errorf("HTML does not contain expected maxlength=\"6\"")
	}
}

func TestVerifyCodeHandler(t *testing.T) {
	st = store.NewStore()
	sessionDuration = 1 * time.Minute
	cookieName = "auth_cookie"

	// Setup Code
	ip := "192.0.2.1" // Default remote addr IP in httptest
	code := "123456"
	st.SetCode(ip, code, 1*time.Minute)

	// Case 1: Valid Code
	form := "code=" + code + "&redirect_url=http://example.com"
	req := httptest.NewRequest("POST", "/_auth_code/verify-code", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(verifyCodeHandler)

	start := time.Now()
	handler.ServeHTTP(rr, req)
	duration := time.Since(start)

	if duration < 2*time.Second {
		t.Errorf("Handler executed too fast, expected delay ~2s, got %v", duration)
	}

	// Expect Redirect 302
	if status := rr.Code; status != http.StatusFound {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusFound)
	}

	// Verify Cookie
	cookies := rr.Result().Cookies()
	var authCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "auth_cookie" {
			authCookie = c
			break
		}
	}
	if authCookie == nil {
		t.Error("Expected auth cookie to be set")
	}

	// Verify Location Header
	if loc := rr.Header().Get("Location"); loc != "http://example.com" {
		t.Errorf("Expected redirect to http://example.com, got %s", loc)
	}

	// Case 2: Invalid Code
	st.SetCode(ip, code, 1*time.Minute) // Reset code

	form = "code=wrong&redirect_url=http://example.com"
	req = httptest.NewRequest("POST", "/_auth_code/verify-code", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rr = httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should return 401 Unauthorized (Rendering Verify Page with Error)
	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code for invalid code: got %v want %v", status, http.StatusUnauthorized)
	}

	if rr.Header().Get("Content-Type") != "text/html" {
		t.Errorf("handler returned wrong content type: got %v want text/html", rr.Header().Get("Content-Type"))
	}
}
