package mailer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

type SMTP struct {
	addr     string
	host     string
	username string
	password string
	from     *mail.Address
}

// net/smtp upgrades with STARTTLS when the server offers it, so we use port
// 587 rather than 465 (implicit TLS isn't supported by smtp.SendMail).
func NewSMTP(host string, port int, username, password, from string) (*SMTP, error) {
	addr, err := mail.ParseAddress(from)
	if err != nil {
		return nil, fmt.Errorf("invalid SMTP_FROM %q: %w", from, err)
	}
	return &SMTP{
		addr:     net.JoinHostPort(host, strconv.Itoa(port)),
		host:     host,
		username: username,
		password: password,
		from:     addr,
	}, nil
}

func (m *SMTP) Send(_ context.Context, to, subject, body string) error {
	msg, err := buildMessage(m.from, to, subject, body, time.Now())
	if err != nil {
		return err
	}
	var auth smtp.Auth
	if m.username != "" {
		auth = smtp.PlainAuth("", m.username, m.password, m.host)
	}
	if err := smtp.SendMail(m.addr, auth, m.from.Address, []string{to}, msg); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	return nil
}

// Log is what we use locally when SMTP isn't configured.
type Log struct {
	logger *slog.Logger
}

func NewLog(logger *slog.Logger) *Log {
	return &Log{logger: logger}
}

func (m *Log) Send(_ context.Context, to, subject, body string) error {
	m.logger.Info("email not sent (SMTP_HOST not set)", "to", to, "subject", subject, "body", body)
	return nil
}

func buildMessage(from *mail.Address, to, subject, body string, now time.Time) ([]byte, error) {
	if strings.ContainsAny(to, "\r\n") || strings.ContainsAny(subject, "\r\n") {
		return nil, errors.New("mail header contains a line break")
	}

	var b strings.Builder
	b.WriteString("From: " + from.String() + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n")
	b.WriteString("Date: " + now.Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	b.WriteString("\r\n")
	b.WriteString(strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\n", "\r\n"))
	return []byte(b.String()), nil
}
