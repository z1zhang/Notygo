package sender

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// EmailConfig holds the configuration for an SMTP email sender.
type EmailConfig struct {
	SMTPHost string   `json:"smtp_host"`
	SMTPPort int      `json:"smtp_port"`
	UseTLS   bool     `json:"use_tls"`
	From     string   `json:"from"`
	To       []string `json:"to"`
	CC       []string `json:"cc,omitempty"`
}

// EmailSender sends messages via SMTP.
type EmailSender struct{}

// NewEmailSender creates a new EmailSender.
func NewEmailSender() *EmailSender {
	return &EmailSender{}
}

// Send delivers a message via SMTP email.
// destConfig.SecretRef is used as the SMTP password for authentication.
func (s *EmailSender) Send(ctx context.Context, destConfig DestinationConfig, msg RenderedMessage) (*SendResult, error) {
	var cfg EmailConfig
	if err := json.Unmarshal(destConfig.Config, &cfg); err != nil {
		return nil, fmt.Errorf("invalid email config: %w", err)
	}

	if cfg.SMTPHost == "" {
		return nil, fmt.Errorf("smtp_host is required")
	}
	if cfg.SMTPPort == 0 {
		cfg.SMTPPort = 465
	}
	if cfg.From == "" {
		return nil, fmt.Errorf("from address is required")
	}
	if len(cfg.To) == 0 {
		return nil, fmt.Errorf("at least one recipient is required")
	}

	// Build email message (RFC 2822)
	var buf bytes.Buffer
	buf.WriteString("From: " + cfg.From + "\r\n")
	buf.WriteString("To: " + strings.Join(cfg.To, ", ") + "\r\n")
	if len(cfg.CC) > 0 {
		buf.WriteString("Cc: " + strings.Join(cfg.CC, ", ") + "\r\n")
	}
	buf.WriteString("Subject: " + mimeEncodeSubject(msg.Title) + "\r\n")

	contentType := "text/plain; charset=UTF-8"
	if msg.Format == "html" {
		contentType = "text/html; charset=UTF-8"
	}
	buf.WriteString("Content-Type: " + contentType + "\r\n")
	buf.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(msg.Body)

	// Collect all recipients (To + CC)
	var recipients []string
	recipients = append(recipients, cfg.To...)
	recipients = append(recipients, cfg.CC...)

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	password := destConfig.SecretRef // decrypted SMTP password

	timeout := destConfig.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	result := &SendResult{}

	var err error
	if cfg.UseTLS {
		err = s.sendTLS(addr, cfg.SMTPHost, password, cfg.From, recipients, buf.Bytes(), timeout)
	} else {
		err = s.sendSTARTTLS(addr, cfg.SMTPHost, password, cfg.From, recipients, buf.Bytes(), timeout)
	}

	if err != nil {
		result.IsRetryable = isSMTPRetryable(err)
		return result, err
	}

	result.StatusCode = 250 // SMTP OK
	return result, nil
}

// sendTLS sends email via direct TLS (SMTPS, typically port 465).
func (s *EmailSender) sendTLS(addr, host, password, from string, recipients []string, msg []byte, timeout time.Duration) error {
	tlsConfig := &tls.Config{ServerName: host}

	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: timeout},
		"tcp",
		addr,
		tlsConfig,
	)
	if err != nil {
		return fmt.Errorf("TLS dial %s: %w", addr, err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("SMTP client: %w", err)
	}
	defer client.Close()

	return mailTransaction(client, host, password, from, recipients, msg)
}

// sendSTARTTLS sends email via STARTTLS upgrade (typically port 587).
func (s *EmailSender) sendSTARTTLS(addr, host, password, from string, recipients []string, msg []byte, timeout time.Duration) error {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("SMTP client: %w", err)
	}
	defer client.Close()

	// Attempt STARTTLS upgrade
	tlsConfig := &tls.Config{ServerName: host}
	if err := client.StartTLS(tlsConfig); err != nil {
		// STARTTLS not supported; continue with plain SMTP
	}

	return mailTransaction(client, host, password, from, recipients, msg)
}

// mailTransaction performs the SMTP AUTH/MAIL/RCPT/DATA transaction.
func mailTransaction(client *smtp.Client, host, password, from string, recipients []string, msg []byte) error {
	if err := client.Hello("localhost"); err != nil {
		return fmt.Errorf("HELO: %w", err)
	}

	// Authenticate if password is provided
	if password != "" {
		auth := smtp.PlainAuth("", extractAddress(from), password, host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("AUTH: %w", err)
		}
	}

	if err := client.Mail(extractAddress(from)); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}

	for _, rcpt := range recipients {
		if err := client.Rcpt(extractAddress(rcpt)); err != nil {
			return fmt.Errorf("RCPT TO %s: %w", rcpt, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close DATA: %w", err)
	}

	return client.Quit()
}

// extractAddress extracts the bare email address from "Name <addr>" or plain "addr" format.
func extractAddress(s string) string {
	if addr, err := mail.ParseAddress(s); err == nil {
		return addr.Address
	}
	return strings.TrimSpace(s)
}

// mimeEncodeSubject encodes a subject line for non-ASCII characters (RFC 2047).
func mimeEncodeSubject(subject string) string {
	for _, r := range subject {
		if r > 127 {
			return fmt.Sprintf("=?UTF-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(subject)))
		}
	}
	return subject
}

// isSMTPRetryable determines if an SMTP error is retryable (4xx codes).
func isSMTPRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	retryableCodes := []string{"421", "450", "451", "452"}
	for _, code := range retryableCodes {
		if strings.Contains(msg, code) {
			return true
		}
	}
	return false
}
