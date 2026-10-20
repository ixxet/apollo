package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr                string
	DatabaseURL             string
	NATSURL                 string
	SessionCookieName       string
	SessionCookieSecret     string
	SessionCookieSecure     bool
	VerificationTokenTTL    time.Duration
	SessionTTL              time.Duration
	LogVerificationTokens   bool
	SMTPHost                string
	SMTPPort                int
	SMTPTLSMode             string
	SMTPFrom                string
	SMTPUsername            string
	SMTPPassword            string
	SMTPTimeout             time.Duration
	AthenaBaseURL           string
	AthenaTimeout           time.Duration
	AthenaInternalReadToken string
	OpsAnalyticsMaxWindow   time.Duration
}

func Load() (Config, error) {
	verificationTokenTTL, err := getDurationEnv("APOLLO_VERIFICATION_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	sessionTTL, err := getDurationEnv("APOLLO_SESSION_TTL", 7*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	sessionCookieSecure, err := getBoolEnv("APOLLO_SESSION_COOKIE_SECURE", true)
	if err != nil {
		return Config{}, err
	}
	logVerificationTokens, err := getBoolEnv("APOLLO_LOG_VERIFICATION_TOKENS", false)
	if err != nil {
		return Config{}, err
	}
	smtpMode := getEnv("APOLLO_SMTP_TLS_MODE", "starttls")
	defaultSMTPPort := "587"
	if smtpMode == "tls" {
		defaultSMTPPort = "465"
	}
	smtpPort, err := strconv.Atoi(getEnv("APOLLO_SMTP_PORT", defaultSMTPPort))
	if err != nil || smtpPort < 1 || smtpPort > 65535 {
		return Config{}, fmt.Errorf("APOLLO_SMTP_PORT must be between 1 and 65535")
	}
	smtpTimeout, err := getDurationEnv("APOLLO_SMTP_TIMEOUT", 4*time.Second)
	if err != nil {
		return Config{}, err
	}
	if smtpTimeout <= 0 || smtpTimeout > 4*time.Second {
		return Config{}, fmt.Errorf("APOLLO_SMTP_TIMEOUT must be greater than zero and at most 4s")
	}
	smtpHost := getEnv("APOLLO_SMTP_HOST", "")
	if smtpHost == "" {
		for _, key := range []string{"APOLLO_SMTP_PORT", "APOLLO_SMTP_TLS_MODE", "APOLLO_SMTP_FROM", "APOLLO_SMTP_USERNAME", "APOLLO_SMTP_PASSWORD", "APOLLO_SMTP_TIMEOUT"} {
			if os.Getenv(key) != "" {
				return Config{}, fmt.Errorf("APOLLO_SMTP_HOST is required when SMTP settings are present")
			}
		}
	}
	athenaTimeout, err := getDurationEnv("APOLLO_ATHENA_TIMEOUT", 2*time.Second)
	if err != nil {
		return Config{}, err
	}
	if athenaTimeout <= 0 {
		return Config{}, fmt.Errorf("APOLLO_ATHENA_TIMEOUT must be greater than zero")
	}
	opsAnalyticsMaxWindow, err := getDurationEnv("APOLLO_OPS_ANALYTICS_MAX_WINDOW", 7*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	if opsAnalyticsMaxWindow <= 0 {
		return Config{}, fmt.Errorf("APOLLO_OPS_ANALYTICS_MAX_WINDOW must be greater than zero")
	}
	athenaBaseURL := getEnv("APOLLO_ATHENA_BASE_URL", "")
	if err := validateOptionalHTTPURL("APOLLO_ATHENA_BASE_URL", athenaBaseURL); err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddr:                getEnv("APOLLO_HTTP_ADDR", ":8081"),
		DatabaseURL:             getEnv("APOLLO_DATABASE_URL", ""),
		NATSURL:                 getEnv("APOLLO_NATS_URL", ""),
		SessionCookieName:       getEnv("APOLLO_SESSION_COOKIE_NAME", "apollo_session"),
		SessionCookieSecret:     getEnv("APOLLO_SESSION_COOKIE_SECRET", ""),
		SessionCookieSecure:     sessionCookieSecure,
		VerificationTokenTTL:    verificationTokenTTL,
		SessionTTL:              sessionTTL,
		LogVerificationTokens:   logVerificationTokens,
		SMTPHost:                smtpHost,
		SMTPPort:                smtpPort,
		SMTPTLSMode:             smtpMode,
		SMTPFrom:                getEnv("APOLLO_SMTP_FROM", ""),
		SMTPUsername:            getEnv("APOLLO_SMTP_USERNAME", ""),
		SMTPPassword:            getEnv("APOLLO_SMTP_PASSWORD", ""),
		SMTPTimeout:             smtpTimeout,
		AthenaBaseURL:           athenaBaseURL,
		AthenaTimeout:           athenaTimeout,
		AthenaInternalReadToken: getEnv("APOLLO_ATHENA_INTERNAL_READ_TOKEN", ""),
		OpsAnalyticsMaxWindow:   opsAnalyticsMaxWindow,
	}, nil
}

func getEnv(key string, fallback string) string {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}

	return value
}

func getBoolEnv(key string, fallback bool) (bool, error) {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s: %w", key, err)
	}

	return parsed, nil
}

func getDurationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}

	return parsed, nil
}

func validateOptionalHTTPURL(key string, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s must use http or https", key)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return fmt.Errorf("%s must include a host", key)
	}

	return nil
}
