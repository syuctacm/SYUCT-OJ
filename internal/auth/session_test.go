package auth

import (
	"testing"
	"time"
)

func TestSessionManager(t *testing.T) {
	manager := NewSessionManager(time.Hour)
	token, err := manager.Issue()
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	if !manager.Validate(token) {
		t.Fatal("new session should be valid")
	}
	manager.Revoke(token)
	if manager.Validate(token) {
		t.Fatal("revoked session should be invalid")
	}
}

func TestSessionExpiry(t *testing.T) {
	manager := NewSessionManager(time.Hour)
	now := time.Unix(100, 0)
	manager.now = func() time.Time { return now }
	token, err := manager.Issue()
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	now = now.Add(2 * time.Hour)
	if manager.Validate(token) {
		t.Fatal("expired session should be invalid")
	}
}
