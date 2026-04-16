package migrations

import (
	"context"
	"testing"
	"time"

	"github.com/ixxet/apollo/internal/testutil"
)

func TestBookingRequestSchemaRequiresRequestStateBeforeScheduleBlock(t *testing.T) {
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

	ownerUserID := insertScheduleUser(t, ctx, postgresEnv, "booking-owner-001", "Booking Owner", "booking-owner-001@example.com")
	sessionID := insertScheduleSession(t, ctx, postgresEnv, ownerUserID)
	resourceKey := insertScheduleResource(t, ctx, postgresEnv, "booking-court-101", "ashtonbee", "gym-floor", "court", "Booking Court 101")

	var requestID string
	if err := postgresEnv.DB.QueryRow(ctx, `
INSERT INTO apollo.booking_requests (
    facility_key,
    zone_key,
    resource_key,
    scope,
    requested_start_at,
    requested_end_at,
    contact_name,
    contact_email,
    status,
    created_by_user_id,
    created_by_session_id,
    created_by_role,
    created_by_capability,
    created_trusted_surface_key,
    updated_by_user_id,
    updated_by_session_id,
    updated_by_role,
    updated_by_capability,
    updated_trusted_surface_key
)
VALUES ($1, $2, $3, 'resource', $4, $5, 'Casey Booker', 'casey@example.com', 'requested', $6, $7, 'manager', 'booking_manage', 'staff-console', $6, $7, 'manager', 'booking_manage', 'staff-console')
RETURNING id
`, "ashtonbee", "gym-floor", resourceKey, time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC), time.Date(2026, 4, 18, 15, 0, 0, 0, time.UTC), ownerUserID, sessionID).Scan(&requestID); err != nil {
		t.Fatalf("Exec(valid booking request) error = %v", err)
	}
	if requestID == "" {
		t.Fatal("requestID is empty")
	}

	var prematureScheduleBlockID string
	if err := postgresEnv.DB.QueryRow(ctx, `
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
    start_at,
    end_at,
    created_by_user_id,
    created_by_session_id,
    created_by_role,
    created_by_capability,
    created_trusted_surface_key,
    updated_by_user_id,
    updated_by_session_id,
    updated_by_role,
    updated_by_capability,
    updated_trusted_surface_key
)
VALUES ($1, $2, $3, 'resource', 'one_off', 'reservation', 'hard_reserve', 'internal', 'scheduled', $4, $5, $6, $7, 'manager', 'booking_manage', 'staff-console', $6, $7, 'manager', 'booking_manage', 'staff-console')
RETURNING id
`, "ashtonbee", "gym-floor", resourceKey, time.Date(2026, 4, 18, 18, 0, 0, 0, time.UTC), time.Date(2026, 4, 18, 19, 0, 0, 0, time.UTC), ownerUserID, sessionID).Scan(&prematureScheduleBlockID); err != nil {
		t.Fatalf("Exec(premature schedule block) error = %v", err)
	}

	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.booking_requests (
    facility_key,
    scope,
    requested_start_at,
    requested_end_at,
    contact_name,
    status,
    created_by_user_id,
    created_by_session_id,
    created_by_role,
    created_by_capability,
    created_trusted_surface_key,
    updated_by_user_id,
    updated_by_session_id,
    updated_by_role,
    updated_by_capability,
    updated_trusted_surface_key
)
VALUES ($1, 'facility', $2, $3, 'No Contact', 'requested', $4, $5, 'manager', 'booking_manage', 'staff-console', $4, $5, 'manager', 'booking_manage', 'staff-console')
`, "ashtonbee", time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC), time.Date(2026, 4, 18, 15, 0, 0, 0, time.UTC), ownerUserID, sessionID); err == nil {
		t.Fatal("Exec(request without contact channel) error = nil, want check violation")
	}

	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.booking_requests (
    facility_key,
    scope,
    requested_start_at,
    requested_end_at,
    contact_name,
    contact_email,
    status,
    created_by_user_id,
    created_by_session_id,
    created_by_role,
    created_by_capability,
    created_trusted_surface_key,
    updated_by_user_id,
    updated_by_session_id,
    updated_by_role,
    updated_by_capability,
    updated_trusted_surface_key
)
VALUES ($1, 'facility', $2, $3, 'Premature Approval', 'approval@example.com', 'approved', $4, $5, 'manager', 'booking_manage', 'staff-console', $4, $5, 'manager', 'booking_manage', 'staff-console')
`, "ashtonbee", time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC), time.Date(2026, 4, 18, 15, 0, 0, 0, time.UTC), ownerUserID, sessionID); err == nil {
		t.Fatal("Exec(approved request without linked block) error = nil, want check violation")
	}

	var scheduleBlockID string
	if err := postgresEnv.DB.QueryRow(ctx, `
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
    start_at,
    end_at,
    created_by_user_id,
    created_by_session_id,
    created_by_role,
    created_by_capability,
    created_trusted_surface_key,
    updated_by_user_id,
    updated_by_session_id,
    updated_by_role,
    updated_by_capability,
    updated_trusted_surface_key
)
VALUES ($1, $2, $3, 'resource', 'one_off', 'reservation', 'hard_reserve', 'internal', 'cancelled', $4, $5, $6, $7, 'manager', 'booking_manage', 'staff-console', $6, $7, 'manager', 'booking_manage', 'staff-console')
RETURNING id
`, "ashtonbee", "gym-floor", resourceKey, time.Date(2026, 4, 18, 16, 0, 0, 0, time.UTC), time.Date(2026, 4, 18, 17, 0, 0, 0, time.UTC), ownerUserID, sessionID).Scan(&scheduleBlockID); err != nil {
		t.Fatalf("Exec(cancelled schedule block) error = %v", err)
	}

	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.booking_requests (
    facility_key,
    zone_key,
    resource_key,
    scope,
    requested_start_at,
    requested_end_at,
    contact_name,
    contact_email,
    status,
    schedule_block_id,
    created_by_user_id,
    created_by_session_id,
    created_by_role,
    created_by_capability,
    created_trusted_surface_key,
    updated_by_user_id,
    updated_by_session_id,
    updated_by_role,
    updated_by_capability,
    updated_trusted_surface_key
)
VALUES ($1, $2, $3, 'resource', $4, $5, 'Cancelled Approval', 'cancelled@example.com', 'cancelled', $6, $7, $8, 'manager', 'booking_manage', 'staff-console', $7, $8, 'manager', 'booking_manage', 'staff-console')
`, "ashtonbee", "gym-floor", resourceKey, time.Date(2026, 4, 18, 16, 0, 0, 0, time.UTC), time.Date(2026, 4, 18, 17, 0, 0, 0, time.UTC), scheduleBlockID, ownerUserID, sessionID); err != nil {
		t.Fatalf("Exec(cancelled request retaining schedule block) error = %v", err)
	}

	if _, err := postgresEnv.DB.Exec(ctx, `
INSERT INTO apollo.booking_requests (
    facility_key,
    zone_key,
    resource_key,
    scope,
    requested_start_at,
    requested_end_at,
    contact_name,
    contact_email,
    status,
    schedule_block_id,
    created_by_user_id,
    created_by_session_id,
    created_by_role,
    created_by_capability,
    created_trusted_surface_key,
    updated_by_user_id,
    updated_by_session_id,
    updated_by_role,
    updated_by_capability,
    updated_trusted_surface_key
)
VALUES ($1, $2, $3, 'resource', $4, $5, 'Premature Link', 'premature-link@example.com', 'requested', $6, $7, $8, 'manager', 'booking_manage', 'staff-console', $7, $8, 'manager', 'booking_manage', 'staff-console')
`, "ashtonbee", "gym-floor", resourceKey, time.Date(2026, 4, 18, 18, 0, 0, 0, time.UTC), time.Date(2026, 4, 18, 19, 0, 0, 0, time.UTC), prematureScheduleBlockID, ownerUserID, sessionID); err == nil {
		t.Fatal("Exec(requested request retaining schedule block) error = nil, want check violation")
	}
}
