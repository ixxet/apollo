package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ixxet/apollo/internal/auth"
	"github.com/ixxet/apollo/internal/config"
	"github.com/ixxet/apollo/internal/testutil"
)

func TestBuildServerDependenciesWiresCompetitionMembershipMatchPreviewAndPresenceRuntime(t *testing.T) {
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
	if deps.Competition == nil {
		t.Fatal("deps.Competition = nil, want competition container runtime wired")
	}
	if deps.Coaching == nil {
		t.Fatal("deps.Coaching = nil, want deterministic coaching runtime wired")
	}
	if deps.Presence == nil {
		t.Fatal("deps.Presence = nil, want member presence runtime wired")
	}
}

func TestNewRootCmdIncludesSportCommand(t *testing.T) {
	rootCmd := newRootCmd()

	if _, _, err := rootCmd.Find([]string{"sport"}); err != nil {
		t.Fatalf("rootCmd.Find(sport) error = %v", err)
	}
}

func TestSportListCommandReturnsDeterministicJSON(t *testing.T) {
	ctx := context.Background()
	postgresEnv, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres() error = %v", err)
	}
	defer func() {
		if closeErr := postgresEnv.Close(); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	}()

	if err := testutil.ApplyApolloSchema(ctx, postgresEnv.DB); err != nil {
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}

	t.Setenv("APOLLO_DATABASE_URL", postgresEnv.DatabaseURL)

	first := runRootCommand(t, "sport", "list", "--format", "json")
	second := runRootCommand(t, "sport", "list", "--format", "json")
	if first != second {
		t.Fatalf("sport list output changed between runs:\nfirst=%s\nsecond=%s", first, second)
	}

	var decoded []map[string]any
	if err := json.Unmarshal([]byte(first), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(decoded) != 2 {
		t.Fatalf("len(decoded) = %d, want 2", len(decoded))
	}
	if decoded[0]["sport_key"] != "badminton" || decoded[1]["sport_key"] != "basketball" {
		t.Fatalf("decoded sport order = %#v, want badminton then basketball", decoded)
	}
}

func TestSportShowCommandIncludesFacilityCapabilityText(t *testing.T) {
	ctx := context.Background()
	postgresEnv, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres() error = %v", err)
	}
	defer func() {
		if closeErr := postgresEnv.Close(); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	}()

	if err := testutil.ApplyApolloSchema(ctx, postgresEnv.DB); err != nil {
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}

	t.Setenv("APOLLO_DATABASE_URL", postgresEnv.DatabaseURL)

	output := runRootCommand(t, "sport", "show", "--sport-key", "badminton", "--format", "text")
	if !strings.Contains(output, `key=badminton`) {
		t.Fatalf("output = %q, want sport key", output)
	}
	if !strings.Contains(output, `sport=badminton facility=ashtonbee zones=gym-floor`) {
		t.Fatalf("output = %q, want facility capability line", output)
	}
}

func TestSportCapabilityListCommandReturnsFacilityNotFoundForUnknownFacility(t *testing.T) {
	ctx := context.Background()
	postgresEnv, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres() error = %v", err)
	}
	defer func() {
		if closeErr := postgresEnv.Close(); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	}()

	if err := testutil.ApplyApolloSchema(ctx, postgresEnv.DB); err != nil {
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}

	t.Setenv("APOLLO_DATABASE_URL", postgresEnv.DatabaseURL)

	_, stderr, err := runRootCommandWithResult(t, "sport", "capability", "list", "--facility-key", "unknown-facility", "--format", "text")
	if err == nil {
		t.Fatal("Execute() error = nil, want facility not found")
	}
	if err.Error() != `facility "unknown-facility" not found` {
		t.Fatalf("err = %q, want facility not found message", err.Error())
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty stderr because Execute returns the error", stderr)
	}
}

func TestSportCapabilityListCommandHonorsKnownFacilityFilter(t *testing.T) {
	ctx := context.Background()
	postgresEnv, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres() error = %v", err)
	}
	defer func() {
		if closeErr := postgresEnv.Close(); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	}()

	if err := testutil.ApplyApolloSchema(ctx, postgresEnv.DB); err != nil {
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}

	t.Setenv("APOLLO_DATABASE_URL", postgresEnv.DatabaseURL)

	output := runRootCommand(t, "sport", "capability", "list", "--facility-key", "ashtonbee", "--format", "text")
	if !strings.Contains(output, `sport=badminton facility=ashtonbee zones=gym-floor`) {
		t.Fatalf("output = %q, want badminton capability line", output)
	}
	if !strings.Contains(output, `sport=basketball facility=ashtonbee zones=gym-floor`) {
		t.Fatalf("output = %q, want basketball capability line", output)
	}
}

func TestSportShowCommandReturnsSportNotFoundForUnknownSport(t *testing.T) {
	ctx := context.Background()
	postgresEnv, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres() error = %v", err)
	}
	defer func() {
		if closeErr := postgresEnv.Close(); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	}()

	if err := testutil.ApplyApolloSchema(ctx, postgresEnv.DB); err != nil {
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}

	t.Setenv("APOLLO_DATABASE_URL", postgresEnv.DatabaseURL)

	_, _, err = runRootCommandWithResult(t, "sport", "show", "--sport-key", "pickleball", "--format", "text")
	if err == nil {
		t.Fatal("Execute() error = nil, want sport not found")
	}
	if err.Error() != `sport "pickleball" not found` {
		t.Fatalf("err = %q, want sport not found message", err.Error())
	}
}

func TestServeCommandShutsDownCleanlyWhenContextIsCanceled(t *testing.T) {
	ctx := context.Background()
	postgresEnv, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres() error = %v", err)
	}
	defer func() {
		if closeErr := postgresEnv.Close(); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	}()

	if err := testutil.ApplyApolloSchema(ctx, postgresEnv.DB); err != nil {
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}

	addr := reserveListenAddress(t)
	t.Setenv("APOLLO_DATABASE_URL", postgresEnv.DatabaseURL)
	t.Setenv("APOLLO_HTTP_ADDR", addr)
	t.Setenv("APOLLO_SESSION_COOKIE_SECRET", "0123456789abcdef0123456789abcdef")

	serveCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	command := newServeCmd()
	command.SetContext(serveCtx)

	done := make(chan error, 1)
	go func() {
		done <- command.Execute()
	}()

	waitForHealthyHTTP(t, fmt.Sprintf("http://%s/api/v1/health", addr))
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve command did not shut down within 10s after context cancellation")
	}
}

func runRootCommand(t *testing.T, args ...string) string {
	t.Helper()

	stdout, stderr, err := runRootCommandWithResult(t, args...)
	if err != nil {
		t.Fatalf("Execute(%v) error = %v stderr=%s", args, err, stderr)
	}

	return stdout
}

func runRootCommandWithResult(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	rootCmd := newRootCmd()
	rootCmd.SetArgs(args)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)

	err := rootCmd.Execute()
	return stdout.String(), stderr.String(), err
}

func reserveListenAddress(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	defer listener.Close()

	return listener.Addr().String()
}

func waitForHealthyHTTP(t *testing.T, url string) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(url) //nolint:gosec // local integration probe
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("health endpoint %s did not become ready", url)
}
