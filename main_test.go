package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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
	store = NewStore()
	config = Config{CookieName: "test_cookie"}

	// Case 1: No Cookie -> Redirect
	req := httptest.NewRequest("GET", "/", nil)
	// Add X-Forwarded headers as expected by our handler logic
	req.Header.Set("X-Forwarded-Host", "example.com")
	req.Header.Set("X-Forwarded-Proto", "https")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(authHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusFound {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusFound)
	}

	expectedLocation := "https://example.com/auth/login"
	if loc := rr.Header().Get("Location"); loc != expectedLocation {
		t.Errorf("handler returned wrong location: got %v want %v", loc, expectedLocation)
	}

	// Case 2: Valid Session -> 200 OK
	sessionID := "valid_session"
	store.AddSession(sessionID, 1*time.Minute)

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
	store = NewStore()
	config = Config{CodeExpiration: 1 * time.Minute}
	mockNotifier := &MockNotifier{}
	notifier = mockNotifier

	req := httptest.NewRequest("POST", "/auth/request-code", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(requestCodeHandler)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Verify code was generated and "sent"
	if mockNotifier.LastCode == "" {
		t.Error("Expected code to be sent via notifier")
	}

	// Verify stored code matches
	// Note: RemoteAddr is used for IP if headers missing. httptest default is 192.0.2.1:1234
	// We should probably check what IP the handler extracted.
	// In test it seems to extract "192.0.2.1" usually.
	// Let's check store values.
	// Since we don't know exact IP loop, let's just check if ANY code exists in store.
	// Actually store exposes map but it is mutex protected, we should use GetCode.
	// But we need the IP used.

	// Check mock notifier for IP
	ip := mockNotifier.LastIP
	found := store.GetCode(ip)
	if found == nil {
		t.Errorf("Code not found in store for ip %s", ip)
	} else {
		if found.Code != mockNotifier.LastCode {
			t.Errorf("Stored code %s does not match sent code %s", found.Code, mockNotifier.LastCode)
		}
	}
}

func TestVerifyCodeHandler(t *testing.T) {
	store = NewStore()
	config = Config{
		SessionDuration: 1 * time.Minute,
		CookieName:      "auth_cookie",
	}

	// Setup Code
	ip := "192.0.2.1" // Default remote addr IP in httptest
	code := "123456"
	store.SetCode(ip, code, 1*time.Minute)

	// Case 1: Valid Code
	reqBody, _ := json.Marshal(map[string]string{"code": code})
	req := httptest.NewRequest("POST", "/auth/verify-code", bytes.NewBuffer(reqBody))
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(verifyCodeHandler)

	// We use "RemoteAddr" in httptest which is mocked to 192.0.2.1:1234

	start := time.Now()
	handler.ServeHTTP(rr, req)
	duration := time.Since(start)

	if duration < 2*time.Second {
		t.Errorf("Handler executed too fast, expected delay ~2s, got %v", duration)
	}

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
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

	// Case 2: Invalid Code
	store.SetCode(ip, code, 1*time.Minute) // Reset code (it was deleted on success)

	reqBody, _ = json.Marshal(map[string]string{"code": "wrong"})
	req = httptest.NewRequest("POST", "/auth/verify-code", bytes.NewBuffer(reqBody))
	rr = httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code for invalid code: got %v want %v", status, http.StatusUnauthorized)
	}
}
