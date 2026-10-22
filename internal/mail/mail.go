package mail

import (
	"context"
	"fmt"
	"mime"
	"net/smtp"
	"strings"
)

type Sender interface {
	Send(ctx context.Context, to, subject, htmlBody string) error
}

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

type SMTPSender struct {
	host     string
	port     int
	username string
	password string
	from     string
}

func New(config Config) Sender {
	if strings.TrimSpace(config.Host) == "" || strings.TrimSpace(config.From) == "" {
		return nil
	}
	port := config.Port
	if port <= 0 {
		port = 587
	}
	return &SMTPSender{
		host: strings.TrimSpace(config.Host), port: port,
		username: strings.TrimSpace(config.Username), password: config.Password,
		from: strings.TrimSpace(config.From),
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
	return smtp.SendMail(fmt.Sprintf("%s:%d", s.host, s.port), auth, s.from, []string{to}, []byte(message))
}
