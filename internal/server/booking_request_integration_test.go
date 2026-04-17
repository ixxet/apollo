package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

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

func TestBookingRequestStaffCreateIdempotency(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	managerCookie, manager := createVerifiedSessionViaHTTP(t, env, "student-booking-staff-idem-001", "booking-staff-idem-001@example.com")
	setUserRole(t, env, manager.ID, authz.RoleManager)
	resourceKey := insertBookingHTTPResource(t, env, "booking-staff-idem-court")
	email := "staff-idempotent@example.com"
	body := bookingRequestJSONWithEmail(resourceKey, "2026-04-18T18:00:00Z", "2026-04-18T19:00:00Z", email)
	headers := staffBookingIdempotencyHeaders(env, "staff-idem-key-001")

	first := env.doRequestWithHeaders(t, http.MethodPost, "/api/v1/booking/requests", bytes.NewBufferString(body), headers, managerCookie)
	if first.Code != http.StatusCreated {
		t.Fatalf("first staff idempotent create code = %d, want %d body=%s", first.Code, http.StatusCreated, first.Body.String())
	}
	firstRequest := decodeBookingRequest(t, first)

	duplicate := env.doRequestWithHeaders(t, http.MethodPost, "/api/v1/booking/requests", bytes.NewBufferString(body), headers, managerCookie)
	if duplicate.Code != http.StatusCreated {
		t.Fatalf("duplicate staff idempotent create code = %d, want %d body=%s", duplicate.Code, http.StatusCreated, duplicate.Body.String())
	}
	duplicateRequest := decodeBookingRequest(t, duplicate)
	if duplicateRequest.ID != firstRequest.ID {
		t.Fatalf("duplicate staff idempotent create ID = %s, want %s", duplicateRequest.ID, firstRequest.ID)
	}
	if count := countBookingHTTPRequestsByEmail(t, env, email); count != 1 {
		t.Fatalf("staff idempotent request count = %d, want 1", count)
	}

	changed := bookingRequestJSONWithEmail(resourceKey, "2026-04-18T19:30:00Z", "2026-04-18T20:30:00Z", email)
	conflict := env.doRequestWithHeaders(t, http.MethodPost, "/api/v1/booking/requests", bytes.NewBufferString(changed), headers, managerCookie)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("changed staff idempotent create code = %d, want %d body=%s", conflict.Code, http.StatusConflict, conflict.Body.String())
	}
	if count := countBookingHTTPRequestsByEmail(t, env, email); count != 1 {
		t.Fatalf("staff idempotent request count after conflict = %d, want 1", count)
	}
}

