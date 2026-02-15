package store

import (
	"testing"
	"time"
)

func TestStore_CodeLifecycle(t *testing.T) {
	store := NewStore()
	ip := "127.0.0.1"
	code := "123456"

	// 1. Set Code
	store.SetCode(ip, code, 1*time.Minute)

	// 2. Get Code
	data := store.GetCode(ip)
	if data == nil {
		t.Fatal("Expected code to be present")
	}
	if data.Code != code {
		t.Errorf("Expected code %s, got %s", code, data.Code)
	}

	// 3. Increment Attempts
	store.IncrementAttempts(ip)
	data = store.GetCode(ip)
	if data.Attempts != 1 {
		t.Errorf("Expected 1 attempt, got %d", data.Attempts)
	}

	// 4. Delete Code
	store.DeleteCode(ip)
	data = store.GetCode(ip)
	if data != nil {
		t.Error("Expected code to be deleted")
	}
}

func TestStore_CodeExpiration(t *testing.T) {
	store := NewStore()
	ip := "192.168.1.1"

	// Set exact minimal duration so we don't have to wait too long if logic fails
	store.SetCode(ip, "654321", 100*time.Millisecond)

	time.Sleep(200 * time.Millisecond)

	data := store.GetCode(ip)
	if data != nil {
		t.Error("Expected code to be expired")
	}
}

func TestStore_SessionLifecycle(t *testing.T) {
	store := NewStore()
	id := "session_abc"

	store.AddSession(id, 1*time.Minute)

	if !store.IsSessionValid(id) {
		t.Error("Expected session to be valid")
	}

	if store.IsSessionValid("invalid_id") {
		t.Error("Expected invalid session to be invalid")
	}
}

func TestStore_SessionExpiration(t *testing.T) {
	store := NewStore()
	id := "session_expired"

	store.AddSession(id, 100*time.Millisecond)
	time.Sleep(200 * time.Millisecond)

	if store.IsSessionValid(id) {
		t.Error("Expected session to be expired")
	}
}

func TestStore_Cleanup(t *testing.T) {
	store := NewStore()

	// Add expired items
	store.SetCode("ip1", "123", -1*time.Minute)
	store.AddSession("sess1", -1*time.Minute)

	// Add valid items
	store.SetCode("ip2", "456", 1*time.Minute)
	store.AddSession("sess2", 1*time.Minute)

	store.Cleanup()

	if store.GetCode("ip1") != nil {
		t.Error("Expired code should have been cleaned up")
	}
	if store.IsSessionValid("sess1") {
		t.Error("Expired session should have been cleaned up")
	}

	// Valid items should remain
	if store.GetCode("ip2") == nil {
		t.Error("Valid code should remain")
	}
	if !store.IsSessionValid("sess2") {
		t.Error("Valid session should remain")
	}
}
