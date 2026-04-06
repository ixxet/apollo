package main

import (
	"testing"
	"time"

	"github.com/ixxet/apollo/internal/auth"
	"github.com/ixxet/apollo/internal/config"
)

func TestBuildServerDependenciesWiresLobbyMembershipAndMatchPreviewRuntime(t *testing.T) {
	cookies, err := auth.NewSessionCookieManager("apollo_session", "0123456789abcdef0123456789abcdef", true)
	if err != nil {
		t.Fatalf("NewSessionCookieManager() error = %v", err)
	}

	deps := buildServerDependencies(nil, false, cookies, auth.LogEmailSender{}, config.Config{
		VerificationTokenTTL: 15 * time.Minute,
		SessionTTL:           7 * 24 * time.Hour,
	})

	if deps.Membership == nil {
		t.Fatal("deps.Membership = nil, want lobby membership runtime wired")
	}
	if deps.MatchPreview == nil {
		t.Fatal("deps.MatchPreview = nil, want match preview runtime wired")
	}
}
