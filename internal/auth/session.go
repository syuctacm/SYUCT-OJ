package auth

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

type Session struct {
	UserID    int64
	Kind      string
	ExpiresAt time.Time
}

type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]Session
	ttl      time.Duration
	now      func() time.Time
}

func NewSessionManager(ttl time.Duration) *SessionManager {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &SessionManager{
		sessions: make(map[string]Session),
		ttl:      ttl,
		now:      time.Now,
	}
}

// Issue creates a judge-compatible session without a browser user identity.
func (s *SessionManager) Issue() (string, error) {
	return s.IssueFor(0, "judge")
}

func (s *SessionManager) IssueForUser(uid int64) (string, error) {
	return s.IssueFor(uid, "user")
}

func (s *SessionManager) IssueFor(uid int64, kind string) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	s.mu.Lock()
	s.sessions[token] = Session{
		UserID:    uid,
		Kind:      kind,
		ExpiresAt: s.now().Add(s.ttl),
	}
	s.mu.Unlock()
	return token, nil
}

func (s *SessionManager) Get(token string) (Session, bool) {
	if token == "" {
		return Session{}, false
	}
	s.mu.RLock()
	session, ok := s.sessions[token]
	s.mu.RUnlock()
	if !ok {
		return Session{}, false
	}
	if s.now().After(session.ExpiresAt) {
		s.mu.Lock()
		delete(s.sessions, token)
		s.mu.Unlock()
		return Session{}, false
	}
	return session, true
}

func (s *SessionManager) Validate(token string) bool {
	_, ok := s.Get(token)
	return ok
}

func (s *SessionManager) Revoke(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}
