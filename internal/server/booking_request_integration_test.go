package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/booking"
	"github.com/ixxet/apollo/internal/schedule"
)

func TestBookingRequestAuthzEnforcesStaffMatrixAndTrustedSurface(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-booking-authz-001", "booking-authz-001@example.com")
	managerCookie, manager := createVerifiedSessionViaHTTP(t, env, "student-booking-authz-002", "booking-authz-002@example.com")
	supervisorCookie, supervisor := createVerifiedSessionViaHTTP(t, env, "student-booking-authz-003", "booking-authz-003@example.com")
	memberCookie, _ := createVerifiedSessionViaHTTP(t, env, "student-booking-authz-004", "booking-authz-004@example.com")

	setUserRole(t, env, owner.ID, authz.RoleOwner)
	setUserRole(t, env, manager.ID, authz.RoleManager)
	setUserRole(t, env, supervisor.ID, authz.RoleSupervisor)
	resourceKey := insertBookingHTTPResource(t, env, "booking-authz-court")
	body := bookingRequestJSON(resourceKey, "2026-04-18T14:00:00Z", "2026-04-18T15:00:00Z")

	if response := env.doRequest(t, http.MethodGet, "/api/v1/booking/requests", nil); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated read code = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if response := env.doRequest(t, http.MethodGet, "/api/v1/booking/requests", nil, memberCookie); response.Code != http.StatusForbidden {
		t.Fatalf("member read code = %d, want %d", response.Code, http.StatusForbidden)
	}
	if response := env.doRequest(t, http.MethodGet, "/api/v1/booking/requests", nil, supervisorCookie); response.Code != http.StatusOK {
		t.Fatalf("supervisor read code = %d, want %d body=%s", response.Code, http.StatusOK, response.Body.String())
	} else if response.Header().Get("X-Apollo-Booking-Can-Manage") != "false" {
		t.Fatalf("supervisor manage hint = %q, want false", response.Header().Get("X-Apollo-Booking-Can-Manage"))
	}
	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests", body, supervisorCookie); response.Code != http.StatusForbidden {
		t.Fatalf("supervisor create code = %d, want %d", response.Code, http.StatusForbidden)
	}
	if response := env.doRequestWithoutTrustedSurface(t, http.MethodPost, "/api/v1/booking/requests", bytes.NewBufferString(body), managerCookie); response.Code != http.StatusForbidden {
		t.Fatalf("manager missing trusted surface create code = %d, want %d", response.Code, http.StatusForbidden)
	}
	invalidHeaders := map[string]string{
		authz.TrustedSurfaceHeader:      env.trustedSurfaceKey,
		authz.TrustedSurfaceTokenHeader: "wrong-secret",
	}
	if response := env.doRequestWithHeaders(t, http.MethodPost, "/api/v1/booking/requests", bytes.NewBufferString(body), invalidHeaders, managerCookie); response.Code != http.StatusForbidden {
		t.Fatalf("manager invalid trusted surface create code = %d, want %d", response.Code, http.StatusForbidden)
	}
	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests", body, managerCookie); response.Code != http.StatusCreated {
		t.Fatalf("manager create code = %d, want %d body=%s", response.Code, http.StatusCreated, response.Body.String())
	} else if response.Header().Get("X-Apollo-Booking-Can-Manage") != "true" {
		t.Fatalf("manager manage hint = %q, want true", response.Header().Get("X-Apollo-Booking-Can-Manage"))
	}
	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests", bookingRequestJSON(resourceKey, "2026-04-18T16:00:00Z", "2026-04-18T17:00:00Z"), ownerCookie); response.Code != http.StatusCreated {
		t.Fatalf("owner create code = %d, want %d body=%s", response.Code, http.StatusCreated, response.Body.String())
	}
}

