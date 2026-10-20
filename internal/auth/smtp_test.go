package auth

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ixxet/apollo/internal/store"
)

type smtpFixtureResult struct {
	commands      []string
	message       string
	authenticated bool
	authBeforeTLS bool
	err           error
}

type smtpFixtureOptions struct {
	mode          string
	noSTARTTLS    bool
	stallGreeting bool
	authFailure   bool
	dataFailure   bool
	dropQuit      bool
}

func smtpFixture(t *testing.T, options smtpFixtureOptions) (SMTPConfig, *x509.CertPool, <-chan smtpFixtureResult) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "SMTP fixture"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true, IsCA: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(certificate)
	serverTLS := &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, MinVersion: tls.VersionTLS12}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	results := make(chan smtpFixtureResult, 1)
	go func() {
		result := smtpFixtureResult{}
		defer func() { results <- result }()
		conn, err := listener.Accept()
		if err != nil {
			result.err = err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		encrypted := options.mode == "tls"
		if encrypted {
			secured := tls.Server(conn, serverTLS)
			if err := secured.Handshake(); err != nil {
				return // Expected for certificate-refusal tests.
			}
			conn = secured
		}
		if options.stallGreeting {
			_, _ = conn.Read(make([]byte, 1))
			return
		}
		reader, writer := bufio.NewReader(conn), bufio.NewWriter(conn)
		write := func(value string) { _, _ = writer.WriteString(value + "\r\n"); _ = writer.Flush() }
		write("220 fixture ESMTP")
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)
			command := strings.SplitN(line, " ", 2)[0]
			result.commands = append(result.commands, command)
			switch command {
			case "EHLO":
				write("250-fixture")
				if !encrypted && !options.noSTARTTLS {
					write("250-STARTTLS")
				}
				write("250 AUTH PLAIN")
			case "STARTTLS":
				write("220 begin TLS")
				secured := tls.Server(conn, serverTLS)
				if err := secured.Handshake(); err != nil {
					return
				}
				conn = secured
				encrypted = true
				reader, writer = bufio.NewReader(conn), bufio.NewWriter(conn)
			case "AUTH":
				result.authBeforeTLS = !encrypted
				parts := strings.Fields(line)
				if len(parts) != 3 {
					result.err = fmt.Errorf("unexpected AUTH shape")
					return
				}
				decoded, _ := base64.StdEncoding.DecodeString(parts[2])
				result.authenticated = string(decoded) == "\x00fixture-user\x00fixture-password"
				if options.authFailure {
					write("535 refused fixture-password private-provider-detail")
				} else {
					write("235 authenticated")
				}
			case "MAIL", "RCPT":
				write("250 accepted")
			case "DATA":
				write("354 send message")
				body, err := textproto.NewReader(reader).ReadDotBytes()
				if err != nil {
					result.err = err
					return
				}
				result.message = string(body)
				if options.dataFailure {
					write("550 private-provider-detail")
				} else {
					write("250 queued")
				}
			case "QUIT":
				if !options.dropQuit {
					write("221 goodbye")
				}
				return
			case "*":
				write("501 authentication cancelled")
			default:
				result.err = fmt.Errorf("unexpected SMTP command %s", command)
				return
			}
		}
	}()
	return SMTPConfig{Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, TLSMode: options.mode, From: "ASHTON <signin@example.invalid>", Username: "fixture-user", Password: "fixture-password", Timeout: time.Second}, roots, results
}

func fixtureDelivery() VerificationDelivery {
	return VerificationDelivery{StudentID: "fixture-staff", Email: "staff@example.invalid", Token: base64.RawURLEncoding.EncodeToString(make([]byte, 32))}
}

func awaitSMTPResult(t *testing.T, results <-chan smtpFixtureResult) smtpFixtureResult {
	t.Helper()
	select {
	case result := <-results:
		if result.err != nil {
			t.Fatal(result.err)
		}
		return result
	case <-time.After(6 * time.Second):
		t.Fatal("SMTP fixture did not finish")
		return smtpFixtureResult{}
	}
}

func TestSMTPVerificationEncryptedDelivery(t *testing.T) {
	for _, mode := range []string{"starttls", "tls"} {
		t.Run(mode, func(t *testing.T) {
			cfg, roots, results := smtpFixture(t, smtpFixtureOptions{mode: mode, dropQuit: true})
			sender, err := NewSMTPEmailSender(cfg)
			if err != nil {
				t.Fatal(err)
			}
			sender.tlsConfig.RootCAs = roots // Trust only this disposable fixture CA.
			if err := sender.SendVerification(context.Background(), fixtureDelivery()); err != nil {
				t.Fatal(err)
			}
			result := awaitSMTPResult(t, results)
			if !result.authenticated || result.authBeforeTLS {
				t.Fatal("authentication was missing or preceded TLS")
			}
			for _, required := range []string{"From: \"ASHTON\" <signin@example.invalid>", "To: <staff@example.invalid>", "Subject: Your ASHTON sign-in code", fixtureDelivery().Token} {
				if !strings.Contains(result.message, required) {
					t.Fatalf("message missing expected %s", strings.SplitN(required, ":", 2)[0])
				}
			}
			if strings.Contains(result.message, fixtureDelivery().StudentID) {
				t.Fatal("message must not disclose student identity")
			}
		})
	}
}

