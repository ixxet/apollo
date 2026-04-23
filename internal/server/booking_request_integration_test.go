package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
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

func TestBookingRequestEditAndRebookBoundariesHTTP(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	managerCookie, manager := createVerifiedSessionViaHTTP(t, env, "student-booking-edit-001", "booking-edit-001@example.com")
	supervisorCookie, supervisor := createVerifiedSessionViaHTTP(t, env, "student-booking-edit-002", "booking-edit-002@example.com")
	setUserRole(t, env, manager.ID, authz.RoleManager)
	setUserRole(t, env, supervisor.ID, authz.RoleSupervisor)

	originalResource := insertBookingHTTPResource(t, env, "booking-edit-original-court")
	editedResource := insertBookingHTTPResource(t, env, "booking-edit-updated-court")
	create := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests", bookingRequestJSONWithEmail(originalResource, "2026-04-19T18:00:00Z", "2026-04-19T19:00:00Z", "booking-edit@example.com"), managerCookie)
	if create.Code != http.StatusCreated {
		t.Fatalf("create.Code = %d, want %d body=%s", create.Code, http.StatusCreated, create.Body.String())
	}
	created := decodeBookingRequest(t, create)

	editBody := bookingRequestEditJSON(editedResource, "2026-04-19T20:00:00Z", "2026-04-19T21:00:00Z", "booking-edit-updated@example.com", 1)
	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+created.ID.String()+"/edit", editBody, supervisorCookie); response.Code != http.StatusForbidden {
		t.Fatalf("supervisor edit code = %d, want %d body=%s", response.Code, http.StatusForbidden, response.Body.String())
	}
	if response := env.doRequestWithoutTrustedSurface(t, http.MethodPost, "/api/v1/booking/requests/"+created.ID.String()+"/edit", bytes.NewBufferString(editBody), managerCookie); response.Code != http.StatusForbidden {
		t.Fatalf("missing trusted edit code = %d, want %d body=%s", response.Code, http.StatusForbidden, response.Body.String())
	}
	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+created.ID.String()+"/edit", strings.Replace(editBody, `"expected_version":1`, `"expected_version":9`, 1), managerCookie); response.Code != http.StatusConflict {
		t.Fatalf("stale edit code = %d, want %d body=%s", response.Code, http.StatusConflict, response.Body.String())
	}

	edit := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+created.ID.String()+"/edit", editBody, managerCookie)
	if edit.Code != http.StatusOK {
		t.Fatalf("edit.Code = %d, want %d body=%s", edit.Code, http.StatusOK, edit.Body.String())
	}
	edited := decodeBookingRequest(t, edit)
	if edited.ID != created.ID || edited.Version != 2 || edited.Status != booking.StatusRequested || edited.ScheduleBlockID != nil {
		t.Fatalf("edited identity/version/status/block = %s/%d/%s/%v", edited.ID, edited.Version, edited.Status, edited.ScheduleBlockID)
	}
	if edited.ResourceKey == nil || *edited.ResourceKey != editedResource || edited.ContactEmail == nil || *edited.ContactEmail != "booking-edit-updated@example.com" {
		t.Fatalf("edited resource/email = %v/%v, want %s/booking-edit-updated@example.com", edited.ResourceKey, edited.ContactEmail, editedResource)
	}

	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+created.ID.String()+"/approve", `{"expected_version":2}`, managerCookie); response.Code != http.StatusOK {
		t.Fatalf("approve edited code = %d, want %d body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	approvedResponse := env.doRequest(t, http.MethodGet, "/api/v1/booking/requests/"+created.ID.String(), nil, managerCookie)
	approved := decodeBookingRequest(t, approvedResponse)
	if approved.Status != booking.StatusApproved || approved.ScheduleBlockID == nil {
		t.Fatalf("approved status/block = %s/%v", approved.Status, approved.ScheduleBlockID)
	}
	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+approved.ID.String()+"/edit", bookingRequestEditJSON(editedResource, "2026-04-19T22:00:00Z", "2026-04-19T23:00:00Z", "booking-edit-approved@example.com", approved.Version), managerCookie); response.Code != http.StatusConflict {
		t.Fatalf("approved edit code = %d, want %d body=%s", response.Code, http.StatusConflict, response.Body.String())
	}

	rebookResource := insertBookingHTTPResource(t, env, "booking-rebook-replacement-court")
	beforeRebookBlocks := countBookingHTTPScheduleBlocks(t, env)
	rebookBody := bookingRequestEditJSON(rebookResource, "2026-04-20T20:00:00Z", "2026-04-20T21:00:00Z", "booking-rebook@example.com", approved.Version)
	rebookHeaders := staffBookingIdempotencyHeaders(env, "booking-rebook-key-001")
	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+created.ID.String()+"/rebook", rebookBody, managerCookie); response.Code != http.StatusBadRequest {
		t.Fatalf("rebook missing idempotency key code = %d, want %d body=%s", response.Code, http.StatusBadRequest, response.Body.String())
	}
	rebook := env.doRequestWithHeaders(t, http.MethodPost, "/api/v1/booking/requests/"+created.ID.String()+"/rebook", bytes.NewBufferString(rebookBody), rebookHeaders, managerCookie)
	if rebook.Code != http.StatusCreated {
		t.Fatalf("rebook.Code = %d, want %d body=%s", rebook.Code, http.StatusCreated, rebook.Body.String())
	}
	replacement := decodeBookingRequest(t, rebook)
	if replacement.ID == approved.ID || replacement.Status != booking.StatusRequested || replacement.ScheduleBlockID != nil {
		t.Fatalf("replacement identity/status/block = %s/%s/%v", replacement.ID, replacement.Status, replacement.ScheduleBlockID)
	}
	if replacement.ReplacesRequestID == nil || *replacement.ReplacesRequestID != approved.ID {
		t.Fatalf("replacement ReplacesRequestID = %v, want %s", replacement.ReplacesRequestID, approved.ID)
	}
	if afterRebookBlocks := countBookingHTTPScheduleBlocks(t, env); afterRebookBlocks != beforeRebookBlocks {
		t.Fatalf("schedule block count after rebook = %d, want unchanged %d", afterRebookBlocks, beforeRebookBlocks)
	}

	duplicate := env.doRequestWithHeaders(t, http.MethodPost, "/api/v1/booking/requests/"+created.ID.String()+"/rebook", bytes.NewBufferString(rebookBody), rebookHeaders, managerCookie)
	if duplicate.Code != http.StatusCreated {
		t.Fatalf("duplicate rebook code = %d, want %d body=%s", duplicate.Code, http.StatusCreated, duplicate.Body.String())
	}
	duplicateReplacement := decodeBookingRequest(t, duplicate)
	if duplicateReplacement.ID != replacement.ID {
		t.Fatalf("duplicate rebook ID = %s, want %s", duplicateReplacement.ID, replacement.ID)
	}
	changedVersion := strings.Replace(rebookBody, `"expected_version":`+strconv.Itoa(approved.Version), `"expected_version":`+strconv.Itoa(approved.Version+1), 1)
	if response := env.doRequestWithHeaders(t, http.MethodPost, "/api/v1/booking/requests/"+created.ID.String()+"/rebook", bytes.NewBufferString(changedVersion), rebookHeaders, managerCookie); response.Code != http.StatusConflict {
		t.Fatalf("changed rebook version idempotency code = %d, want %d body=%s", response.Code, http.StatusConflict, response.Body.String())
	}
	changed := strings.Replace(rebookBody, "booking-rebook@example.com", "booking-rebook-changed@example.com", 1)
	if response := env.doRequestWithHeaders(t, http.MethodPost, "/api/v1/booking/requests/"+created.ID.String()+"/rebook", bytes.NewBufferString(changed), rebookHeaders, managerCookie); response.Code != http.StatusConflict {
		t.Fatalf("changed rebook idempotency code = %d, want %d body=%s", response.Code, http.StatusConflict, response.Body.String())
	}
	if count := countBookingHTTPRequestsByEmail(t, env, "booking-rebook@example.com"); count != 1 {
		t.Fatalf("replacement request count = %d, want 1", count)
	}
}

