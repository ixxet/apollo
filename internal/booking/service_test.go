package booking

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/schedule"
	"github.com/ixxet/apollo/internal/testutil"
)

func TestBookingRequestStateMachineApprovalAndScheduleConflictTruth(t *testing.T) {
	service, actor, env, cleanup := newBookingServiceFixture(t)
	defer cleanup()

	fullCourt := insertBookingResource(t, env, "booking-full-court")
	halfCourt := insertBookingResource(t, env, "booking-half-court")
	insertBookingResourceEdge(t, env, fullCourt, halfCourt, schedule.EdgeContains)

	fullRequest, err := service.CreateRequest(context.Background(), actor, bookingRequestInput(fullCourt, "2026-04-18T14:00:00Z", "2026-04-18T15:00:00Z"))
	if err != nil {
		t.Fatalf("CreateRequest(full court) error = %v", err)
	}
	if fullRequest.Availability.Status != AvailabilityAvailable || !fullRequest.Availability.Available {
		t.Fatalf("fullRequest.Availability = %#v, want available", fullRequest.Availability)
	}

	approved, err := service.Approve(context.Background(), actor, fullRequest.ID, TransitionInput{ExpectedVersion: 1})
	if err != nil {
		t.Fatalf("Approve(full court) error = %v", err)
	}
	if approved.Status != StatusApproved {
		t.Fatalf("approved.Status = %q, want %q", approved.Status, StatusApproved)
	}
	if approved.ScheduleBlockID == nil {
		t.Fatal("approved.ScheduleBlockID = nil, want linked schedule block")
	}
	assertLinkedReservationBlock(t, env, *approved.ScheduleBlockID, fullCourt)

	halfRequest, err := service.CreateRequest(context.Background(), actor, bookingRequestInput(halfCourt, "2026-04-18T14:30:00Z", "2026-04-18T15:30:00Z"))
	if err != nil {
		t.Fatalf("CreateRequest(half court) error = %v", err)
	}
	if halfRequest.Availability.Status != AvailabilityConflict || len(halfRequest.Availability.Conflicts) == 0 {
		t.Fatalf("halfRequest.Availability = %#v, want conflict with schedule truth", halfRequest.Availability)
	}
	if _, err := service.Approve(context.Background(), actor, halfRequest.ID, TransitionInput{ExpectedVersion: 1}); !errors.Is(err, schedule.ErrBlockConflictRejected) {
		t.Fatalf("Approve(conflicting half court) error = %v, want %v", err, schedule.ErrBlockConflictRejected)
	}
	unchanged, err := service.GetRequest(context.Background(), halfRequest.ID)
	if err != nil {
		t.Fatalf("GetRequest(after conflict) error = %v", err)
	}
	if unchanged.Status != StatusRequested || unchanged.ScheduleBlockID != nil {
		t.Fatalf("conflicting approval mutated request: status=%s schedule_block_id=%v", unchanged.Status, unchanged.ScheduleBlockID)
	}

	rejected, err := service.Reject(context.Background(), actor, halfRequest.ID, TransitionInput{ExpectedVersion: 1})
	if err != nil {
		t.Fatalf("Reject(conflicting request) error = %v", err)
	}
	if _, err := service.Approve(context.Background(), actor, rejected.ID, TransitionInput{ExpectedVersion: rejected.Version}); !errors.Is(err, ErrRequestTransitionInvalid) {
		t.Fatalf("Approve(rejected) error = %v, want %v", err, ErrRequestTransitionInvalid)
	}
}

