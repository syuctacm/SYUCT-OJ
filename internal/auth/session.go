package auth

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]time.Time
	ttl      time.Duration
	now      func() time.Time
}

func NewSessionManager(ttl time.Duration) *SessionManager {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &SessionManager{
		sessions: make(map[string]time.Time),
		ttl:      ttl,
		now:      time.Now,
	}
}

func (s *SessionManager) Issue() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	s.mu.Lock()
	s.sessions[token] = s.now().Add(s.ttl)
	s.mu.Unlock()
	return token, nil
}

func (s *SessionManager) Validate(token string) bool {
	if token == "" {
		return false
	}
	s.mu.RLock()
	expiresAt, ok := s.sessions[token]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	if s.now().After(expiresAt) {
		s.mu.Lock()
		delete(s.sessions, token)
		s.mu.Unlock()
		return false
	}
	return true
}

func (s *SessionManager) Revoke(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}
