package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"sync"
)

var ErrNotConfigured = errors.New("mail transport is not configured")

type Sender interface {
	Send(ctx context.Context, to, subject, htmlBody string) error
}

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	Secure   bool
}

type SMTPSender struct {
	host     string
	port     int
	username string
	password string
	from     string
	secure   bool
}

type Manager struct {
	mu     sync.RWMutex
	sender Sender
}

func New(config Config) *Manager {
	manager := &Manager{}
	manager.Update(config)
	return manager
}

func (m *Manager) Update(config Config) {
	next := newSender(config)
	m.mu.Lock()
	m.sender = next
	m.mu.Unlock()
}

func (m *Manager) Enabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sender != nil
}

func (m *Manager) Send(ctx context.Context, to, subject, htmlBody string) error {
	m.mu.RLock()
	sender := m.sender
	m.mu.RUnlock()
	if sender == nil {
		return ErrNotConfigured
	}
	return sender.Send(ctx, to, subject, htmlBody)
}

func IsEnabled(sender Sender) bool {
	if sender == nil {
		return false
	}
	if availability, ok := sender.(interface{ Enabled() bool }); ok {
		return availability.Enabled()
	}
	return true
}

func IsConfigured(config Config) bool {
	return strings.TrimSpace(config.Host) != "" && strings.TrimSpace(config.From) != ""
}

func newSender(config Config) Sender {
	if !IsConfigured(config) {
		return nil
	}
	port := config.Port
	if port <= 0 {
		port = 587
	}
	return &SMTPSender{
		host: strings.TrimSpace(config.Host), port: port,
		username: strings.TrimSpace(config.Username), password: config.Password,
		from: strings.TrimSpace(config.From), secure: config.Secure,
	}
}

func (s *SMTPSender) Send(ctx context.Context, to, subject, htmlBody string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var auth smtp.Auth
	if s.username != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}
	headers := []string{
		"From: " + s.from,
		"To: " + to,
		"Subject: " + mime.QEncoding.Encode("UTF-8", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=UTF-8",
	}
	message := strings.Join(headers, "\r\n") + "\r\n\r\n" + htmlBody
	if s.secure {
		dialer := &tls.Dialer{
			NetDialer: &net.Dialer{},
			Config:    &tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12},
		}
		connection, err := dialer.DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", s.host, s.port))
		if err != nil {
			return err
		}
		defer connection.Close()
		client, err := smtp.NewClient(connection, s.host)
		if err != nil {
			return err
		}
		defer client.Close()
		if auth != nil {
			if err := client.Auth(auth); err != nil {
				return err
			}
		}
		if err := client.Mail(s.from); err != nil {
			return err
		}
		if err := client.Rcpt(to); err != nil {
			return err
		}
		writer, err := client.Data()
		if err != nil {
			return err
		}
		if _, err := writer.Write([]byte(message)); err != nil {
			_ = writer.Close()
			return err
		}
		if err := writer.Close(); err != nil {
			return err
		}
		return client.Quit()
	}
	return smtp.SendMail(fmt.Sprintf("%s:%d", s.host, s.port), auth, s.from, []string{to}, []byte(message))
}