func TestBookingRequestApprovalCreatesLinkedReservationAndRejectsConflicts(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	managerCookie, manager := createVerifiedSessionViaHTTP(t, env, "student-booking-runtime-001", "booking-runtime-001@example.com")
	setUserRole(t, env, manager.ID, authz.RoleManager)
	fullCourt := insertBookingHTTPResource(t, env, "booking-runtime-full-court")
	halfCourt := insertBookingHTTPResource(t, env, "booking-runtime-half-court")
	insertBookingHTTPResourceEdge(t, env, fullCourt, halfCourt, schedule.EdgeContains)

	fullCreate := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests", bookingRequestJSON(fullCourt, "2026-04-19T14:00:00Z", "2026-04-19T15:00:00Z"), managerCookie)
	if fullCreate.Code != http.StatusCreated {
		t.Fatalf("fullCreate.Code = %d, want %d body=%s", fullCreate.Code, http.StatusCreated, fullCreate.Body.String())
	}
	fullRequest := decodeBookingRequest(t, fullCreate)

	fullApprove := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+fullRequest.ID.String()+"/approve", `{"expected_version":1}`, managerCookie)
	if fullApprove.Code != http.StatusOK {
		t.Fatalf("fullApprove.Code = %d, want %d body=%s", fullApprove.Code, http.StatusOK, fullApprove.Body.String())
	}
	approved := decodeBookingRequest(t, fullApprove)
	if approved.Status != booking.StatusApproved || approved.ScheduleBlockID == nil {
		t.Fatalf("approved status/block = %s/%v, want approved with linked block", approved.Status, approved.ScheduleBlockID)
	}
	assertBookingHTTPReservationBlock(t, env, *approved.ScheduleBlockID, fullCourt)

	halfCreate := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests", bookingRequestJSON(halfCourt, "2026-04-19T14:30:00Z", "2026-04-19T15:30:00Z"), managerCookie)
	if halfCreate.Code != http.StatusCreated {
		t.Fatalf("halfCreate.Code = %d, want %d body=%s", halfCreate.Code, http.StatusCreated, halfCreate.Body.String())
	}
	halfRequest := decodeBookingRequest(t, halfCreate)
	if halfRequest.Availability.Status != booking.AvailabilityConflict || len(halfRequest.Availability.Conflicts) == 0 {
		t.Fatalf("halfRequest.Availability = %#v, want APOLLO schedule conflict", halfRequest.Availability)
	}

	conflictApprove := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+halfRequest.ID.String()+"/approve", `{"expected_version":1}`, managerCookie)
	if conflictApprove.Code != http.StatusConflict {
		t.Fatalf("conflictApprove.Code = %d, want %d body=%s", conflictApprove.Code, http.StatusConflict, conflictApprove.Body.String())
	}
	afterConflict := env.doRequest(t, http.MethodGet, "/api/v1/booking/requests/"+halfRequest.ID.String(), nil, managerCookie)
	if afterConflict.Code != http.StatusOK {
		t.Fatalf("afterConflict.Code = %d, want %d body=%s", afterConflict.Code, http.StatusOK, afterConflict.Body.String())
	}
	unchanged := decodeBookingRequest(t, afterConflict)
	if unchanged.Status != booking.StatusRequested || unchanged.ScheduleBlockID != nil {
		t.Fatalf("conflict mutated request to status=%s schedule_block_id=%v", unchanged.Status, unchanged.ScheduleBlockID)
	}
}

