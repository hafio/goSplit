// Package mail sends transactional email. It exposes a Mailer interface with an
// SMTP implementation and a log-only fallback used when SMTP is unconfigured
// (so magic-link/reset flows still work in local dev — the link is logged).
package mail

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/hafio/gosplit/internal/config"
)

// Mailer sends a plain-text email.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// New returns an SMTP mailer when EMAIL_SERVER_HOST is set, otherwise a
// LogMailer that records messages via slog (dev fallback).
func New(cfg *config.Config) Mailer {
	if strings.TrimSpace(cfg.EmailServerHost) == "" {
		slog.Warn("mail: SMTP not configured (EMAIL_SERVER_HOST empty), using log-only mailer — magic-link/reset/invite bodies are printed to the logs")
		return &LogMailer{From: cfg.FromEmail}
	}
	slog.Info("mail: SMTP configured",
		"host", cfg.EmailServerHost, "port", cfg.EmailServerPort,
		"from", cfg.FromEmail, "auth", cfg.EmailServerUser != "")
	return &SMTPMailer{cfg: cfg}
}

// LogMailer writes emails to the log instead of sending them.
type LogMailer struct{ From string }

// Send logs the message.
func (m *LogMailer) Send(_ context.Context, to, subject, body string) error {
	slog.Info("mail (log-only)", "to", to, "subject", subject, "body", body)
	return nil
}

// SMTPMailer sends via SMTP with optional STARTTLS auth.
type SMTPMailer struct{ cfg *config.Config }

// dial establishes an SMTP client, using implicit TLS on port 465 (SMTPS) and
// STARTTLS on every other port when the server advertises it.
func (m *SMTPMailer) dial(addr string, tlsCfg *tls.Config) (*smtp.Client, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	var conn net.Conn
	var err error
	if m.cfg.EmailServerPort == 465 {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsCfg)
	} else {
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return nil, fmt.Errorf("mail: dial %s: %w", addr, err)
	}
	c, err := smtp.NewClient(conn, m.cfg.EmailServerHost)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("mail: smtp client: %w", err)
	}
	if m.cfg.EmailServerPort != 465 {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(tlsCfg); err != nil {
				_ = c.Close()
				return nil, fmt.Errorf("mail: starttls: %w", err)
			}
		}
	}
	return c, nil
}

// Send delivers a plain-text message over SMTP.
func (m *SMTPMailer) Send(_ context.Context, to, subject, body string) error {
	addr := net.JoinHostPort(m.cfg.EmailServerHost, strconv.Itoa(m.cfg.EmailServerPort))
	msg := buildMessage(m.cfg.FromEmail, to, subject, body)
	slog.Debug("mail: sending", "to", to, "subject", subject, "server", addr)

	tlsCfg := &tls.Config{
		ServerName:         m.cfg.EmailServerHost,
		InsecureSkipVerify: !m.cfg.EmailTLSRejectUnauthorized,
	}
	c, err := m.dial(addr, tlsCfg)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close() }()

	if m.cfg.EmailServerUser != "" {
		auth := smtp.PlainAuth("", m.cfg.EmailServerUser, m.cfg.EmailServerPassword, m.cfg.EmailServerHost)
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("mail: auth: %w", err)
		}
	}
	if err := c.Mail(m.cfg.FromEmail); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	wc, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := wc.Write([]byte(msg)); err != nil {
		return err
	}
	if err := wc.Close(); err != nil {
		return err
	}
	if err := c.Quit(); err != nil {
		return err
	}
	slog.Debug("mail: sent", "to", to, "subject", subject)
	return nil
}

func buildMessage(from, to, subject, body string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(body)
	return b.String()
}