func TestBookingRequestEditUpdatesPublicStatusWindowHTTP(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	optionID := insertPublicBookingHTTPResource(t, env, "booking-public-edit-court", "Editable public court", true, true)
	start, end := publicBookingWindow(96*time.Hour, time.Hour)
	submit := env.doRequestWithHeaders(t, http.MethodPost, "/api/v1/public/booking/requests", bytes.NewBufferString(publicBookingRequestJSON(optionID, start, end, `"contact_email":"public-edit@example.com"`)), map[string]string{
		"Idempotency-Key":                "public-edit-key-001",
		"X-Apollo-Public-Intake-Channel": booking.IntakeChannelPublicWeb,
	})
	if submit.Code != http.StatusAccepted {
		t.Fatalf("public edit submit code = %d, want %d body=%s", submit.Code, http.StatusAccepted, submit.Body.String())
	}
	receipt := decodePublicReceipt(t, submit)
	requestID := bookingHTTPRequestIDByEmail(t, env, "public-edit@example.com")

	managerCookie, manager := createVerifiedSessionViaHTTP(t, env, "student-public-edit-manager", "public-edit-manager@example.com")
	setUserRole(t, env, manager.ID, authz.RoleManager)
	updatedStart, updatedEnd := publicBookingWindow(120*time.Hour, 90*time.Minute)
	edit := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+requestID.String()+"/edit", bookingRequestEditJSON("booking-public-edit-court", updatedStart, updatedEnd, "public-edit@example.com", 1), managerCookie)
	if edit.Code != http.StatusOK {
		t.Fatalf("public edit code = %d, want %d body=%s", edit.Code, http.StatusOK, edit.Body.String())
	}

	statusResponse := env.doRequest(t, http.MethodGet, "/api/v1/public/booking/requests/status?receipt_code="+url.QueryEscape(receipt.ReceiptCode), nil)
	if statusResponse.Code != http.StatusOK {
		t.Fatalf("public status after edit code = %d, want %d body=%s", statusResponse.Code, http.StatusOK, statusResponse.Body.String())
	}
	publicStatus := decodePublicStatus(t, statusResponse)
	if publicStatus.RequestedStartAt.Format(time.RFC3339) != updatedStart || publicStatus.RequestedEndAt.Format(time.RFC3339) != updatedEnd {
		t.Fatalf("public status window = %s/%s, want %s/%s", publicStatus.RequestedStartAt.Format(time.RFC3339), publicStatus.RequestedEndAt.Format(time.RFC3339), updatedStart, updatedEnd)
	}
	assertPublicBookingResponseSafe(t, statusResponse)
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

