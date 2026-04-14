package migrations

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/testutil"
)

func TestScheduleSubstrateSchemaEnforcesResourceEdgesAndShape(t *testing.T) {
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

	facilityKey := "ashtonbee"
	resourceOne := insertScheduleResource(t, ctx, postgresEnv, "full-court", facilityKey, "gym-floor", "court", "Full Court")
	resourceTwo := insertScheduleResource(t, ctx, postgresEnv, "half-court-a", facilityKey, "gym-floor", "court", "Half Court A")
	resourceThree := insertScheduleResource(t, ctx, postgresEnv, "half-court-b", facilityKey, "gym-floor", "court", "Half Court B")

	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.schedule_resource_edges (
    resource_key,
    related_resource_key,
    edge_type
)
VALUES ($1, $1, 'contains')
`, resourceOne); err == nil {
		t.Fatal("Exec(self-referential edge) error = nil, want check violation")
	}

	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.schedule_resource_edges (
    resource_key,
    related_resource_key,
    edge_type
)
VALUES ($1, $2, 'exclusive_with')
`, resourceTwo, resourceOne); err == nil {
		t.Fatal("Exec(reversed exclusive_with) error = nil, want check violation")
	}

	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.schedule_resource_edges (
    resource_key,
    related_resource_key,
    edge_type
)
VALUES ($1, $2, 'contains')
`, resourceOne, resourceTwo); err != nil {
		t.Fatalf("Exec(parent contains child) error = %v", err)
	}
	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.schedule_resource_edges (
    resource_key,
    related_resource_key,
    edge_type
)
VALUES ($1, $2, 'contains')
`, resourceOne, resourceThree); err != nil {
		t.Fatalf("Exec(parent contains second child) error = %v", err)
	}
}

func TestScheduleBlocksSchemaSupportsOneOffAndWeeklyShapes(t *testing.T) {
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

	ownerUserID := insertScheduleUser(t, ctx, postgresEnv, "schedule-owner-001", "Schedule Owner", "schedule-owner-001@example.com")
	sessionID := insertScheduleSession(t, ctx, postgresEnv, ownerUserID)

	facilityKey := "ashtonbee"
	resourceKey := insertScheduleResource(t, ctx, postgresEnv, "court-101", facilityKey, "gym-floor", "court", "Court 101")

	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.schedule_blocks (
    facility_key,
    scope,
    schedule_type,
    kind,
    effect,
    visibility,
    status,
    start_at,
    end_at,
    created_by_user_id,
    created_by_session_id,
    created_by_role,
    created_by_capability,
    created_trusted_surface_key,
    created_trusted_surface_label,
    updated_by_user_id,
    updated_by_session_id,
    updated_by_role,
    updated_by_capability,
    updated_trusted_surface_key,
    updated_trusted_surface_label
)
VALUES ($1, 'facility', 'one_off', 'closure', 'closed', 'internal', 'scheduled', NOW(), NOW() + INTERVAL '1 hour', $2, $3, 'owner', 'schedule_manage', 'staff-console', 'staff-console', $2, $3, 'owner', 'schedule_manage', 'staff-console', 'staff-console')
`, facilityKey, ownerUserID, sessionID); err != nil {
		t.Fatalf("Exec(one-off block) error = %v", err)
	}

	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.schedule_blocks (
    facility_key,
    zone_key,
    resource_key,
    scope,
    schedule_type,
    kind,
    effect,
    visibility,
    status,
    weekday,
    start_time,
    end_time,
    timezone,
    recurrence_start_date,
    recurrence_end_date,
    created_by_user_id,
    created_by_session_id,
    created_by_role,
    created_by_capability,
    created_trusted_surface_key,
    created_trusted_surface_label,
    updated_by_user_id,
    updated_by_session_id,
    updated_by_role,
    updated_by_capability,
    updated_trusted_surface_key,
    updated_trusted_surface_label
)
VALUES ($1, $2, $3, 'resource', 'weekly', 'operating_hours', 'informational', 'public_labeled', 'scheduled', 1, '09:00', '10:00', 'America/Toronto', DATE '2026-04-01', DATE '2026-04-29', $4, $5, 'owner', 'schedule_manage', 'staff-console', 'staff-console', $4, $5, 'owner', 'schedule_manage', 'staff-console', 'staff-console')
`, facilityKey, "gym-floor", resourceKey, ownerUserID, sessionID); err != nil {
		t.Fatalf("Exec(weekly block) error = %v", err)
	}

	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.schedule_blocks (
    facility_key,
    zone_key,
    resource_key,
    scope,
    schedule_type,
    kind,
    effect,
    visibility,
    status,
    weekday,
    start_time,
    end_time,
    timezone,
    recurrence_start_date,
    created_by_user_id,
    created_by_session_id,
    created_by_role,
    created_by_capability,
    created_trusted_surface_key,
    created_trusted_surface_label,
    updated_by_user_id,
    updated_by_session_id,
    updated_by_role,
    updated_by_capability,
    updated_trusted_surface_key,
    updated_trusted_surface_label
)
VALUES ($1, $2, $3, 'resource', 'weekly', 'operating_hours', 'informational', 'public_labeled', 'scheduled', 1, '09:00', '10:00', 'America/Toronto', DATE '2026-04-01', $4, $5, 'owner', 'schedule_manage', 'staff-console', 'staff-console', $4, $5, 'owner', 'schedule_manage', 'staff-console', 'staff-console')
`, facilityKey, "gym-floor", resourceKey, ownerUserID, sessionID); err == nil {
		t.Fatal("Exec(weekly block without recurrence_end_date) error = nil, want check violation")
	}
}

func insertScheduleResource(t *testing.T, ctx context.Context, postgresEnv *testutil.PostgresEnv, resourceKey string, facilityKey string, zoneKey string, resourceType string, displayName string) string {
	t.Helper()

	var insertedKey string
	if err := postgresEnv.DB.QueryRow(ctx, `
INSERT INTO apollo.schedule_resources (
    resource_key,
    facility_key,
    zone_key,
    resource_type,
    display_name
)
VALUES ($1, $2, $3, $4, $5)
RETURNING resource_key
`, resourceKey, facilityKey, zoneKey, resourceType, displayName).Scan(&insertedKey); err != nil {
		t.Fatalf("insert schedule resource error = %v", err)
	}
	return insertedKey
}

func insertScheduleUser(t *testing.T, ctx context.Context, postgresEnv *testutil.PostgresEnv, studentID string, displayName string, email string) uuid.UUID {
	t.Helper()

	var userID uuid.UUID
	if err := postgresEnv.DB.QueryRow(ctx, `
INSERT INTO apollo.users (student_id, display_name, email)
VALUES ($1, $2, $3)
RETURNING id
`, studentID, displayName, email).Scan(&userID); err != nil {
		t.Fatalf("insert schedule user error = %v", err)
	}
	return userID
}

func insertScheduleSession(t *testing.T, ctx context.Context, postgresEnv *testutil.PostgresEnv, ownerUserID uuid.UUID) uuid.UUID {
	t.Helper()

	var sessionID uuid.UUID
	if err := postgresEnv.DB.QueryRow(ctx, `
INSERT INTO apollo.sessions (user_id, expires_at, revoked_at)
VALUES ($1, NOW() + INTERVAL '1 hour', NULL)
RETURNING id
`, ownerUserID).Scan(&sessionID); err != nil {
		t.Fatalf("insert schedule session error = %v", err)
	}
	return sessionID
}