func TestBookingRequestTransitionsRequireFreshVersions(t *testing.T) {
	service, actor, env, cleanup := newBookingServiceFixture(t)
	defer cleanup()

	resourceKey := insertBookingResource(t, env, "booking-version-court")
	request, err := service.CreateRequest(context.Background(), actor, bookingRequestInput(resourceKey, "2026-04-19T14:00:00Z", "2026-04-19T15:00:00Z"))
	if err != nil {
		t.Fatalf("CreateRequest() error = %v", err)
	}

	if _, err := service.StartReview(context.Background(), actor, request.ID, TransitionInput{ExpectedVersion: 2}); !errors.Is(err, ErrRequestVersionStale) {
		t.Fatalf("StartReview(stale) error = %v, want %v", err, ErrRequestVersionStale)
	}
	reviewing, err := service.StartReview(context.Background(), actor, request.ID, TransitionInput{ExpectedVersion: 1})
	if err != nil {
		t.Fatalf("StartReview() error = %v", err)
	}
	if reviewing.Status != StatusUnderReview || reviewing.Version != 2 {
		t.Fatalf("reviewing = status %s version %d, want under_review version 2", reviewing.Status, reviewing.Version)
	}
	if _, err := service.StartReview(context.Background(), actor, request.ID, TransitionInput{ExpectedVersion: 2}); !errors.Is(err, ErrRequestTransitionInvalid) {
		t.Fatalf("StartReview(repeated) error = %v, want %v", err, ErrRequestTransitionInvalid)
	}
	needsChanges, err := service.NeedsChanges(context.Background(), actor, request.ID, TransitionInput{ExpectedVersion: 2})
	if err != nil {
		t.Fatalf("NeedsChanges() error = %v", err)
	}
	cancelled, err := service.Cancel(context.Background(), actor, request.ID, TransitionInput{ExpectedVersion: needsChanges.Version})
	if err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if _, err := service.Approve(context.Background(), actor, request.ID, TransitionInput{ExpectedVersion: cancelled.Version}); !errors.Is(err, ErrRequestTransitionInvalid) {
		t.Fatalf("Approve(cancelled) error = %v, want %v", err, ErrRequestTransitionInvalid)
	}
}

func TestBookingRequestValidationAndDeterministicList(t *testing.T) {
	service, actor, env, cleanup := newBookingServiceFixture(t)
	defer cleanup()

	resourceKey := insertBookingResource(t, env, "booking-list-court")
	input := bookingRequestInput(resourceKey, "2026-04-20T14:00:00Z", "2026-04-20T15:00:00Z")

	invalid := input
	invalid.ContactEmail = nil
	invalid.ContactPhone = nil
	if _, err := service.CreateRequest(context.Background(), actor, invalid); !errors.Is(err, ErrContactChannelRequired) {
		t.Fatalf("CreateRequest(no contact) error = %v, want %v", err, ErrContactChannelRequired)
	}
	invalid = input
	invalid.RequestedEndAt = invalid.RequestedStartAt
	if _, err := service.CreateRequest(context.Background(), actor, invalid); !errors.Is(err, ErrWindowInvalid) {
		t.Fatalf("CreateRequest(invalid window) error = %v, want %v", err, ErrWindowInvalid)
	}

	firstNow := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return firstNow }
	first, err := service.CreateRequest(context.Background(), actor, input)
	if err != nil {
		t.Fatalf("CreateRequest(first) error = %v", err)
	}
	secondNow := firstNow.Add(time.Minute)
	service.now = func() time.Time { return secondNow }
	secondInput := bookingRequestInput(resourceKey, "2026-04-20T16:00:00Z", "2026-04-20T17:00:00Z")
	second, err := service.CreateRequest(context.Background(), actor, secondInput)
	if err != nil {
		t.Fatalf("CreateRequest(second) error = %v", err)
	}

	requests, err := service.ListRequests(context.Background(), "ashtonbee")
	if err != nil {
		t.Fatalf("ListRequests() error = %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("len(requests) = %d, want 2", len(requests))
	}
	if requests[0].ID != second.ID || requests[1].ID != first.ID {
		t.Fatalf("requests order = [%s, %s], want newest first [%s, %s]", requests[0].ID, requests[1].ID, second.ID, first.ID)
	}
}

func newBookingServiceFixture(t *testing.T) (*Service, StaffActor, *testutil.PostgresEnv, func()) {
	t.Helper()

	ctx := context.Background()
	postgresEnv, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres() error = %v", err)
	}
	if err := testutil.ApplyApolloSchema(ctx, postgresEnv.DB); err != nil {
		_ = postgresEnv.Close()
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}

	userID := insertBookingUser(t, postgresEnv, "booking-manager-001", "Booking Manager", "booking-manager-001@example.com")
	sessionID := insertBookingSession(t, postgresEnv, userID)
	scheduleService := schedule.NewService(schedule.NewRepository(postgresEnv.DB))
	service := NewService(NewRepository(postgresEnv.DB), scheduleService)
	actor := StaffActor{
		UserID:              userID,
		SessionID:           sessionID,
		Role:                authz.RoleManager,
		Capability:          authz.CapabilityBookingManage,
		TrustedSurfaceKey:   "staff-console",
		TrustedSurfaceLabel: "staff-console",
	}

	return service, actor, postgresEnv, func() {
		if closeErr := postgresEnv.Close(); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	}
}

