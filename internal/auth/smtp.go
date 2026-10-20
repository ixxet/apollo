package auth

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

type SMTPConfig struct {
	Host     string
	Port     int
	TLSMode  string
	From     string
	Username string
	Password string
	Timeout  time.Duration
}

type SMTPEmailSender struct {
	config    SMTPConfig
	from      *mail.Address
	tlsConfig *tls.Config
}

func NewSMTPEmailSender(config SMTPConfig) (*SMTPEmailSender, error) {
	if config.Host == "" || strings.ContainsAny(config.Host, "/\\@ \t\r\n") || (strings.Contains(config.Host, ":") && net.ParseIP(config.Host) == nil) {
		return nil, fmt.Errorf("APOLLO_SMTP_HOST must be a hostname or IP without a port")
	}
	if config.Port < 1 || config.Port > 65535 {
		return nil, fmt.Errorf("APOLLO_SMTP_PORT must be between 1 and 65535")
	}
	if config.TLSMode != "starttls" && config.TLSMode != "tls" {
		return nil, fmt.Errorf("APOLLO_SMTP_TLS_MODE must be starttls or tls")
	}
	if config.Timeout <= 0 || config.Timeout > 4*time.Second {
		return nil, fmt.Errorf("APOLLO_SMTP_TIMEOUT must be greater than zero and at most 4s")
	}
	from, err := mail.ParseAddress(config.From)
	if err != nil || strings.ContainsAny(config.From, "\r\n\x00") {
		return nil, fmt.Errorf("APOLLO_SMTP_FROM must be one valid sender address")
	}
	if (config.Username == "") != (config.Password == "") {
		return nil, fmt.Errorf("APOLLO_SMTP_USERNAME and APOLLO_SMTP_PASSWORD must both be set or both absent")
	}
	if strings.ContainsAny(config.Username+config.Password, "\r\n\x00") {
		return nil, fmt.Errorf("APOLLO SMTP credentials contain invalid control characters")
	}
	return &SMTPEmailSender{
		config: config,
		from:   from,
		tlsConfig: &tls.Config{
			ServerName: config.Host,
			MinVersion: tls.VersionTLS12,
		},
	}, nil
}

func (s *SMTPEmailSender) SendVerification(ctx context.Context, delivery VerificationDelivery) error {
	recipient, err := normalizeEmail(delivery.Email)
	if err != nil {
		return err
	}
	tokenBytes, err := base64.RawURLEncoding.DecodeString(delivery.Token)
	if err != nil || len(tokenBytes) != 32 {
		return fmt.Errorf("invalid SMTP verification code")
	}
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	failure := func(phase string) error {
		// SMTP servers may echo credentials, recipients or message content in
		// failures. Expose only the phase and context state to logs/HTTP callers.
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("SMTP %s: %w", phase, err)
		}
		if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
			return fmt.Errorf("SMTP %s: %w", phase, context.DeadlineExceeded)
		}
		return fmt.Errorf("SMTP %s failed", phase)
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port)))
	if err != nil {
		return failure("connection")
	}
	defer conn.Close()
	stopClose := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopClose()
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		return failure("deadline")
	}
	var transport net.Conn = conn
	if s.config.TLSMode == "tls" {
		encrypted := tls.Client(conn, s.tlsConfig.Clone())
		if err := encrypted.HandshakeContext(ctx); err != nil {
			return failure("TLS handshake")
		}
		transport = encrypted
	}
	client, err := smtp.NewClient(transport, s.config.Host)
	if err != nil {
		return failure("greeting")
	}
	defer client.Close()
	if s.config.TLSMode == "starttls" {
		if supported, _ := client.Extension("STARTTLS"); !supported {
			return fmt.Errorf("SMTP server must support STARTTLS")
		}
		if err := client.StartTLS(s.tlsConfig.Clone()); err != nil {
			return failure("STARTTLS")
		}
	}
	if s.config.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)); err != nil {
			return failure("authentication")
		}
	}
	if err := client.Mail(s.from.Address); err != nil {
		return failure("sender")
	}
	if err := client.Rcpt(recipient); err != nil {
		return failure("recipient")
	}
	writer, err := client.Data()
	if err != nil {
		return failure("message start")
	}
	message := "From: " + s.from.String() + "\r\nTo: " + (&mail.Address{Address: recipient}).String() + "\r\n" +
		"Subject: Your ASHTON sign-in code\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n" +
		"Content-Transfer-Encoding: 7bit\r\n\r\n" +
		"Return to the ASHTON sign-in page where you requested this email and paste this verification code:\r\n\r\n" +
		delivery.Token + "\r\n\r\nIf you did not request this code, ignore this email.\r\n"
	if _, err := writer.Write([]byte(message)); err != nil {
		return failure("message write")
	}
	if err := writer.Close(); err != nil {
		return failure("message acceptance")
	}
	// DATA's final 250 response establishes SMTP acceptance. A failed QUIT
	// afterwards must not turn accepted mail into an automatic retry.
	_ = client.Quit()
	return nil
}