func TestBookingRequestStaffCreateConcurrentSameKeySerializesDuplicate(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	managerCookie, manager := createVerifiedSessionViaHTTP(t, env, "student-booking-staff-concurrent-001", "booking-staff-concurrent-001@example.com")
	setUserRole(t, env, manager.ID, authz.RoleManager)
	resourceKey := insertBookingHTTPResource(t, env, "booking-staff-concurrent-court")
	email := "staff-concurrent@example.com"
	body := bookingRequestJSONWithEmail(resourceKey, "2026-04-18T20:00:00Z", "2026-04-18T21:00:00Z", email)
	headers := staffBookingIdempotencyHeaders(env, "staff-concurrent-key-001")

	responses := doConcurrentStaffBookingCreates(env, managerCookie, headers, body, body, body, body, body)
	var firstID uuid.UUID
	for index, response := range responses {
		if response.Code != http.StatusCreated {
			t.Fatalf("concurrent staff create %d code = %d, want %d body=%s", index, response.Code, http.StatusCreated, response.Body.String())
		}
		request := decodeBookingRequest(t, response)
		if index == 0 {
			firstID = request.ID
			continue
		}
		if request.ID != firstID {
			t.Fatalf("concurrent staff create %d ID = %s, want %s", index, request.ID, firstID)
		}
	}
	if count := countBookingHTTPRequestsByEmail(t, env, email); count != 1 {
		t.Fatalf("concurrent staff request count = %d, want 1", count)
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

func TestPublicBookingOptionsExposeOnlySafePublicLabels(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	publicOptionID := insertPublicBookingHTTPResource(t, env, "booking-public-options-court", "Community Court", true, true)
	insertPublicBookingHTTPResource(t, env, "booking-public-options-inactive", "Inactive Court", true, false)
	insertPublicBookingHTTPResource(t, env, "booking-public-options-unbookable", "Unbookable Court", false, true)
	insertBookingHTTPResource(t, env, "booking-public-options-unlabeled")

	response := env.doRequest(t, http.MethodGet, "/api/v1/public/booking/options", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("public options code = %d, want %d body=%s", response.Code, http.StatusOK, response.Body.String())
	}

	var options []booking.PublicOption
	if err := json.Unmarshal(response.Body.Bytes(), &options); err != nil {
		t.Fatalf("json.Unmarshal(public options) error = %v body=%s", err, response.Body.String())
	}
	if len(options) != 1 {
		t.Fatalf("public options len = %d, want 1: %#v", len(options), options)
	}
	if options[0].OptionID != publicOptionID || options[0].Label != "Community Court" {
		t.Fatalf("public option = %#v, want %s/Community Court", options[0], publicOptionID)
	}
	assertPublicBookingResponseSafe(t, response)
	for _, forbidden := range []string{"booking-public-options-court", "booking-public-options-inactive", "booking-public-options-unbookable", "resource_key", "facility_key", "zone_key", "display_name", "schedule_block", "conflict", "created_by", "trusted_surface"} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("public options leaked %q in body %s", forbidden, response.Body.String())
		}
	}
}