func TestPublicBookingAvailabilityReadIsSanitizedAndReadOnly(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	managerCookie, manager := createVerifiedSessionViaHTTP(t, env, "student-booking-availability-manager", "booking-availability-manager@example.com")
	setUserRole(t, env, manager.ID, authz.RoleManager)
	optionID := insertPublicBookingHTTPResource(t, env, "booking-public-availability-court", "Availability Court", true, true)

	operating := createAvailabilityScheduleBlock(t, env, managerCookie, publicAvailabilityWeeklyBlockJSON("booking-public-availability-court", schedule.KindOperatingHours, schedule.EffectInformational, schedule.VisibilityPublicLabeled, 1, "09:00", "17:00", "America/Toronto", "2026-05-04", "2026-05-11"))
	closure := createAvailabilityScheduleBlock(t, env, managerCookie, publicAvailabilityOneOffBlockJSON("booking-public-availability-court", schedule.KindClosure, schedule.EffectClosed, schedule.VisibilityInternal, "2026-05-04T16:00:00Z", "2026-05-04T17:00:00Z"))
	reservation := createAvailabilityScheduleBlock(t, env, managerCookie, publicAvailabilityOneOffBlockJSON("booking-public-availability-court", schedule.KindReservation, schedule.EffectHardReserve, schedule.VisibilityInternal, "2026-05-04T18:00:00Z", "2026-05-04T19:00:00Z"))
	publicBusy := createAvailabilityScheduleBlock(t, env, managerCookie, publicAvailabilityOneOffBlockJSON("booking-public-availability-court", schedule.KindEvent, schedule.EffectInformational, schedule.VisibilityPublicBusy, "2026-05-04T19:30:00Z", "2026-05-04T20:00:00Z"))

	beforeBlocks := countBookingHTTPScheduleBlocks(t, env)
	beforeRequests := countBookingHTTPRequests(t, env)
	response := env.doRequest(t, http.MethodGet, publicAvailabilityURL(optionID, "2026-05-04T13:00:00Z", "2026-05-04T21:00:00Z"), nil)
	if response.Code != http.StatusOK {
		t.Fatalf("public availability code = %d, want %d body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	if afterBlocks := countBookingHTTPScheduleBlocks(t, env); afterBlocks != beforeBlocks {
		t.Fatalf("availability read schedule block count = %d, want unchanged %d", afterBlocks, beforeBlocks)
	}
	if afterRequests := countBookingHTTPRequests(t, env); afterRequests != beforeRequests {
		t.Fatalf("availability read booking request count = %d, want unchanged %d", afterRequests, beforeRequests)
	}

	availability := decodePublicAvailability(t, response)
	if availability.OptionID != optionID {
		t.Fatalf("availability.OptionID = %s, want %s", availability.OptionID, optionID)
	}
	if availability.From.Format(time.RFC3339) != "2026-05-04T13:00:00Z" || availability.Until.Format(time.RFC3339) != "2026-05-04T21:00:00Z" {
		t.Fatalf("availability window = %s/%s", availability.From.Format(time.RFC3339), availability.Until.Format(time.RFC3339))
	}
	if availability.TimeZone == nil || *availability.TimeZone != "America/Toronto" {
		t.Fatalf("availability.TimeZone = %v, want America/Toronto", availability.TimeZone)
	}
	assertAvailabilityWindows(t, availability.RequestableWindows, []availabilityWindowExpectation{
		{startAt: "2026-05-04T13:00:00Z", endAt: "2026-05-04T16:00:00Z"},
		{startAt: "2026-05-04T17:00:00Z", endAt: "2026-05-04T18:00:00Z"},
		{startAt: "2026-05-04T19:00:00Z", endAt: "2026-05-04T19:30:00Z"},
		{startAt: "2026-05-04T20:00:00Z", endAt: "2026-05-04T21:00:00Z"},
	})
	assertUnavailableBlocks(t, availability.UnavailableBlocks, []unavailableBlockExpectation{
		{startAt: "2026-05-04T16:00:00Z", endAt: "2026-05-04T17:00:00Z", reason: "closed"},
		{startAt: "2026-05-04T18:00:00Z", endAt: "2026-05-04T19:00:00Z", reason: "booked"},
		{startAt: "2026-05-04T19:30:00Z", endAt: "2026-05-04T20:00:00Z", reason: "unavailable"},
	})

	assertPublicBookingResponseSafe(t, response)
	for _, forbidden := range []string{
		operating.ID.String(),
		closure.ID.String(),
		reservation.ID.String(),
		publicBusy.ID.String(),
		"booking-public-availability-court",
		"Availability Court",
		"ashtonbee",
		"gym-floor",
		"reservation",
		"public_busy",
		"internal",
		"label",
	} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("public availability leaked %q in body %s", forbidden, response.Body.String())
		}
	}
}

