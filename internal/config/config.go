package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr              string
	DatabaseURL           string
	NATSURL               string
	SessionCookieName     string
	SessionCookieSecret   string
	SessionCookieSecure   bool
	VerificationTokenTTL  time.Duration
	SessionTTL            time.Duration
	LogVerificationTokens bool
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

	return Config{
		HTTPAddr:              getEnv("APOLLO_HTTP_ADDR", ":8081"),
		DatabaseURL:           getEnv("APOLLO_DATABASE_URL", ""),
		NATSURL:               getEnv("APOLLO_NATS_URL", ""),
		SessionCookieName:     getEnv("APOLLO_SESSION_COOKIE_NAME", "apollo_session"),
		SessionCookieSecret:   getEnv("APOLLO_SESSION_COOKIE_SECRET", ""),
		SessionCookieSecure:   sessionCookieSecure,
		VerificationTokenTTL:  verificationTokenTTL,
		SessionTTL:            sessionTTL,
		LogVerificationTokens: logVerificationTokens,
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