func TestPublicBookingSubmitCreatesRequestedRequestWithoutReservationAndIsIdempotent(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	optionID := insertPublicBookingHTTPResource(t, env, "booking-public-submit-court", "Court request", true, true)
	beforeBlocks := countBookingHTTPScheduleBlocks(t, env)
	start, end := publicBookingWindow(72*time.Hour, time.Hour)
	body := publicBookingRequestJSON(optionID, start, end, `"contact_email":"public-booking@example.com"`)
	headers := map[string]string{
		"Idempotency-Key":                "public-submit-key-001",
		"X-Apollo-Public-Intake-Channel": booking.IntakeChannelPublicWeb,
	}

	response := env.doRequestWithHeaders(t, http.MethodPost, "/api/v1/public/booking/requests", bytes.NewBufferString(body), headers)
	if response.Code != http.StatusAccepted {
		t.Fatalf("public submit code = %d, want %d body=%s", response.Code, http.StatusAccepted, response.Body.String())
	}
	var receipt booking.PublicReceipt
	if err := json.Unmarshal(response.Body.Bytes(), &receipt); err != nil {
		t.Fatalf("json.Unmarshal(public receipt) error = %v body=%s", err, response.Body.String())
	}
	if receipt.Status != "received" {
		t.Fatalf("receipt.Status = %q, want received", receipt.Status)
	}
	assertPublicBookingResponseSafe(t, response)

	var status, source, channel string
	var scheduleBlockNull, createdUserNull, createdSessionNull, createdRoleNull, createdCapabilityNull, createdSurfaceNull bool
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT status,
       request_source,
       intake_channel,
       schedule_block_id IS NULL,
       created_by_user_id IS NULL,
       created_by_session_id IS NULL,
       created_by_role IS NULL,
       created_by_capability IS NULL,
       created_trusted_surface_key IS NULL
FROM apollo.booking_requests
WHERE contact_email = 'public-booking@example.com'
`).Scan(&status, &source, &channel, &scheduleBlockNull, &createdUserNull, &createdSessionNull, &createdRoleNull, &createdCapabilityNull, &createdSurfaceNull); err != nil {
		t.Fatalf("QueryRow(public booking request) error = %v", err)
	}
	if status != booking.StatusRequested || source != booking.RequestSourcePublic || channel != booking.IntakeChannelPublicWeb {
		t.Fatalf("public request status/source/channel = %s/%s/%s, want requested/public/public_web", status, source, channel)
	}
	if !scheduleBlockNull {
		t.Fatal("public submit created schedule_block_id, want nil")
	}
	if !createdUserNull || !createdSessionNull || !createdRoleNull || !createdCapabilityNull || !createdSurfaceNull {
		t.Fatalf("public request has staff creation attribution: user=%t session=%t role=%t cap=%t surface=%t", createdUserNull, createdSessionNull, createdRoleNull, createdCapabilityNull, createdSurfaceNull)
	}
	if afterBlocks := countBookingHTTPScheduleBlocks(t, env); afterBlocks != beforeBlocks {
		t.Fatalf("schedule block count = %d, want unchanged %d", afterBlocks, beforeBlocks)
	}

	duplicate := env.doRequestWithHeaders(t, http.MethodPost, "/api/v1/public/booking/requests", bytes.NewBufferString(body), headers)
	if duplicate.Code != http.StatusAccepted {
		t.Fatalf("duplicate submit code = %d, want %d body=%s", duplicate.Code, http.StatusAccepted, duplicate.Body.String())
	}
	if count := countBookingHTTPRequestsByEmail(t, env, "public-booking@example.com"); count != 1 {
		t.Fatalf("public booking request count = %d, want 1", count)
	}

	changedStart, changedEnd := publicBookingWindow(74*time.Hour, time.Hour)
	changedBody := publicBookingRequestJSON(optionID, changedStart, changedEnd, `"contact_email":"public-booking@example.com"`)
	conflict := env.doRequestWithHeaders(t, http.MethodPost, "/api/v1/public/booking/requests", bytes.NewBufferString(changedBody), headers)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("changed idempotency submit code = %d, want %d body=%s", conflict.Code, http.StatusConflict, conflict.Body.String())
	}
	if count := countBookingHTTPRequestsByEmail(t, env, "public-booking@example.com"); count != 1 {
		t.Fatalf("public booking request count after conflict = %d, want 1", count)
	}

	managerCookie, manager := createVerifiedSessionViaHTTP(t, env, "student-public-staff-001", "public-staff-001@example.com")
	setUserRole(t, env, manager.ID, authz.RoleManager)
	listResponse := env.doRequest(t, http.MethodGet, "/api/v1/booking/requests?facility_key=ashtonbee", nil, managerCookie)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("staff list code = %d, want %d body=%s", listResponse.Code, http.StatusOK, listResponse.Body.String())
	}
	var staffRequests []booking.Request
	if err := json.Unmarshal(listResponse.Body.Bytes(), &staffRequests); err != nil {
		t.Fatalf("json.Unmarshal(staff booking list) error = %v body=%s", err, listResponse.Body.String())
	}
	if len(staffRequests) == 0 || staffRequests[0].RequestSource != booking.RequestSourcePublic || staffRequests[0].IntakeChannel != booking.IntakeChannelPublicWeb {
		t.Fatalf("staff source/channel = %#v, want public/public_web visible", staffRequests)
	}
}

func TestPublicBookingSubmitConcurrentSameKeySerializesDuplicate(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	optionID := insertPublicBookingHTTPResource(t, env, "booking-public-concurrent-court", "Concurrent Court", true, true)
	beforeBlocks := countBookingHTTPScheduleBlocks(t, env)
	start, end := publicBookingWindow(72*time.Hour, time.Hour)
	body := publicBookingRequestJSON(optionID, start, end, `"contact_email":"public-concurrent@example.com"`)
	headers := map[string]string{
		"Idempotency-Key":                "public-concurrent-key-001",
		"X-Apollo-Public-Intake-Channel": booking.IntakeChannelPublicWeb,
	}

	responses := doConcurrentPublicBookingSubmits(env, headers, body, body, body, body, body)
	for index, response := range responses {
		if response.Code != http.StatusAccepted {
			t.Fatalf("concurrent duplicate submit %d code = %d, want %d body=%s", index, response.Code, http.StatusAccepted, response.Body.String())
		}
		assertPublicBookingResponseSafe(t, response)
	}
	if count := countBookingHTTPRequestsByEmail(t, env, "public-concurrent@example.com"); count != 1 {
		t.Fatalf("concurrent public booking request count = %d, want 1", count)
	}
	if afterBlocks := countBookingHTTPScheduleBlocks(t, env); afterBlocks != beforeBlocks {
		t.Fatalf("schedule block count = %d, want unchanged %d", afterBlocks, beforeBlocks)
	}
}

func TestPublicBookingSubmitConcurrentSameKeyDifferentPayloadConflicts(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	optionID := insertPublicBookingHTTPResource(t, env, "booking-public-concurrent-conflict-court", "Concurrent Conflict Court", true, true)
	start, end := publicBookingWindow(72*time.Hour, time.Hour)
	changedStart, changedEnd := publicBookingWindow(74*time.Hour, time.Hour)
	body := publicBookingRequestJSON(optionID, start, end, `"contact_email":"public-concurrent-conflict@example.com"`)
	changedBody := publicBookingRequestJSON(optionID, changedStart, changedEnd, `"contact_email":"public-concurrent-conflict@example.com"`)
	headers := map[string]string{
		"Idempotency-Key":                "public-concurrent-conflict-key-001",
		"X-Apollo-Public-Intake-Channel": booking.IntakeChannelPublicWeb,
	}

	responses := doConcurrentPublicBookingSubmits(env, headers, body, changedBody)
	accepted := 0
	conflicted := 0
	for index, response := range responses {
		switch response.Code {
		case http.StatusAccepted:
			accepted++
		case http.StatusConflict:
			conflicted++
		default:
			t.Fatalf("concurrent changed submit %d code = %d, want accepted or conflict body=%s", index, response.Code, response.Body.String())
		}
		assertPublicBookingResponseSafe(t, response)
	}
	if accepted != 1 || conflicted != 1 {
		t.Fatalf("concurrent changed submit accepted/conflicted = %d/%d, want 1/1", accepted, conflicted)
	}
	if count := countBookingHTTPRequestsByEmail(t, env, "public-concurrent-conflict@example.com"); count != 1 {
		t.Fatalf("concurrent changed public booking request count = %d, want 1", count)
	}
}

func TestPublicBookingSubmitValidationAndPrivateBoundaries(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	optionID := insertPublicBookingHTTPResource(t, env, "booking-public-validation-court", "Validation Court", true, true)
	inactiveOptionID := insertPublicBookingHTTPResource(t, env, "booking-public-validation-inactive", "Inactive Validation Court", true, false)
	unbookableOptionID := insertPublicBookingHTTPResource(t, env, "booking-public-validation-unbookable", "Unbookable Validation Court", false, true)
	unlabeledOptionID := insertPrivateBookingHTTPResourceWithOption(t, env, "booking-public-validation-unlabeled")
	start, end := publicBookingWindow(96*time.Hour, time.Hour)
	validBody := publicBookingRequestJSON(optionID, start, end, `"contact_email":"validation@example.com"`)

	if response := env.doRequest(t, http.MethodPost, "/api/v1/public/booking/requests", bytes.NewBufferString(validBody)); response.Code != http.StatusBadRequest {
		t.Fatalf("missing idempotency code = %d, want %d body=%s", response.Code, http.StatusBadRequest, response.Body.String())
	}
	unknownField := strings.Replace(validBody, `"attendee_count":8`, `"attendee_count":8,"internal_notes":"do not accept"`, 1)
	if response := env.doRequestWithHeaders(t, http.MethodPost, "/api/v1/public/booking/requests", bytes.NewBufferString(unknownField), map[string]string{"Idempotency-Key": "public-validation-unknown"}); response.Code != http.StatusBadRequest {
		t.Fatalf("unknown public field code = %d, want %d body=%s", response.Code, http.StatusBadRequest, response.Body.String())
	}

	testCases := []struct {
		name string
		body string
		code int
	}{
		{
			name: "missing option",
			body: strings.Replace(validBody, `"option_id":"`+optionID.String()+`",`, "", 1),
			code: http.StatusBadRequest,
		},
		{
			name: "date only",
			body: publicBookingRequestJSON(optionID, "2026-04-20", end, `"contact_email":"date-only@example.com"`),
			code: http.StatusBadRequest,
		},
		{
			name: "reversed window",
			body: publicBookingRequestJSON(optionID, end, start, `"contact_email":"reversed@example.com"`),
			code: http.StatusBadRequest,
		},
		{
			name: "past window",
			body: publicBookingRequestJSON(optionID, time.Now().UTC().Add(-48*time.Hour).Format(time.RFC3339), time.Now().UTC().Add(-47*time.Hour).Format(time.RFC3339), `"contact_email":"past@example.com"`),
			code: http.StatusBadRequest,
		},
		{
			name: "far future window",
			body: publicBookingRequestJSON(optionID, time.Now().UTC().Add(181*24*time.Hour).Format(time.RFC3339), time.Now().UTC().Add(181*24*time.Hour+time.Hour).Format(time.RFC3339), `"contact_email":"far@example.com"`),
			code: http.StatusBadRequest,
		},
		{
			name: "oversized duration",
			body: publicBookingRequestJSON(optionID, start, time.Now().UTC().Add(96*time.Hour+9*time.Hour).Format(time.RFC3339), `"contact_email":"duration@example.com"`),
			code: http.StatusBadRequest,
		},
		{
			name: "bad email",
			body: publicBookingRequestJSON(optionID, start, end, `"contact_email":"not an email"`),
			code: http.StatusBadRequest,
		},
		{
			name: "bad phone",
			body: publicBookingRequestJSON(optionID, start, end, `"contact_phone":"abc"`),
			code: http.StatusBadRequest,
		},
		{
			name: "no contact",
			body: publicBookingRequestJSON(optionID, start, end, `"organization":"No Contact"`),
			code: http.StatusBadRequest,
		},
		{
			name: "long field",
			body: publicBookingRequestJSON(optionID, start, end, `"contact_email":"long@example.com","contact_name":"`+strings.Repeat("x", 121)+`"`),
			code: http.StatusBadRequest,
		},
		{
			name: "invalid attendee count",
			body: strings.Replace(publicBookingRequestJSON(optionID, start, end, `"contact_email":"attendee@example.com"`), `"attendee_count":8`, `"attendee_count":0`, 1),
			code: http.StatusBadRequest,
		},
		{
			name: "inactive option",
			body: publicBookingRequestJSON(inactiveOptionID, start, end, `"contact_email":"inactive@example.com"`),
			code: http.StatusNotFound,
		},
		{
			name: "unbookable option",
			body: publicBookingRequestJSON(unbookableOptionID, start, end, `"contact_email":"unbookable@example.com"`),
			code: http.StatusNotFound,
		},
		{
			name: "unlabeled option",
			body: publicBookingRequestJSON(unlabeledOptionID, start, end, `"contact_email":"unlabeled@example.com"`),
			code: http.StatusNotFound,
		},
		{
			name: "missing option id",
			body: publicBookingRequestJSON(uuid.New(), start, end, `"contact_email":"missing-option@example.com"`),
			code: http.StatusNotFound,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			headers := map[string]string{"Idempotency-Key": "public-validation-" + strings.ReplaceAll(testCase.name, " ", "-")}
			response := env.doRequestWithHeaders(t, http.MethodPost, "/api/v1/public/booking/requests", bytes.NewBufferString(testCase.body), headers)
			if response.Code != testCase.code {
				t.Fatalf("code = %d, want %d body=%s", response.Code, testCase.code, response.Body.String())
			}
			assertPublicBookingResponseSafe(t, response)
		})
	}
}

func TestBookingRequestApprovedCancellationCancelsLinkedReservationHTTP(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	managerCookie, manager := createVerifiedSessionViaHTTP(t, env, "student-booking-cancel-001", "booking-cancel-001@example.com")
	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-booking-cancel-002", "booking-cancel-002@example.com")
	supervisorCookie, supervisor := createVerifiedSessionViaHTTP(t, env, "student-booking-cancel-003", "booking-cancel-003@example.com")
	setUserRole(t, env, manager.ID, authz.RoleManager)
	setUserRole(t, env, owner.ID, authz.RoleOwner)
	setUserRole(t, env, supervisor.ID, authz.RoleSupervisor)

	managerResource := insertBookingHTTPResource(t, env, "booking-http-manager-cancel-court")
	managerCreate := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests", bookingRequestJSON(managerResource, "2026-04-21T14:00:00Z", "2026-04-21T15:00:00Z"), managerCookie)
	if managerCreate.Code != http.StatusCreated {
		t.Fatalf("managerCreate.Code = %d, want %d body=%s", managerCreate.Code, http.StatusCreated, managerCreate.Body.String())
	}
	managerRequest := decodeBookingRequest(t, managerCreate)
	managerApprove := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+managerRequest.ID.String()+"/approve", `{"expected_version":1}`, managerCookie)
	if managerApprove.Code != http.StatusOK {
		t.Fatalf("managerApprove.Code = %d, want %d body=%s", managerApprove.Code, http.StatusOK, managerApprove.Body.String())
	}
	managerApproved := decodeBookingRequest(t, managerApprove)
	if managerApproved.ScheduleBlockID == nil {
		t.Fatal("managerApproved.ScheduleBlockID = nil, want linked schedule block")
	}

	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+managerApproved.ID.String()+"/cancel", `{"expected_version":1}`, managerCookie); response.Code != http.StatusConflict {
		t.Fatalf("stale approved cancel code = %d, want %d body=%s", response.Code, http.StatusConflict, response.Body.String())
	}
	assertBookingHTTPReservationScheduled(t, env, *managerApproved.ScheduleBlockID)

	if response := env.doRequestWithoutTrustedSurface(t, http.MethodPost, "/api/v1/booking/requests/"+managerApproved.ID.String()+"/cancel", bytes.NewBufferString(`{"expected_version":2}`), managerCookie); response.Code != http.StatusForbidden {
		t.Fatalf("missing trusted approved cancel code = %d, want %d body=%s", response.Code, http.StatusForbidden, response.Body.String())
	}
	assertBookingHTTPReservationScheduled(t, env, *managerApproved.ScheduleBlockID)

	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+managerApproved.ID.String()+"/cancel", `{"expected_version":2}`, supervisorCookie); response.Code != http.StatusForbidden {
		t.Fatalf("supervisor approved cancel code = %d, want %d body=%s", response.Code, http.StatusForbidden, response.Body.String())
	}
	assertBookingHTTPReservationScheduled(t, env, *managerApproved.ScheduleBlockID)

	managerCancel := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+managerApproved.ID.String()+"/cancel", `{"expected_version":2}`, managerCookie)
	if managerCancel.Code != http.StatusOK {
		t.Fatalf("managerCancel.Code = %d, want %d body=%s", managerCancel.Code, http.StatusOK, managerCancel.Body.String())
	}
	managerCancelled := decodeBookingRequest(t, managerCancel)
	if managerCancelled.Status != booking.StatusCancelled || managerCancelled.ScheduleBlockID == nil || *managerCancelled.ScheduleBlockID != *managerApproved.ScheduleBlockID {
		t.Fatalf("managerCancelled status/block = %s/%v, want cancelled with retained block %v", managerCancelled.Status, managerCancelled.ScheduleBlockID, managerApproved.ScheduleBlockID)
	}
	assertBookingHTTPReservationCancelled(t, env, *managerApproved.ScheduleBlockID)

	ownerResource := insertBookingHTTPResource(t, env, "booking-http-owner-cancel-court")
	ownerCreate := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests", bookingRequestJSON(ownerResource, "2026-04-21T16:00:00Z", "2026-04-21T17:00:00Z"), ownerCookie)
	if ownerCreate.Code != http.StatusCreated {
		t.Fatalf("ownerCreate.Code = %d, want %d body=%s", ownerCreate.Code, http.StatusCreated, ownerCreate.Body.String())
	}
	ownerRequest := decodeBookingRequest(t, ownerCreate)
	ownerApprove := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+ownerRequest.ID.String()+"/approve", `{"expected_version":1}`, ownerCookie)
	if ownerApprove.Code != http.StatusOK {
		t.Fatalf("ownerApprove.Code = %d, want %d body=%s", ownerApprove.Code, http.StatusOK, ownerApprove.Body.String())
	}
	ownerApproved := decodeBookingRequest(t, ownerApprove)
	ownerCancel := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+ownerApproved.ID.String()+"/cancel", `{"expected_version":2}`, ownerCookie)
	if ownerCancel.Code != http.StatusOK {
		t.Fatalf("ownerCancel.Code = %d, want %d body=%s", ownerCancel.Code, http.StatusOK, ownerCancel.Body.String())
	}
	ownerCancelled := decodeBookingRequest(t, ownerCancel)
	if ownerCancelled.Status != booking.StatusCancelled || ownerCancelled.ScheduleBlockID == nil {
		t.Fatalf("ownerCancelled status/block = %s/%v, want cancelled with retained block", ownerCancelled.Status, ownerCancelled.ScheduleBlockID)
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

func publicBookingWindow(offset time.Duration, duration time.Duration) (string, string) {
	start := time.Now().UTC().Add(offset).Truncate(time.Second)
	end := start.Add(duration)
	return start.Format(time.RFC3339), end.Format(time.RFC3339)
}

func publicBookingRequestJSON(optionID uuid.UUID, startsAt string, endsAt string, contactFields string) string {
	if strings.TrimSpace(contactFields) != "" {
		contactFields = contactFields + ","
	}
	return `{
		"option_id":"` + optionID.String() + `",
		"requested_start_at":"` + startsAt + `",
		"requested_end_at":"` + endsAt + `",
		"contact_name":"Public Booker",
		` + contactFields + `
		"purpose":"Community court request",
		"attendee_count":8
	}`
}

func doConcurrentPublicBookingSubmits(env *authProfileServerEnv, headers map[string]string, bodies ...string) []*httptest.ResponseRecorder {
	var wg sync.WaitGroup
	start := make(chan struct{})
	responses := make([]*httptest.ResponseRecorder, len(bodies))

	for index, body := range bodies {
		wg.Add(1)
		go func(index int, body string) {
			defer wg.Done()
			<-start
			request := httptest.NewRequest(http.MethodPost, "/api/v1/public/booking/requests", bytes.NewBufferString(body))
			request.Header.Set("Content-Type", "application/json")
			for key, value := range headers {
				request.Header.Set(key, value)
			}
			recorder := httptest.NewRecorder()
			env.handler.ServeHTTP(recorder, request)
			responses[index] = recorder
		}(index, body)
	}

	close(start)
	wg.Wait()
	return responses
}

func doConcurrentStaffBookingCreates(env *authProfileServerEnv, cookie *http.Cookie, headers map[string]string, bodies ...string) []*httptest.ResponseRecorder {
	var wg sync.WaitGroup
	start := make(chan struct{})
	responses := make([]*httptest.ResponseRecorder, len(bodies))

	for index, body := range bodies {
		wg.Add(1)
		go func(index int, body string) {
			defer wg.Done()
			<-start
			request := httptest.NewRequest(http.MethodPost, "/api/v1/booking/requests", bytes.NewBufferString(body))
			request.Header.Set("Content-Type", "application/json")
			for key, value := range headers {
				request.Header.Set(key, value)
			}
			request.AddCookie(cookie)
			recorder := httptest.NewRecorder()
			env.handler.ServeHTTP(recorder, request)
			responses[index] = recorder
		}(index, body)
	}

	close(start)
	wg.Wait()
	return responses
}

func staffBookingIdempotencyHeaders(env *authProfileServerEnv, key string) map[string]string {
	return map[string]string{
		authz.TrustedSurfaceHeader:      env.trustedSurfaceKey,
		authz.TrustedSurfaceTokenHeader: env.trustedSurfaceToken,
		"Idempotency-Key":               key,
	}
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

func bookingRequestJSONWithEmail(resourceKey string, startsAt string, endsAt string, email string) string {
	return strings.Replace(bookingRequestJSON(resourceKey, startsAt, endsAt), `"contact_email":"casey@example.com"`, `"contact_email":"`+email+`"`, 1)
}

func assertPublicBookingResponseSafe(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()

	body := strings.ToLower(response.Body.String())
	for _, forbidden := range []string{
		"request_id",
		"booking_request_id",
		"schedule_block_id",
		"conflicts",
		"internal_notes",
		"actor",
		"session_id",
		"created_by",
		"updated_by",
		"trusted_surface",
		"staff",
		"resource_key",
		"facility_key",
		"zone_key",
		"display_name",
		"payment",
		"quote",
		"invoice",
		"deposit",
		"checkout",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("public booking response leaked forbidden field %q in body %s", forbidden, response.Body.String())
		}
	}
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

func assertBookingHTTPReservationScheduled(t *testing.T, env *authProfileServerEnv, blockID uuid.UUID) {
	t.Helper()

	var status string
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT status
FROM apollo.schedule_blocks
WHERE id = $1
`, blockID).Scan(&status); err != nil {
		t.Fatalf("QueryRow(schedule block status) error = %v", err)
	}
	if status != schedule.StatusScheduled {
		t.Fatalf("schedule block status = %q, want %q", status, schedule.StatusScheduled)
	}
}