func TestPublicBookingAvailabilityValidationAndOptionFiltering(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	optionID := insertPublicBookingHTTPResource(t, env, "booking-public-availability-validation-court", "Availability Validation Court", true, true)
	inactiveOptionID := insertPublicBookingHTTPResource(t, env, "booking-public-availability-validation-inactive", "Inactive Availability Court", true, false)
	unbookableOptionID := insertPublicBookingHTTPResource(t, env, "booking-public-availability-validation-unbookable", "Unbookable Availability Court", false, true)
	unlabeledOptionID := insertPrivateBookingHTTPResourceWithOption(t, env, "booking-public-availability-validation-unlabeled")
	from := "2026-05-05T13:00:00Z"
	until := "2026-05-05T15:00:00Z"

	valid := env.doRequest(t, http.MethodGet, publicAvailabilityURL(optionID, from, until), nil)
	if valid.Code != http.StatusOK {
		t.Fatalf("valid availability code = %d, want %d body=%s", valid.Code, http.StatusOK, valid.Body.String())
	}
	availability := decodePublicAvailability(t, valid)
	if availability.RequestableWindows == nil || availability.UnavailableBlocks == nil {
		t.Fatalf("availability slices should be empty arrays, got requestable=%#v unavailable=%#v", availability.RequestableWindows, availability.UnavailableBlocks)
	}

	testCases := []struct {
		name string
		path string
		code int
	}{
		{name: "missing from", path: "/api/v1/public/booking/options/" + optionID.String() + "/availability?until=" + url.QueryEscape(until), code: http.StatusBadRequest},
		{name: "missing until", path: "/api/v1/public/booking/options/" + optionID.String() + "/availability?from=" + url.QueryEscape(from), code: http.StatusBadRequest},
		{name: "date only", path: publicAvailabilityURL(optionID, "2026-05-05", until), code: http.StatusBadRequest},
		{name: "equal window", path: publicAvailabilityURL(optionID, from, from), code: http.StatusBadRequest},
		{name: "reversed window", path: publicAvailabilityURL(optionID, until, from), code: http.StatusBadRequest},
		{name: "oversized window", path: publicAvailabilityURL(optionID, from, "2026-05-19T13:00:01Z"), code: http.StatusBadRequest},
		{name: "inactive option", path: publicAvailabilityURL(inactiveOptionID, from, until), code: http.StatusNotFound},
		{name: "unbookable option", path: publicAvailabilityURL(unbookableOptionID, from, until), code: http.StatusNotFound},
		{name: "unlabeled option", path: publicAvailabilityURL(unlabeledOptionID, from, until), code: http.StatusNotFound},
		{name: "unknown option", path: publicAvailabilityURL(uuid.New(), from, until), code: http.StatusNotFound},
		{name: "invalid option id", path: "/api/v1/public/booking/options/not-a-uuid/availability?from=" + url.QueryEscape(from) + "&until=" + url.QueryEscape(until), code: http.StatusBadRequest},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			response := env.doRequest(t, http.MethodGet, testCase.path, nil)
			if response.Code != testCase.code {
				t.Fatalf("code = %d, want %d body=%s", response.Code, testCase.code, response.Body.String())
			}
			assertPublicBookingResponseSafe(t, response)
		})
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
	receipt := decodePublicReceipt(t, response)
	if receipt.Status != "received" {
		t.Fatalf("receipt.Status = %q, want received", receipt.Status)
	}
	if receipt.ReceiptCode == "" {
		t.Fatal("receipt.ReceiptCode is empty")
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
	duplicateReceipt := decodePublicReceipt(t, duplicate)
	if duplicateReceipt.ReceiptCode != receipt.ReceiptCode || duplicateReceipt.Status != receipt.Status {
		t.Fatalf("duplicate receipt = %#v, want same as %#v", duplicateReceipt, receipt)
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

	statusResponse := env.doRequest(t, http.MethodGet, "/api/v1/public/booking/requests/status?receipt_code="+url.QueryEscape(receipt.ReceiptCode), nil)
	if statusResponse.Code != http.StatusOK {
		t.Fatalf("public status code = %d, want %d body=%s", statusResponse.Code, http.StatusOK, statusResponse.Body.String())
	}
	publicStatus := decodePublicStatus(t, statusResponse)
	if publicStatus.ReceiptCode != receipt.ReceiptCode || publicStatus.Status != "received" {
		t.Fatalf("public status = %#v, want receipt %s received", publicStatus, receipt.ReceiptCode)
	}
	if publicStatus.RequestedStartAt.IsZero() || publicStatus.RequestedEndAt.IsZero() || publicStatus.UpdatedAt.IsZero() {
		t.Fatalf("public status timestamps not populated: %#v", publicStatus)
	}
	assertPublicBookingResponseSafe(t, statusResponse)

	unknown := env.doRequest(t, http.MethodGet, "/api/v1/public/booking/requests/status?receipt_code=BR-UNKNOWN", nil)
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown public status code = %d, want %d body=%s", unknown.Code, http.StatusNotFound, unknown.Body.String())
	}
	assertPublicBookingResponseSafe(t, unknown)

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

func TestPublicBookingStatusMapsDecisionsAndPublicMessageBoundaries(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	optionID := insertPublicBookingHTTPResource(t, env, "booking-public-status-court", "Status Court", true, true)
	start, end := publicBookingWindow(80*time.Hour, time.Hour)
	body := publicBookingRequestJSON(optionID, start, end, `"contact_email":"public-status@example.com"`)
	headers := map[string]string{
		"Idempotency-Key":                "public-status-key-001",
		"X-Apollo-Public-Intake-Channel": booking.IntakeChannelPublicWeb,
	}

	submit := env.doRequestWithHeaders(t, http.MethodPost, "/api/v1/public/booking/requests", bytes.NewBufferString(body), headers)
	if submit.Code != http.StatusAccepted {
		t.Fatalf("public status submit code = %d, want %d body=%s", submit.Code, http.StatusAccepted, submit.Body.String())
	}
	receipt := decodePublicReceipt(t, submit)
	requestID := bookingHTTPRequestIDByEmail(t, env, "public-status@example.com")

	managerCookie, manager := createVerifiedSessionViaHTTP(t, env, "student-public-status-manager", "public-status-manager@example.com")
	supervisorCookie, supervisor := createVerifiedSessionViaHTTP(t, env, "student-public-status-supervisor", "public-status-supervisor@example.com")
	setUserRole(t, env, manager.ID, authz.RoleManager)
	setUserRole(t, env, supervisor.ID, authz.RoleSupervisor)

	supervisorUpdate := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+requestID.String()+"/public-message", `{"expected_version":1,"public_message":"We are reviewing your request."}`, supervisorCookie)
	if supervisorUpdate.Code != http.StatusForbidden {
		t.Fatalf("supervisor public message code = %d, want %d body=%s", supervisorUpdate.Code, http.StatusForbidden, supervisorUpdate.Body.String())
	}
	missingSurface := env.doRequestWithoutTrustedSurface(t, http.MethodPost, "/api/v1/booking/requests/"+requestID.String()+"/public-message", bytes.NewBufferString(`{"expected_version":1,"public_message":"We are reviewing your request."}`), managerCookie)
	if missingSurface.Code != http.StatusForbidden {
		t.Fatalf("missing trusted public message code = %d, want %d body=%s", missingSurface.Code, http.StatusForbidden, missingSurface.Body.String())
	}

	messageBody := `{"expected_version":1,"public_message":"Please send the updated attendee count when you can."}`
	messageUpdate := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+requestID.String()+"/public-message", messageBody, managerCookie)
	if messageUpdate.Code != http.StatusOK {
		t.Fatalf("manager public message code = %d, want %d body=%s", messageUpdate.Code, http.StatusOK, messageUpdate.Body.String())
	}
	updated := decodeBookingRequest(t, messageUpdate)
	if updated.PublicMessage == nil || *updated.PublicMessage != "Please send the updated attendee count when you can." {
		t.Fatalf("updated public message = %v", updated.PublicMessage)
	}
	if updated.InternalNotes != nil {
		t.Fatalf("public message update changed internal notes to %q", *updated.InternalNotes)
	}
	if updated.Version != 2 {
		t.Fatalf("public message update version = %d, want 2", updated.Version)
	}

	review := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+requestID.String()+"/review", `{"expected_version":2,"internal_notes":"secret internal conflict note"}`, managerCookie)
	if review.Code != http.StatusOK {
		t.Fatalf("review code = %d, want %d body=%s", review.Code, http.StatusOK, review.Body.String())
	}
	assertPublicStatus(t, env, receipt.ReceiptCode, "under_review", "Please send the updated attendee count when you can.", "secret internal conflict note")

	needsChanges := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+requestID.String()+"/needs-changes", `{"expected_version":3}`, managerCookie)
	if needsChanges.Code != http.StatusOK {
		t.Fatalf("needs changes code = %d, want %d body=%s", needsChanges.Code, http.StatusOK, needsChanges.Body.String())
	}
	assertPublicStatus(t, env, receipt.ReceiptCode, "more_information_needed", "Please send the updated attendee count when you can.", "")

	reReview := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+requestID.String()+"/review", `{"expected_version":4}`, managerCookie)
	if reReview.Code != http.StatusOK {
		t.Fatalf("re-review code = %d, want %d body=%s", reReview.Code, http.StatusOK, reReview.Body.String())
	}
	approve := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+requestID.String()+"/approve", `{"expected_version":5}`, managerCookie)
	if approve.Code != http.StatusOK {
		t.Fatalf("approve code = %d, want %d body=%s", approve.Code, http.StatusOK, approve.Body.String())
	}
	assertPublicStatus(t, env, receipt.ReceiptCode, "approved", "Please send the updated attendee count when you can.", "")

	cancel := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+requestID.String()+"/cancel", `{"expected_version":6}`, managerCookie)
	if cancel.Code != http.StatusOK {
		t.Fatalf("cancel code = %d, want %d body=%s", cancel.Code, http.StatusOK, cancel.Body.String())
	}
	assertPublicStatus(t, env, receipt.ReceiptCode, "cancelled", "Please send the updated attendee count when you can.", "")

	rejectOptionID := insertPublicBookingHTTPResource(t, env, "booking-public-status-reject-court", "Reject Court", true, true)
	rejectStart, rejectEnd := publicBookingWindow(90*time.Hour, time.Hour)
	rejectBody := publicBookingRequestJSON(rejectOptionID, rejectStart, rejectEnd, `"contact_email":"public-status-reject@example.com"`)
	rejectSubmit := env.doRequestWithHeaders(t, http.MethodPost, "/api/v1/public/booking/requests", bytes.NewBufferString(rejectBody), map[string]string{
		"Idempotency-Key":                "public-status-reject-key-001",
		"X-Apollo-Public-Intake-Channel": booking.IntakeChannelPublicWeb,
	})
	if rejectSubmit.Code != http.StatusAccepted {
		t.Fatalf("reject public submit code = %d, want %d body=%s", rejectSubmit.Code, http.StatusAccepted, rejectSubmit.Body.String())
	}
	rejectReceipt := decodePublicReceipt(t, rejectSubmit)
	rejectRequestID := bookingHTTPRequestIDByEmail(t, env, "public-status-reject@example.com")
	reject := env.doJSONRequest(t, http.MethodPost, "/api/v1/booking/requests/"+rejectRequestID.String()+"/reject", `{"expected_version":1}`, managerCookie)
	if reject.Code != http.StatusOK {
		t.Fatalf("reject code = %d, want %d body=%s", reject.Code, http.StatusOK, reject.Body.String())
	}
	assertPublicStatus(t, env, rejectReceipt.ReceiptCode, "declined", "", "")
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

	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/schedule/blocks/"+managerApproved.ScheduleBlockID.String()+"/cancel", `{"expected_version":1}`, managerCookie); response.Code != http.StatusConflict {
		t.Fatalf("schedule linked reservation cancel code = %d, want %d body=%s", response.Code, http.StatusConflict, response.Body.String())
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