func bookingRequestInput(resourceKey string, startsAt string, endsAt string) RequestInput {
	email := "casey@example.com"
	start := mustParseBookingTime(startsAt)
	end := mustParseBookingTime(endsAt)
	return RequestInput{
		FacilityKey:      "ashtonbee",
		ZoneKey:          stringPtr("gym-floor"),
		ResourceKey:      &resourceKey,
		RequestedStartAt: start,
		RequestedEndAt:   end,
		ContactName:      "Casey Booker",
		ContactEmail:     &email,
		Purpose:          stringPtr("Staff-entered court request"),
	}
}

func assertLinkedReservationBlock(t *testing.T, env *testutil.PostgresEnv, blockID uuid.UUID, resourceKey string) {
	t.Helper()

	var kind, effect, visibility, capability, storedResourceKey string
	if err := env.DB.QueryRow(context.Background(), `
SELECT kind,
       effect,
       visibility,
       created_by_capability,
       resource_key
FROM apollo.schedule_blocks
WHERE id = $1
`, blockID).Scan(&kind, &effect, &visibility, &capability, &storedResourceKey); err != nil {
		t.Fatalf("QueryRow(schedule block) error = %v", err)
	}
	if kind != schedule.KindReservation || effect != schedule.EffectHardReserve || visibility != schedule.VisibilityInternal {
		t.Fatalf("schedule block shape = %s/%s/%s, want reservation/hard_reserve/internal", kind, effect, visibility)
	}
	if capability != string(authz.CapabilityBookingManage) {
		t.Fatalf("created_by_capability = %q, want %q", capability, authz.CapabilityBookingManage)
	}
	if storedResourceKey != resourceKey {
		t.Fatalf("resource_key = %q, want %q", storedResourceKey, resourceKey)
	}
}

func insertBookingResource(t *testing.T, env *testutil.PostgresEnv, resourceKey string) string {
	t.Helper()

	var insertedKey string
	if err := env.DB.QueryRow(context.Background(), `
INSERT INTO apollo.schedule_resources (
    resource_key,
    facility_key,
    zone_key,
    resource_type,
    display_name
)
VALUES ($1, 'ashtonbee', 'gym-floor', 'court', $2)
RETURNING resource_key
`, resourceKey, resourceKey).Scan(&insertedKey); err != nil {
		t.Fatalf("insert booking resource error = %v", err)
	}
	return insertedKey
}

func insertBookingResourceEdge(t *testing.T, env *testutil.PostgresEnv, resourceKey string, relatedResourceKey string, edgeType string) {
	t.Helper()

	if _, err := env.DB.Exec(context.Background(), `
INSERT INTO apollo.schedule_resource_edges (
    resource_key,
    related_resource_key,
    edge_type
)
VALUES ($1, $2, $3)
`, resourceKey, relatedResourceKey, edgeType); err != nil {
		t.Fatalf("insert booking resource edge error = %v", err)
	}
}

func insertBookingUser(t *testing.T, env *testutil.PostgresEnv, studentID string, displayName string, email string) uuid.UUID {
	t.Helper()

	var userID uuid.UUID
	if err := env.DB.QueryRow(context.Background(), `
INSERT INTO apollo.users (student_id, display_name, email)
VALUES ($1, $2, $3)
RETURNING id
`, studentID, displayName, email).Scan(&userID); err != nil {
		t.Fatalf("insert booking user error = %v", err)
	}
	return userID
}

func insertBookingSession(t *testing.T, env *testutil.PostgresEnv, userID uuid.UUID) uuid.UUID {
	t.Helper()

	var sessionID uuid.UUID
	if err := env.DB.QueryRow(context.Background(), `
INSERT INTO apollo.sessions (user_id, expires_at, revoked_at)
VALUES ($1, NOW() + INTERVAL '1 hour', NULL)
RETURNING id
`, userID).Scan(&sessionID); err != nil {
		t.Fatalf("insert booking session error = %v", err)
	}
	return sessionID
}

func mustParseBookingTime(raw string) time.Time {
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		panic(err)
	}
	return parsed
}

func stringPtr(value string) *string {
	return &value
}
