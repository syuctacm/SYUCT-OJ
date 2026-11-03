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

func TestSessionManagerSeparateTTLs(t *testing.T) {
	manager := NewSessionManager(time.Hour)
	manager.SetTTLs(2*time.Hour, 5*time.Minute)
	now := time.Unix(100, 0)
	manager.now = func() time.Time { return now }

	userToken, err := manager.IssueForUser(7)
	if err != nil {
		t.Fatal(err)
	}
	judgeToken, err := manager.Issue()
	if err != nil {
		t.Fatal(err)
	}
	userSession, ok := manager.Get(userToken)
	if !ok || userSession.ExpiresAt.Sub(now) != 2*time.Hour {
		t.Fatalf("user session = %+v", userSession)
	}
	judgeSession, ok := manager.Get(judgeToken)
	if !ok || judgeSession.ExpiresAt.Sub(now) != 5*time.Minute {
		t.Fatalf("judge session = %+v", judgeSession)
	}
	if manager.TTLFor("user") != 2*time.Hour || manager.TTLFor("judge") != 5*time.Minute {
		t.Fatalf("TTLFor = user %s judge %s", manager.TTLFor("user"), manager.TTLFor("judge"))
	}
}