func decodePublicReceipt(t *testing.T, response *httptest.ResponseRecorder) booking.PublicReceipt {
	t.Helper()

	var receipt booking.PublicReceipt
	if err := json.Unmarshal(response.Body.Bytes(), &receipt); err != nil {
		t.Fatalf("json.Unmarshal(public receipt) error = %v body=%s", err, response.Body.String())
	}
	return receipt
}

func decodePublicStatus(t *testing.T, response *httptest.ResponseRecorder) booking.PublicStatus {
	t.Helper()

	var status booking.PublicStatus
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatalf("json.Unmarshal(public status) error = %v body=%s", err, response.Body.String())
	}
	return status
}

func decodePublicAvailability(t *testing.T, response *httptest.ResponseRecorder) booking.PublicAvailability {
	t.Helper()

	var availability booking.PublicAvailability
	if err := json.Unmarshal(response.Body.Bytes(), &availability); err != nil {
		t.Fatalf("json.Unmarshal(public availability) error = %v body=%s", err, response.Body.String())
	}
	return availability
}

func createAvailabilityScheduleBlock(t *testing.T, env *authProfileServerEnv, cookie *http.Cookie, body string) schedule.Block {
	t.Helper()

	response := env.doJSONRequest(t, http.MethodPost, "/api/v1/schedule/blocks", body, cookie)
	if response.Code != http.StatusCreated {
		t.Fatalf("create availability schedule block code = %d, want %d body=%s", response.Code, http.StatusCreated, response.Body.String())
	}
	var block schedule.Block
	if err := json.Unmarshal(response.Body.Bytes(), &block); err != nil {
		t.Fatalf("json.Unmarshal(schedule block) error = %v body=%s", err, response.Body.String())
	}
	return block
}

