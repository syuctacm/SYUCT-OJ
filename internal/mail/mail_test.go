package mail

import (
	"context"
	"errors"
	"testing"
)

func TestManagerEnabledAndUpdate(t *testing.T) {
	manager := New(Config{})
	if manager.Enabled() || IsEnabled(manager) {
		t.Fatal("empty SMTP config must be disabled")
	}
	if err := manager.Send(context.Background(), "user@example.com", "subject", "body"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("disabled manager send error = %v", err)
	}

	manager.Update(Config{
		Host: "smtp.example.com", Port: 465, Username: "mailer",
		Password: "secret", From: "noreply@example.com", Secure: true,
	})
	if !manager.Enabled() || !IsEnabled(manager) {
		t.Fatal("configured manager must be enabled")
	}
	manager.Update(Config{Host: "smtp.example.com"})
	if manager.Enabled() {
		t.Fatal("SMTP without From must be disabled")
	}
}

func TestIsEnabledSupportsLegacySender(t *testing.T) {
	if !IsEnabled(fakeSender{}) {
		t.Fatal("legacy sender without Enabled method should remain enabled")
	}
}

type fakeSender struct{}

func (fakeSender) Send(context.Context, string, string, string) error { return nil }
