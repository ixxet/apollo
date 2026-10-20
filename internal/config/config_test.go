package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadReadsOpsAthenaConfig(t *testing.T) {
	t.Setenv("APOLLO_ATHENA_BASE_URL", "http://127.0.0.1:8080")
	t.Setenv("APOLLO_ATHENA_TIMEOUT", "1500ms")
	t.Setenv("APOLLO_ATHENA_INTERNAL_READ_TOKEN", "bridge-token")
	t.Setenv("APOLLO_OPS_ANALYTICS_MAX_WINDOW", "48h")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AthenaBaseURL != "http://127.0.0.1:8080" {
		t.Fatalf("AthenaBaseURL = %q, want configured URL", cfg.AthenaBaseURL)
	}
	if cfg.AthenaTimeout != 1500*time.Millisecond {
		t.Fatalf("AthenaTimeout = %s, want 1500ms", cfg.AthenaTimeout)
	}
	if cfg.AthenaInternalReadToken != "bridge-token" {
		t.Fatalf("AthenaInternalReadToken = %q, want bridge-token", cfg.AthenaInternalReadToken)
	}
	if cfg.OpsAnalyticsMaxWindow != 48*time.Hour {
		t.Fatalf("OpsAnalyticsMaxWindow = %s, want 48h", cfg.OpsAnalyticsMaxWindow)
	}
}

func TestLoadRejectsInvalidOpsAthenaConfig(t *testing.T) {
	testCases := []struct {
		name string
		key  string
		val  string
		want string
	}{
		{
			name: "base url scheme",
			key:  "APOLLO_ATHENA_BASE_URL",
			val:  "ftp://127.0.0.1",
			want: "APOLLO_ATHENA_BASE_URL",
		},
		{
			name: "timeout",
			key:  "APOLLO_ATHENA_TIMEOUT",
			val:  "0s",
			want: "APOLLO_ATHENA_TIMEOUT",
		},
		{
			name: "max window",
			key:  "APOLLO_OPS_ANALYTICS_MAX_WINDOW",
			val:  "0s",
			want: "APOLLO_OPS_ANALYTICS_MAX_WINDOW",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("APOLLO_ATHENA_BASE_URL", "")
			t.Setenv("APOLLO_ATHENA_TIMEOUT", "")
			t.Setenv("APOLLO_OPS_ANALYTICS_MAX_WINDOW", "")
			t.Setenv(testCase.key, testCase.val)
			_, err := Load()
			if err == nil {
				t.Fatal("Load() error = nil, want refusal")
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("Load() error = %q, want %q", err.Error(), testCase.want)
			}
		})
	}
}

func TestLoadSMTPConfiguration(t *testing.T) {
	for _, mode := range []string{"starttls", "tls"} {
		t.Run(mode, func(t *testing.T) {
			for _, key := range []string{"APOLLO_SMTP_PORT", "APOLLO_SMTP_TIMEOUT"} {
				t.Setenv(key, "")
			}
			t.Setenv("APOLLO_SMTP_HOST", "smtp.example.invalid")
			t.Setenv("APOLLO_SMTP_TLS_MODE", mode)
			t.Setenv("APOLLO_SMTP_FROM", "signin@example.invalid")
			t.Setenv("APOLLO_SMTP_USERNAME", "fixture-user")
			t.Setenv("APOLLO_SMTP_PASSWORD", "fixture-password")
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			port := 587
			if mode == "tls" {
				port = 465
			}
			if cfg.SMTPHost != "smtp.example.invalid" || cfg.SMTPPort != port || cfg.SMTPTimeout != 4*time.Second || cfg.SMTPFrom != "signin@example.invalid" || cfg.SMTPUsername != "fixture-user" || cfg.SMTPPassword != "fixture-password" {
				t.Fatal("SMTP settings were not loaded correctly")
			}
		})
	}
}

func TestLoadRejectsInvalidOrPartialSMTPConfiguration(t *testing.T) {
	for _, testCase := range []struct{ key, value string }{
		{"APOLLO_SMTP_PORT", "invalid-port"},
		{"APOLLO_SMTP_PORT", "65536"},
		{"APOLLO_SMTP_TIMEOUT", "0s"},
		{"APOLLO_SMTP_TIMEOUT", "5s"},
		{"APOLLO_SMTP_FROM", "signin@example.invalid"},
	} {
		t.Run(testCase.key+testCase.value, func(t *testing.T) {
			for _, key := range []string{"APOLLO_SMTP_HOST", "APOLLO_SMTP_PORT", "APOLLO_SMTP_TLS_MODE", "APOLLO_SMTP_FROM", "APOLLO_SMTP_USERNAME", "APOLLO_SMTP_PASSWORD", "APOLLO_SMTP_TIMEOUT"} {
				t.Setenv(key, "")
			}
			t.Setenv(testCase.key, testCase.value)
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), "APOLLO_SMTP_") {
				t.Fatalf("expected SMTP configuration error, got %v", err)
			}
		})
	}
}