func publicAvailabilityURL(optionID uuid.UUID, from string, until string) string {
	return "/api/v1/public/booking/options/" + optionID.String() + "/availability?from=" + url.QueryEscape(from) + "&until=" + url.QueryEscape(until)
}

func publicAvailabilityOneOffBlockJSON(resourceKey string, kind string, effect string, visibility string, startsAt string, endsAt string) string {
	return `{
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"resource_key":"` + resourceKey + `",
		"scope":"resource",
		"kind":"` + kind + `",
		"effect":"` + effect + `",
		"visibility":"` + visibility + `",
		"one_off":{
			"starts_at":"` + startsAt + `",
			"ends_at":"` + endsAt + `"
		}
	}`
}

func publicAvailabilityWeeklyBlockJSON(resourceKey string, kind string, effect string, visibility string, weekday int, startTime string, endTime string, timezone string, recurrenceStartDate string, recurrenceEndDate string) string {
	return `{
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"resource_key":"` + resourceKey + `",
		"scope":"resource",
		"kind":"` + kind + `",
		"effect":"` + effect + `",
		"visibility":"` + visibility + `",
		"weekly":{
			"weekday":` + strconv.Itoa(weekday) + `,
			"start_time":"` + startTime + `",
			"end_time":"` + endTime + `",
			"timezone":"` + timezone + `",
			"recurrence_start_date":"` + recurrenceStartDate + `",
			"recurrence_end_date":"` + recurrenceEndDate + `"
		}
	}`
}

