package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadReadsOpsAthenaConfig(t *testing.T) {
	t.Setenv("APOLLO_ATHENA_BASE_URL", "http://127.0.0.1:8080")
	t.Setenv("APOLLO_ATHENA_TIMEOUT", "1500ms")
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
