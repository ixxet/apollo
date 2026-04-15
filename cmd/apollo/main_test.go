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

	"github.com/google/uuid"
	"github.com/ixxet/apollo/internal/auth"
	"github.com/ixxet/apollo/internal/config"
	"github.com/ixxet/apollo/internal/schedule"
	"github.com/ixxet/apollo/internal/testutil"
)

func TestBuildServerDependenciesWiresCompetitionMembershipMatchPreviewAndPresenceRuntime(t *testing.T) {
	cookies, err := auth.NewSessionCookieManager("apollo_session", "0123456789abcdef0123456789abcdef", true)
	if err != nil {
		t.Fatalf("NewSessionCookieManager() error = %v", err)
	}

	deps, err := buildServerDependencies(nil, false, cookies, auth.LogEmailSender{}, config.Config{
		VerificationTokenTTL: 15 * time.Minute,
		SessionTTL:           7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("buildServerDependencies() error = %v", err)
	}

	if deps.Membership == nil {
		t.Fatal("deps.Membership = nil, want lobby membership runtime wired")
	}
	if deps.MatchPreview == nil {
		t.Fatal("deps.MatchPreview = nil, want match preview runtime wired")
	}
	if deps.Competition == nil {
		t.Fatal("deps.Competition = nil, want competition container runtime wired")
	}
	if deps.Schedule == nil {
		t.Fatal("deps.Schedule = nil, want schedule substrate runtime wired")
	}
	if deps.Coaching == nil {
		t.Fatal("deps.Coaching = nil, want deterministic coaching runtime wired")
	}
	if deps.Presence == nil {
		t.Fatal("deps.Presence = nil, want member presence runtime wired")
	}
	if deps.Ops != nil {
		t.Fatal("deps.Ops != nil without ATHENA config")
	}

	deps, err = buildServerDependencies(nil, false, cookies, auth.LogEmailSender{}, config.Config{
		VerificationTokenTTL:  15 * time.Minute,
		SessionTTL:            7 * 24 * time.Hour,
		AthenaBaseURL:         "http://127.0.0.1:8080",
		AthenaTimeout:         2 * time.Second,
		OpsAnalyticsMaxWindow: 7 * 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("buildServerDependencies(with ATHENA) error = %v", err)
	}
	if deps.Ops == nil {
		t.Fatal("deps.Ops = nil, want ops overview runtime wired when ATHENA config is present")
	}
}

func TestNewRootCmdIncludesSportCommand(t *testing.T) {
	rootCmd := newRootCmd()

	if _, _, err := rootCmd.Find([]string{"sport"}); err != nil {
		t.Fatalf("rootCmd.Find(sport) error = %v", err)
	}
}

func TestNewRootCmdIncludesScheduleCommand(t *testing.T) {
	rootCmd := newRootCmd()

	if _, _, err := rootCmd.Find([]string{"schedule"}); err != nil {
		t.Fatalf("rootCmd.Find(schedule) error = %v", err)
	}
}

func TestScheduleCommandsRoundTripResourcesBlocksAndCalendar(t *testing.T) {
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

	actorUserID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	actorSessionID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	if _, err := postgresEnv.DB.Exec(ctx, `INSERT INTO apollo.users (id, student_id, display_name, email) VALUES ($1, $2, $3, $4)`, actorUserID, "schedule-cli-owner", "Schedule CLI Owner", "schedule-cli-owner@example.com"); err != nil {
		t.Fatalf("insert actor user error = %v", err)
	}
	if _, err := postgresEnv.DB.Exec(ctx, `INSERT INTO apollo.sessions (id, user_id, expires_at, revoked_at) VALUES ($1, $2, NOW() + INTERVAL '1 hour', NULL)`, actorSessionID, actorUserID); err != nil {
		t.Fatalf("insert actor session error = %v", err)
	}

	actorFlags := []string{
		"--actor-user-id", actorUserID.String(),
		"--actor-session-id", actorSessionID.String(),
		"--actor-role", "owner",
		"--trusted-surface-key", "staff-console",
	}
	managerFlags := []string{
		"--actor-user-id", actorUserID.String(),
		"--actor-session-id", actorSessionID.String(),
		"--actor-role", "manager",
		"--trusted-surface-key", "staff-console",
	}

	if _, _, err := runRootCommandWithResult(t, append([]string{"schedule", "resource", "upsert", "--resource-key", "manager-graph-attempt", "--facility-key", "ashtonbee", "--zone-key", "gym-floor", "--resource-type", "court", "--display-name", "Manager Graph Attempt"}, managerFlags...)...); err == nil || !strings.Contains(err.Error(), "schedule graph authoring requires owner role") {
		t.Fatalf("manager graph authoring error = %v, want owner-only refusal", err)
	}

	runRootCommand(t, append([]string{"schedule", "resource", "upsert", "--resource-key", "full-court", "--facility-key", "ashtonbee", "--zone-key", "gym-floor", "--resource-type", "court", "--display-name", "Full Court"}, actorFlags...)...)
	runRootCommand(t, append([]string{"schedule", "resource", "upsert", "--resource-key", "half-court-a", "--facility-key", "ashtonbee", "--zone-key", "gym-floor", "--resource-type", "court", "--display-name", "Half Court A"}, actorFlags...)...)
	runRootCommand(t, append([]string{"schedule", "resource", "edge", "upsert", "--resource-key", "full-court", "--related-resource-key", "half-court-a", "--edge-type", "contains"}, actorFlags...)...)

	resourceListJSON := runRootCommand(t, "schedule", "resource", "list", "--facility-key", "ashtonbee", "--format", "json")
	var resources []schedule.Resource
	if err := json.Unmarshal([]byte(resourceListJSON), &resources); err != nil {
		t.Fatalf("json.Unmarshal(resourceListJSON) error = %v output=%s", err, resourceListJSON)
	}
	if len(resources) != 2 {
		t.Fatalf("len(resources) = %d, want 2", len(resources))
	}
	resourceKeys := make(map[string]struct{}, len(resources))
	for _, resource := range resources {
		resourceKeys[resource.ResourceKey] = struct{}{}
	}
	if _, ok := resourceKeys["full-court"]; !ok {
		t.Fatalf("resource list missing full-court: %#v", resourceKeys)
	}
	if _, ok := resourceKeys["half-court-a"]; !ok {
		t.Fatalf("resource list missing half-court-a: %#v", resourceKeys)
	}

	resourceListText := runRootCommand(t, "schedule", "resource", "list", "--facility-key", "ashtonbee", "--format", "text")
	if !strings.Contains(resourceListText, "full-court court Full Court") {
		t.Fatalf("resourceListText = %q, want full-court line", resourceListText)
	}
	if !strings.Contains(resourceListText, "half-court-a court Half Court A") {
		t.Fatalf("resourceListText = %q, want half-court-a line", resourceListText)
	}

	resourceShowOutput := runRootCommand(t, "schedule", "resource", "show", "--resource-key", "full-court")
	var shown schedule.Resource
	if err := json.Unmarshal([]byte(resourceShowOutput), &shown); err != nil {
		t.Fatalf("json.Unmarshal(resourceShowOutput) error = %v output=%s", err, resourceShowOutput)
	}
	if shown.ResourceKey != "full-court" {
		t.Fatalf("shown.ResourceKey = %q, want %q", shown.ResourceKey, "full-court")
	}
	if len(shown.Edges) != 1 {
		t.Fatalf("len(shown.Edges) = %d, want 1", len(shown.Edges))
	}
	if shown.Edges[0].EdgeType != schedule.EdgeContains || shown.Edges[0].RelatedResourceKey != "half-court-a" {
		t.Fatalf("shown.Edges[0] = %#v, want contains edge to half-court-a", shown.Edges[0])
	}

	blockOutput := runRootCommand(t, append([]string{"schedule", "block", "create",
		"--facility-key", "ashtonbee",
		"--scope", "facility",
		"--kind", "operating_hours",
		"--effect", "informational",
		"--visibility", "public_labeled",
		"--weekly-weekday", "1",
		"--weekly-start-time", "09:00",
		"--weekly-end-time", "10:00",
		"--weekly-timezone", "America/Toronto",
		"--weekly-recurrence-start-date", "2026-04-06",
		"--weekly-recurrence-end-date", "2026-04-20",
	}, actorFlags...)...)

	var created schedule.Block
	if err := json.Unmarshal([]byte(blockOutput), &created); err != nil {
		t.Fatalf("json.Unmarshal(blockOutput) error = %v output=%s", err, blockOutput)
	}

	blockListJSON := runRootCommand(t, "schedule", "block", "list", "--facility-key", "ashtonbee", "--format", "json")
	var listedBlocks []schedule.Block
	if err := json.Unmarshal([]byte(blockListJSON), &listedBlocks); err != nil {
		t.Fatalf("json.Unmarshal(blockListJSON) error = %v output=%s", err, blockListJSON)
	}
	foundCreated := false
	for _, block := range listedBlocks {
		if block.ID == created.ID {
			foundCreated = true
			break
		}
	}
	if !foundCreated {
		t.Fatalf("block list missing created block %s", created.ID)
	}

	blockListText := runRootCommand(t, "schedule", "block", "list", "--facility-key", "ashtonbee", "--format", "text")
	if !strings.Contains(blockListText, created.ID.String()) || !strings.Contains(blockListText, "operating_hours") {
		t.Fatalf("blockListText = %q, want created operating-hours block", blockListText)
	}

	calendarOutput := runRootCommand(t, "schedule", "calendar", "--facility-key", "ashtonbee", "--from", "2026-04-06T00:00:00Z", "--until", "2026-04-27T00:00:00Z", "--format", "json")
	var occurrences []schedule.Occurrence
	if err := json.Unmarshal([]byte(calendarOutput), &occurrences); err != nil {
		t.Fatalf("json.Unmarshal(calendarOutput) error = %v output=%s", err, calendarOutput)
	}
	if len(occurrences) != 3 {
		t.Fatalf("len(occurrences) = %d, want 3", len(occurrences))
	}

	calendarText := runRootCommand(t, "schedule", "calendar", "--facility-key", "ashtonbee", "--from", "2026-04-06T00:00:00Z", "--until", "2026-04-27T00:00:00Z", "--format", "text")
	if !strings.Contains(calendarText, "2026-04-06 facility operating_hours 2026-04-06T13:00:00Z 2026-04-06T14:00:00Z") {
		t.Fatalf("calendarText = %q, want first operating-hours occurrence", calendarText)
	}

	exceptionOutput := runRootCommand(t, append([]string{"schedule", "block", "except", "--block-id", created.ID.String(), "--expected-version", "1", "--exception-date", "2026-04-13"}, actorFlags...)...)
	if err := json.Unmarshal([]byte(exceptionOutput), &created); err != nil {
		t.Fatalf("json.Unmarshal(exceptionOutput) error = %v output=%s", err, exceptionOutput)
	}
	if created.Version != 2 {
		t.Fatalf("created.Version = %d, want 2", created.Version)
	}

	calendarOutput = runRootCommand(t, "schedule", "calendar", "--facility-key", "ashtonbee", "--from", "2026-04-06T00:00:00Z", "--until", "2026-04-27T00:00:00Z", "--format", "json")
	occurrences = nil
	if err := json.Unmarshal([]byte(calendarOutput), &occurrences); err != nil {
		t.Fatalf("json.Unmarshal(calendarOutput after exception) error = %v output=%s", err, calendarOutput)
	}
	if len(occurrences) != 2 {
		t.Fatalf("len(occurrences) = %d, want 2", len(occurrences))
	}

	cancelOutput := runRootCommand(t, append([]string{"schedule", "block", "cancel", "--block-id", created.ID.String(), "--expected-version", "2"}, actorFlags...)...)
	if err := json.Unmarshal([]byte(cancelOutput), &created); err != nil {
		t.Fatalf("json.Unmarshal(cancelOutput) error = %v output=%s", err, cancelOutput)
	}
	if created.Status != schedule.StatusCancelled {
		t.Fatalf("created.Status = %q, want %q", created.Status, schedule.StatusCancelled)
	}

	_, _, err = runRootCommandWithResult(t, "schedule", "calendar", "--facility-key", "ashtonbee", "--from", "2026-04-06", "--until", "2026-04-27", "--format", "json")
	if err == nil || !strings.Contains(err.Error(), "RFC3339") {
		t.Fatalf("date-only schedule calendar error = %v, want RFC3339 refusal", err)
	}

	timezoneBlockOutput := runRootCommand(t, append([]string{"schedule", "block", "create",
		"--facility-key", "ashtonbee",
		"--scope", "facility",
		"--kind", "event",
		"--effect", "informational",
		"--visibility", "internal",
		"--weekly-weekday", "7",
		"--weekly-start-time", "23:30",
		"--weekly-end-time", "23:50",
		"--weekly-timezone", "America/Toronto",
		"--weekly-recurrence-start-date", "2026-04-05",
		"--weekly-recurrence-end-date", "2026-04-12",
	}, actorFlags...)...)
	var timezoneBlock schedule.Block
	if err := json.Unmarshal([]byte(timezoneBlockOutput), &timezoneBlock); err != nil {
		t.Fatalf("json.Unmarshal(timezoneBlockOutput) error = %v output=%s", err, timezoneBlockOutput)
	}
	if timezoneBlock.ID == uuid.Nil {
		t.Fatal("timezoneBlock.ID = nil")
	}

	calendarOutput = runRootCommand(t, "schedule", "calendar", "--facility-key", "ashtonbee", "--from", "2026-04-06T03:00:00Z", "--until", "2026-04-06T04:00:00Z", "--format", "json")
	occurrences = nil
	if err := json.Unmarshal([]byte(calendarOutput), &occurrences); err != nil {
		t.Fatalf("json.Unmarshal(calendarOutput timezone window) error = %v output=%s", err, calendarOutput)
	}
	if len(occurrences) != 1 {
		t.Fatalf("len(occurrences) = %d, want 1 for timezone edge window", len(occurrences))
	}
	if occurrences[0].OccurrenceDate != "2026-04-05" {
		t.Fatalf("occurrences[0].OccurrenceDate = %q, want %q", occurrences[0].OccurrenceDate, "2026-04-05")
	}
	if got := occurrences[0].StartsAt.UTC().Format(time.RFC3339); got != "2026-04-06T03:30:00Z" {
		t.Fatalf("occurrences[0].StartsAt = %q, want %q", got, "2026-04-06T03:30:00Z")
	}

	calendarOutput = runRootCommand(t, "schedule", "calendar", "--facility-key", "ashtonbee", "--from", "2026-04-06T04:00:00Z", "--until", "2026-04-06T05:00:00Z", "--format", "json")
	occurrences = nil
	if err := json.Unmarshal([]byte(calendarOutput), &occurrences); err != nil {
		t.Fatalf("json.Unmarshal(calendarOutput narrow timezone window) error = %v output=%s", err, calendarOutput)
	}
	if len(occurrences) != 0 {
		t.Fatalf("len(occurrences) = %d, want 0 for narrow timezone window", len(occurrences))
	}

	nonOwnerBlockOutput := runRootCommand(t, append([]string{"schedule", "block", "create",
		"--facility-key", "ashtonbee",
		"--scope", "facility",
		"--kind", "closure",
		"--effect", "closed",
		"--visibility", "internal",
		"--one-off-starts-at", "2026-04-07T10:00:00Z",
		"--one-off-ends-at", "2026-04-07T11:00:00Z",
	}, managerFlags...)...)
	if !strings.Contains(nonOwnerBlockOutput, `"kind": "closure"`) {
		t.Fatalf("manager block write output = %q, want created closure block", nonOwnerBlockOutput)
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