func TestBookingRequestTransitionBoundariesAndValidation(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	managerCookie, manager := createVerifiedSessionViaHTTP(t, env, "student-booking-boundary-001", "booking-boundary-001@example.com")
	setUserRole(t, env, manager.ID, authz.RoleManager)
	resourceKey := insertBookingHTTPResource(t, env, "booking-boundary-court")

	missingContact := `{
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"resource_key":"` + resourceKey + `",
		"requested_start_at":"2026-04-20T14:00:00Z",
		"requested_end_at":"2026-04-20T15:00:00Z",
		"contact_name":"Casey Booker"
	}`
	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests", missingContact, managerCookie); response.Code != http.StatusBadRequest {
		t.Fatalf("missing contact create code = %d, want %d", response.Code, http.StatusBadRequest)
	}
	badWindow := strings.Replace(bookingRequestJSON(resourceKey, "2026-04-20T14:00:00Z", "2026-04-20T15:00:00Z"), "2026-04-20T15:00:00Z", "not-rfc3339", 1)
	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests", badWindow, managerCookie); response.Code != http.StatusBadRequest {
		t.Fatalf("bad window create code = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests", bookingRequestJSON("missing-resource", "2026-04-20T14:00:00Z", "2026-04-20T15:00:00Z"), managerCookie); response.Code != http.StatusNotFound {
		t.Fatalf("missing resource create code = %d, want %d body=%s", response.Code, http.StatusNotFound, response.Body.String())
	}

	createResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests", bookingRequestJSON(resourceKey, "2026-04-20T14:00:00Z", "2026-04-20T15:00:00Z"), managerCookie)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("createResponse.Code = %d, want %d body=%s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}
	created := decodeBookingRequest(t, createResponse)
	assertBookingResponseHasNoForbiddenFields(t, createResponse)

	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+created.ID.String()+"/review", `{}`, managerCookie); response.Code != http.StatusBadRequest {
		t.Fatalf("missing expected_version transition code = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+created.ID.String()+"/review", `{"expected_version":9}`, managerCookie); response.Code != http.StatusConflict {
		t.Fatalf("stale transition code = %d, want %d", response.Code, http.StatusConflict)
	}
	reviewResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+created.ID.String()+"/review", `{"expected_version":1}`, managerCookie)
	if reviewResponse.Code != http.StatusOK {
		t.Fatalf("reviewResponse.Code = %d, want %d body=%s", reviewResponse.Code, http.StatusOK, reviewResponse.Body.String())
	}
	reviewing := decodeBookingRequest(t, reviewResponse)
	if reviewing.Status != booking.StatusUnderReview || reviewing.Version != 2 {
		t.Fatalf("reviewing status/version = %s/%d, want under_review/2", reviewing.Status, reviewing.Version)
	}
	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+created.ID.String()+"/review", `{"expected_version":2}`, managerCookie); response.Code != http.StatusConflict {
		t.Fatalf("invalid repeated review code = %d, want %d", response.Code, http.StatusConflict)
	}

	rejectResource := insertBookingHTTPResource(t, env, "booking-reject-court")
	rejectCreate := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests", bookingRequestJSON(rejectResource, "2026-04-20T16:00:00Z", "2026-04-20T17:00:00Z"), managerCookie)
	rejectedCandidate := decodeBookingRequest(t, rejectCreate)
	rejectResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+rejectedCandidate.ID.String()+"/reject", `{"expected_version":1}`, managerCookie)
	if rejectResponse.Code != http.StatusOK {
		t.Fatalf("rejectResponse.Code = %d, want %d body=%s", rejectResponse.Code, http.StatusOK, rejectResponse.Body.String())
	}
	rejected := decodeBookingRequest(t, rejectResponse)
	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+rejected.ID.String()+"/approve", `{"expected_version":2}`, managerCookie); response.Code != http.StatusConflict {
		t.Fatalf("approve rejected code = %d, want %d", response.Code, http.StatusConflict)
	}

	cancelResource := insertBookingHTTPResource(t, env, "booking-cancel-court")
	cancelCreate := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests", bookingRequestJSON(cancelResource, "2026-04-20T18:00:00Z", "2026-04-20T19:00:00Z"), managerCookie)
	cancelCandidate := decodeBookingRequest(t, cancelCreate)
	cancelResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+cancelCandidate.ID.String()+"/cancel", `{"expected_version":1}`, managerCookie)
	if cancelResponse.Code != http.StatusOK {
		t.Fatalf("cancelResponse.Code = %d, want %d body=%s", cancelResponse.Code, http.StatusOK, cancelResponse.Body.String())
	}
	cancelled := decodeBookingRequest(t, cancelResponse)
	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+cancelled.ID.String()+"/approve", `{"expected_version":2}`, managerCookie); response.Code != http.StatusConflict {
		t.Fatalf("approve cancelled code = %d, want %d", response.Code, http.StatusConflict)
	}
}

func decodeBookingRequest(t *testing.T, response *httptest.ResponseRecorder) booking.Request {
	t.Helper()

	var payload booking.Request
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(booking request) error = %v body=%s", err, response.Body.String())
	}
	return payload
}

func bookingRequestJSON(resourceKey string, startsAt string, endsAt string) string {
	return `{
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"resource_key":"` + resourceKey + `",
		"requested_start_at":"` + startsAt + `",
		"requested_end_at":"` + endsAt + `",
		"contact_name":"Casey Booker",
		"contact_email":"casey@example.com",
		"organization":"Ashton Staff Intake",
		"purpose":"Court request",
		"attendee_count":8,
		"internal_notes":"entered by staff"
	}`
}

func assertBookingHTTPReservationBlock(t *testing.T, env *authProfileServerEnv, blockID uuid.UUID, resourceKey string) {
	t.Helper()

	var kind, effect, visibility, capability, storedResourceKey string
	if err := env.db.DB.QueryRow(context.Background(), `
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

func assertBookingResponseHasNoForbiddenFields(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()

	body := response.Body.String()
	for _, forbidden := range []string{"payment", "quote", "invoice", "deposit", "checkout", "card", "bank", "public_booking"} {
		if strings.Contains(strings.ToLower(body), forbidden) {
			t.Fatalf("booking response leaked forbidden field %q in body %s", forbidden, body)
		}
	}
}

func insertBookingHTTPResource(t *testing.T, env *authProfileServerEnv, resourceKey string) string {
	t.Helper()

	var insertedKey string
	if err := env.db.DB.QueryRow(context.Background(), `
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
		t.Fatalf("insert booking HTTP resource error = %v", err)
	}
	return insertedKey
}

func insertBookingHTTPResourceEdge(t *testing.T, env *authProfileServerEnv, resourceKey string, relatedResourceKey string, edgeType string) {
	t.Helper()

	if _, err := env.db.DB.Exec(context.Background(), `
INSERT INTO apollo.schedule_resource_edges (
    resource_key,
    related_resource_key,
    edge_type
)
VALUES ($1, $2, $3)
`, resourceKey, relatedResourceKey, edgeType); err != nil {
		t.Fatalf("insert booking HTTP resource edge error = %v", err)
	}
}
