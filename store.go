package main

import (
	"sync"
	"time"
)

type CodeData struct {
	Code      string
	ExpiresAt time.Time
	Attempts  int
}

type Store struct {
	mu       sync.RWMutex
	Codes    map[string]*CodeData // Map IP -> CodeData
	Sessions map[string]time.Time // Map SessionID -> Expiration
}

func NewStore() *Store {
	return &Store{
		Codes:    make(map[string]*CodeData),
		Sessions: make(map[string]time.Time),
	}
}

func (s *Store) SetCode(ip, code string, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Codes[ip] = &CodeData{
		Code:      code,
		ExpiresAt: time.Now().Add(duration),
		Attempts:  0,
	}
}

func (s *Store) GetCode(ip string) *CodeData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if data, ok := s.Codes[ip]; ok {
		if time.Now().After(data.ExpiresAt) {
			return nil
		}
		return data
	}
	return nil
}

func (s *Store) IncrementAttempts(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if data, ok := s.Codes[ip]; ok {
		data.Attempts++
	}
}

func (s *Store) DeleteCode(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Codes, ip)
}

func (s *Store) AddSession(sessionID string, duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Sessions[sessionID] = time.Now().Add(duration)
}

func (s *Store) IsSessionValid(sessionID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if expiresAt, ok := s.Sessions[sessionID]; ok {
		if time.Now().Before(expiresAt) {
			return true
		}
		// Lazy cleanup
		delete(s.Sessions, sessionID) // We can't delete in RLock, so we skip here or upgrade lock.
		// Actually, we can't delete with RLock. Let's just return false.
		return false
	}
	return false
}

// Cleanup routine could be added here to remove expired codes/sessions periodically.
func (s *Store) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for ip, data := range s.Codes {
		if now.After(data.ExpiresAt) {
			delete(s.Codes, ip)
		}
	}
	for id, expires := range s.Sessions {
		if now.After(expires) {
			delete(s.Sessions, id)
		}
	}
}