func assertBookingHTTPReservationCancelled(t *testing.T, env *authProfileServerEnv, blockID uuid.UUID) {
	t.Helper()

	var status, capability string
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT status,
       cancelled_by_capability
FROM apollo.schedule_blocks
WHERE id = $1
`, blockID).Scan(&status, &capability); err != nil {
		t.Fatalf("QueryRow(cancelled schedule block) error = %v", err)
	}
	if status != schedule.StatusCancelled {
		t.Fatalf("schedule block status = %q, want %q", status, schedule.StatusCancelled)
	}
	if capability != string(authz.CapabilityBookingManage) {
		t.Fatalf("cancelled_by_capability = %q, want %q", capability, authz.CapabilityBookingManage)
	}
}

func countBookingHTTPScheduleBlocks(t *testing.T, env *authProfileServerEnv) int {
	t.Helper()

	var count int
	if err := env.db.DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM apollo.schedule_blocks`).Scan(&count); err != nil {
		t.Fatalf("QueryRow(schedule block count) error = %v", err)
	}
	return count
}

func countBookingHTTPRequestsByEmail(t *testing.T, env *authProfileServerEnv, email string) int {
	t.Helper()

	var count int
	if err := env.db.DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM apollo.booking_requests WHERE contact_email = $1`, email).Scan(&count); err != nil {
		t.Fatalf("QueryRow(booking request count) error = %v", err)
	}
	return count
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

func insertPublicBookingHTTPResource(t *testing.T, env *authProfileServerEnv, resourceKey string, publicLabel string, bookable bool, active bool) uuid.UUID {
	t.Helper()

	var optionID uuid.UUID
	if err := env.db.DB.QueryRow(context.Background(), `
INSERT INTO apollo.schedule_resources (
    resource_key,
    facility_key,
    zone_key,
    resource_type,
    display_name,
    public_label,
    bookable,
    active
)
VALUES ($1, 'ashtonbee', 'gym-floor', 'court', $2, $3, $4, $5)
RETURNING public_option_id
`, resourceKey, "Internal "+resourceKey, publicLabel, bookable, active).Scan(&optionID); err != nil {
		t.Fatalf("insert public booking HTTP resource error = %v", err)
	}
	return optionID
}

func insertPrivateBookingHTTPResourceWithOption(t *testing.T, env *authProfileServerEnv, resourceKey string) uuid.UUID {
	t.Helper()

	var optionID uuid.UUID
	if err := env.db.DB.QueryRow(context.Background(), `
INSERT INTO apollo.schedule_resources (
    resource_key,
    facility_key,
    zone_key,
    resource_type,
    display_name
)
VALUES ($1, 'ashtonbee', 'gym-floor', 'court', $2)
RETURNING public_option_id
`, resourceKey, "Internal "+resourceKey).Scan(&optionID); err != nil {
		t.Fatalf("insert private booking HTTP resource error = %v", err)
	}
	return optionID
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