type availabilityWindowExpectation struct {
	startAt string
	endAt   string
}

type unavailableBlockExpectation struct {
	startAt string
	endAt   string
	reason  string
}

func assertAvailabilityWindows(t *testing.T, got []booking.PublicAvailabilityWindow, want []availabilityWindowExpectation) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("requestable window count = %d, want %d: %#v", len(got), len(want), got)
	}
	for index, expected := range want {
		if got[index].StartAt.Format(time.RFC3339) != expected.startAt || got[index].EndAt.Format(time.RFC3339) != expected.endAt {
			t.Fatalf("requestable window %d = %s/%s, want %s/%s", index, got[index].StartAt.Format(time.RFC3339), got[index].EndAt.Format(time.RFC3339), expected.startAt, expected.endAt)
		}
	}
}

func assertUnavailableBlocks(t *testing.T, got []booking.PublicUnavailableBlock, want []unavailableBlockExpectation) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("unavailable block count = %d, want %d: %#v", len(got), len(want), got)
	}
	for index, expected := range want {
		if got[index].StartAt.Format(time.RFC3339) != expected.startAt || got[index].EndAt.Format(time.RFC3339) != expected.endAt || got[index].Reason != expected.reason {
			t.Fatalf("unavailable block %d = %s/%s/%s, want %s/%s/%s", index, got[index].StartAt.Format(time.RFC3339), got[index].EndAt.Format(time.RFC3339), got[index].Reason, expected.startAt, expected.endAt, expected.reason)
		}
		if got[index].Label != nil {
			t.Fatalf("unavailable block %d label = %q, want omitted", index, *got[index].Label)
		}
	}
}

