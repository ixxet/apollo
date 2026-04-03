package main

import (
	"testing"
	"time"

	"github.com/ixxet/apollo/internal/auth"
	"github.com/ixxet/apollo/internal/config"
)

func TestBuildServerDependenciesIncludesWorkoutRuntime(t *testing.T) {
	cookies, err := auth.NewSessionCookieManager("apollo_session", "0123456789abcdef0123456789abcdef", true)
	if err != nil {
		t.Fatalf("NewSessionCookieManager() error = %v", err)
	}

	deps := buildServerDependencies(nil, true, cookies, auth.NoopEmailSender{}, config.Config{
		VerificationTokenTTL: 15 * time.Minute,
		SessionTTL:           7 * 24 * time.Hour,
	})

	if deps.Auth == nil {
		t.Fatal("deps.Auth = nil, want auth service")
	}
	if deps.Profile == nil {
		t.Fatal("deps.Profile = nil, want profile service")
	}
	if deps.Eligibility == nil {
		t.Fatal("deps.Eligibility = nil, want eligibility service")
	}
	if deps.Recommendations == nil {
		t.Fatal("deps.Recommendations = nil, want recommendation service")
	}
	if deps.Workouts == nil {
		t.Fatal("deps.Workouts = nil, want workout service")
	}
	if !deps.ConsumerEnabled {
		t.Fatal("deps.ConsumerEnabled = false, want true")
	}
}