func TestVerificationServiceSendsItsGeneratedCodeThroughSMTP(t *testing.T) {
	cfg, roots, results := smtpFixture(t, smtpFixtureOptions{mode: "starttls"})
	sender, err := NewSMTPEmailSender(cfg)
	if err != nil {
		t.Fatal(err)
	}
	sender.tlsConfig.RootCAs = roots
	var storedHash string
	service := NewService(stubStore{
		startVerificationFunc: func(_ context.Context, _, _, _, tokenHash string, _ time.Time) (*store.ApolloUser, *store.ApolloEmailVerificationToken, error) {
			storedHash = tokenHash
			return &store.ApolloUser{}, &store.ApolloEmailVerificationToken{}, nil
		},
	}, mustCookieManager(t), sender, 15*time.Minute, time.Hour)
	if err := service.StartVerification(context.Background(), StartVerificationInput{StudentID: "fixture-staff", Email: "staff@example.invalid"}); err != nil {
		t.Fatal(err)
	}
	message := awaitSMTPResult(t, results).message
	found := false
	for _, line := range strings.Split(message, "\n") {
		if hashVerificationToken(strings.TrimSpace(line)) == storedHash {
			found = true
		}
	}
	if storedHash == "" || !found {
		t.Fatal("SMTP did not receive the code corresponding to the stored verification hash")
	}
}

func TestSMTPRefusesMissingTLSAndUntrustedCertificates(t *testing.T) {
	for _, testCase := range []struct {
		name string
		opts smtpFixtureOptions
	}{
		{"STARTTLS absent", smtpFixtureOptions{mode: "starttls", noSTARTTLS: true}},
		{"STARTTLS untrusted certificate", smtpFixtureOptions{mode: "starttls"}},
		{"implicit TLS untrusted certificate", smtpFixtureOptions{mode: "tls"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			cfg, _, results := smtpFixture(t, testCase.opts)
			sender, err := NewSMTPEmailSender(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := sender.SendVerification(context.Background(), fixtureDelivery()); err == nil {
				t.Fatal("expected secure transport refusal")
			}
			result := awaitSMTPResult(t, results)
			for _, command := range result.commands {
				if command == "AUTH" || command == "MAIL" || command == "RCPT" || command == "DATA" {
					t.Fatalf("sent %s before a trusted encrypted connection", command)
				}
			}
		})
	}
}

func TestSMTPDeadlineAndCancellationStopStalledGreeting(t *testing.T) {
	for _, cancelRequest := range []bool{false, true} {
		t.Run(strconv.FormatBool(cancelRequest), func(t *testing.T) {
			cfg, _, results := smtpFixture(t, smtpFixtureOptions{mode: "starttls", stallGreeting: true})
			cfg.Timeout = 150 * time.Millisecond
			sender, err := NewSMTPEmailSender(cfg)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if cancelRequest {
				timer := time.AfterFunc(30*time.Millisecond, cancel)
				defer timer.Stop()
			}
			started := time.Now()
			err = sender.SendVerification(ctx, fixtureDelivery())
			want := context.DeadlineExceeded
			if cancelRequest {
				want = context.Canceled
			}
			if !errors.Is(err, want) || time.Since(started) > time.Second {
				t.Fatalf("SMTP did not stop promptly with %v: %v", want, err)
			}
			awaitSMTPResult(t, results)
		})
	}
}

func TestSMTPFailureResponsesDoNotLeakProviderDetails(t *testing.T) {
	for _, options := range []smtpFixtureOptions{{mode: "starttls", authFailure: true}, {mode: "tls", dataFailure: true}} {
		cfg, roots, results := smtpFixture(t, options)
		sender, err := NewSMTPEmailSender(cfg)
		if err != nil {
			t.Fatal(err)
		}
		sender.tlsConfig.RootCAs = roots
		err = sender.SendVerification(context.Background(), fixtureDelivery())
		if err == nil || strings.Contains(err.Error(), "private-provider-detail") || strings.Contains(err.Error(), cfg.Password) || strings.Contains(err.Error(), fixtureDelivery().Token) {
			t.Fatalf("expected redacted SMTP error, got %v", err)
		}
		awaitSMTPResult(t, results)
	}
}

func TestSMTPRejectsInvalidConfigAndDelivery(t *testing.T) {
	valid := SMTPConfig{Host: "smtp.example.invalid", Port: 587, TLSMode: "starttls", From: "signin@example.invalid", Timeout: time.Second}
	for _, mutate := range []func(*SMTPConfig){
		func(c *SMTPConfig) { c.Host = "smtp.example.invalid:587" },
		func(c *SMTPConfig) { c.Port = 0 },
		func(c *SMTPConfig) { c.TLSMode = "plain" },
		func(c *SMTPConfig) { c.Timeout = 5 * time.Second },
		func(c *SMTPConfig) { c.From = "signin@example.invalid\r\nBcc: other@example.invalid" },
		func(c *SMTPConfig) { c.Username = "one-sided" },
		func(c *SMTPConfig) { c.Username, c.Password = "user", "password\x00" },
	} {
		cfg := valid
		mutate(&cfg)
		if _, err := NewSMTPEmailSender(cfg); err == nil {
			t.Fatal("invalid SMTP configuration accepted")
		}
	}
	sender, err := NewSMTPEmailSender(valid)
	if err != nil {
		t.Fatal(err)
	}
	for _, delivery := range []VerificationDelivery{
		{Email: "staff@example.invalid\r\nBcc: other@example.invalid", Token: fixtureDelivery().Token},
		{Email: "staff@example.invalid", Token: "\r\nInjected"},
	} {
		if err := sender.SendVerification(context.Background(), delivery); err == nil || strings.Contains(err.Error(), "connection") {
			t.Fatal("invalid delivery reached the network")
		}
	}
}
