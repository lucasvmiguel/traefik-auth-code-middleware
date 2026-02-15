package store

import (
	"sync"
	"time"
)

// Store holds memory data for codes and sessions.
type Store struct {
	mu       sync.RWMutex
	codes    map[string]*CodeData
	sessions map[string]time.Time
}

type CodeData struct {
	Code      string
	ExpiresAt time.Time
	Attempts  int
}

func NewStore() *Store {
	return &Store{
		codes:    make(map[string]*CodeData),
		sessions: make(map[string]time.Time),
	}
}

// Cleanup removes expired codes and sessions.
func (s *Store) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for ip, data := range s.codes {
		if now.After(data.ExpiresAt) {
			delete(s.codes, ip)
		}
	}
	for id, expires := range s.sessions {
		if now.After(expires) {
			delete(s.sessions, id)
		}
	}
}

// SetCode stores a generated code for an IP.
func (s *Store) SetCode(ip, code string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.codes[ip] = &CodeData{
		Code:      code,
		ExpiresAt: time.Now().Add(ttl),
		Attempts:  0,
	}
}

// GetCode retrieves code data for an IP.
func (s *Store) GetCode(ip string) *CodeData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if data, ok := s.codes[ip]; ok {
		if time.Now().Before(data.ExpiresAt) {
			return data
		}
	}
	return nil
}

// IncrementAttempts increases attempt count for an IP.
func (s *Store) IncrementAttempts(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if data, ok := s.codes[ip]; ok {
		data.Attempts++
	}
}

// DeleteCode removes code data for an IP.
func (s *Store) DeleteCode(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.codes, ip)
}

// AddSession stores a valid session ID.
func (s *Store) AddSession(id string, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[id] = time.Now().Add(ttl)
}

// IsSessionValid checks if a session ID exists and is not expired.
func (s *Store) IsSessionValid(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if expires, ok := s.sessions[id]; ok {
		return time.Now().Before(expires)
	}
	return false
}