func assertPublicStatus(t *testing.T, env *authProfileServerEnv, receiptCode string, wantStatus string, wantMessage string, forbiddenText string) {
	t.Helper()

	response := env.doRequest(t, http.MethodGet, "/api/v1/public/booking/requests/status?receipt_code="+url.QueryEscape(receiptCode), nil)
	if response.Code != http.StatusOK {
		t.Fatalf("public status code = %d, want %d body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	status := decodePublicStatus(t, response)
	if status.ReceiptCode != receiptCode || status.Status != wantStatus {
		t.Fatalf("public status = %#v, want receipt %s status %s", status, receiptCode, wantStatus)
	}
	if wantMessage == "" {
		if status.Message != nil {
			t.Fatalf("public status message = %q, want nil", *status.Message)
		}
	} else if status.Message == nil || *status.Message != wantMessage {
		t.Fatalf("public status message = %v, want %q", status.Message, wantMessage)
	}
	if forbiddenText != "" && strings.Contains(response.Body.String(), forbiddenText) {
		t.Fatalf("public status leaked forbidden text %q in body %s", forbiddenText, response.Body.String())
	}
	assertPublicBookingResponseSafe(t, response)
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

func bookingRequestEditJSON(resourceKey string, startsAt string, endsAt string, email string, expectedVersion int) string {
	return `{
		"expected_version":` + strconv.Itoa(expectedVersion) + `,
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"resource_key":"` + resourceKey + `",
		"requested_start_at":"` + startsAt + `",
		"requested_end_at":"` + endsAt + `",
		"contact_name":"Casey Booker",
		"contact_email":"` + email + `",
		"organization":"Ashton Staff Intake",
		"purpose":"Court request",
		"attendee_count":8,
		"internal_notes":"updated by staff"
	}`
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

func countBookingHTTPRequests(t *testing.T, env *authProfileServerEnv) int {
	t.Helper()

	var count int
	if err := env.db.DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM apollo.booking_requests`).Scan(&count); err != nil {
		t.Fatalf("QueryRow(booking request count) error = %v", err)
	}
	return count
}

func bookingHTTPRequestIDByEmail(t *testing.T, env *authProfileServerEnv, email string) uuid.UUID {
	t.Helper()

	var id uuid.UUID
	if err := env.db.DB.QueryRow(context.Background(), `SELECT id FROM apollo.booking_requests WHERE contact_email = $1`, email).Scan(&id); err != nil {
		t.Fatalf("QueryRow(booking request id) error = %v", err)
	}
	return id
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
