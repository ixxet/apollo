package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/presence"
	"github.com/ixxet/apollo/internal/visits"
)

func TestPresenceClaimRuntimeSupportsSelfScopedMemberClaimsAndRejectsUnsafeCases(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	memberCookie, member := createVerifiedSessionViaHTTP(t, env, "student-presence-claim-001", "presence-claim-001@example.com")
	_, foreignMember := createVerifiedSessionViaHTTP(t, env, "student-presence-claim-002", "presence-claim-002@example.com")

	if response := env.doRequest(t, http.MethodGet, "/api/v1/presence/claims", nil); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated GET /api/v1/presence/claims = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/presence/claims", `{"tag_hash":"claim-unauth-001"}`); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated POST /api/v1/presence/claims = %d, want %d", response.Code, http.StatusUnauthorized)
	}

	initialClaimsResponse := env.doRequest(t, http.MethodGet, "/api/v1/presence/claims", nil, memberCookie)
	if initialClaimsResponse.Code != http.StatusOK {
		t.Fatalf("initialClaimsResponse.Code = %d, want %d", initialClaimsResponse.Code, http.StatusOK)
	}
	if claims := decodePresenceClaimsResponse(t, initialClaimsResponse); len(claims) != 0 {
		t.Fatalf("len(initial claims) = %d, want 0", len(claims))
	}

	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/presence/claims", `{"tag_hash":" "}`, memberCookie); response.Code != http.StatusBadRequest {
		t.Fatalf("blank claim response code = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/presence/claims", `{"tag_hash":"bad*claim"}`, memberCookie); response.Code != http.StatusBadRequest {
		t.Fatalf("malformed claim response code = %d, want %d", response.Code, http.StatusBadRequest)
	}

	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.claimed_tags (user_id, tag_hash, label, is_active) VALUES ($1, $2, $3, TRUE)", foreignMember.ID, "foreign-claim-001", "foreign claim"); err != nil {
		t.Fatalf("Exec(insert foreign claimed tag) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.claimed_tags (user_id, tag_hash, label, is_active) VALUES ($1, $2, $3, FALSE)", member.ID, "inactive-claim-001", "inactive claim"); err != nil {
		t.Fatalf("Exec(insert inactive claimed tag) error = %v", err)
	}

	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/presence/claims", `{"tag_hash":"foreign-claim-001"}`, memberCookie); response.Code != http.StatusConflict {
		t.Fatalf("foreign claim response code = %d, want %d", response.Code, http.StatusConflict)
	}
	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/presence/claims", `{"tag_hash":"inactive-claim-001"}`, memberCookie); response.Code != http.StatusConflict {
		t.Fatalf("inactive claim response code = %d, want %d", response.Code, http.StatusConflict)
	}

	createResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/presence/claims", `{"tag_hash":"member-claim-001","label":"North tag"}`, memberCookie)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("createResponse.Code = %d, want %d body=%s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}
	created := decodePresenceClaimResponse(t, createResponse)
	if created.TagHash != "member-claim-001" {
		t.Fatalf("created.TagHash = %q, want %q", created.TagHash, "member-claim-001")
	}
	if created.Status != presence.ClaimStatusActive {
		t.Fatalf("created.Status = %q, want %q", created.Status, presence.ClaimStatusActive)
	}
	if created.Label == nil || *created.Label != "North tag" {
		t.Fatalf("created.Label = %#v, want North tag", created.Label)
	}

	if response := env.doJSONRequest(t, http.MethodPost, "/api/v1/presence/claims", `{"tag_hash":"member-claim-001","label":"North tag"}`, memberCookie); response.Code != http.StatusConflict {
		t.Fatalf("replay claim response code = %d, want %d", response.Code, http.StatusConflict)
	}

	finalClaimsResponse := env.doRequest(t, http.MethodGet, "/api/v1/presence/claims", nil, memberCookie)
	if finalClaimsResponse.Code != http.StatusOK {
		t.Fatalf("finalClaimsResponse.Code = %d, want %d", finalClaimsResponse.Code, http.StatusOK)
	}
	finalClaims := decodePresenceClaimsResponse(t, finalClaimsResponse)
	if got, want := len(finalClaims), 2; got != want {
		t.Fatalf("len(finalClaims) = %d, want %d", got, want)
	}
	if finalClaims[0].TagHash != "member-claim-001" || finalClaims[0].Status != presence.ClaimStatusActive {
		t.Fatalf("finalClaims[0] = %#v, want new active claim first", finalClaims[0])
	}
	if finalClaims[1].TagHash != "inactive-claim-001" || finalClaims[1].Status != presence.ClaimStatusInactive {
		t.Fatalf("finalClaims[1] = %#v, want existing inactive claim second", finalClaims[1])
	}
}

func TestPresenceClaimRuntimeLinksClaimedArrivalIntoMemberPresenceSummary(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	memberCookie, _ := createVerifiedSessionViaHTTP(t, env, "student-presence-claim-link-001", "presence-claim-link-001@example.com")
	tagHash := "member-claim-link-001"
	base := time.Now().UTC().Truncate(time.Second)

	createResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/presence/claims", `{"tag_hash":"`+tagHash+`","label":"North turnstile"}`, memberCookie)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("createResponse.Code = %d, want %d body=%s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}

	presenceService := presence.NewService(presence.NewRepository(env.db.DB), visits.NewService(visits.NewRepository(env.db.DB)))
	if _, err := presenceService.RecordArrival(context.Background(), visits.ArrivalInput{
		SourceEventID:        "presence-claim-link-arrival-001",
		FacilityKey:          "ashtonbee",
		ZoneKey:              stringPtr("gym-floor"),
		ExternalIdentityHash: tagHash,
		ArrivedAt:            base,
	}); err != nil {
		t.Fatalf("RecordArrival(ashtonbee claimed tag) error = %v", err)
	}

	summaryResponse := env.doRequest(t, http.MethodGet, "/api/v1/presence", nil, memberCookie)
	if summaryResponse.Code != http.StatusOK {
		t.Fatalf("summaryResponse.Code = %d, want %d body=%s", summaryResponse.Code, http.StatusOK, summaryResponse.Body.String())
	}

	summary := decodePresenceSummaryResponse(t, summaryResponse)
	if got, want := len(summary.Facilities), 1; got != want {
		t.Fatalf("len(summary.Facilities) = %d, want %d", got, want)
	}

	facility := summary.Facilities[0]
	if facility.FacilityKey != "ashtonbee" {
		t.Fatalf("facility.FacilityKey = %q, want %q", facility.FacilityKey, "ashtonbee")
	}
	if facility.Status != presence.StatusPresent {
		t.Fatalf("facility.Status = %q, want %q", facility.Status, presence.StatusPresent)
	}
	if facility.Current == nil {
		t.Fatal("facility.Current = nil, want claimed linked visit")
	}
	if facility.Current.ZoneKey == nil || *facility.Current.ZoneKey != "gym-floor" {
		t.Fatalf("facility.Current.ZoneKey = %#v, want gym-floor", facility.Current.ZoneKey)
	}
	if facility.Current.TapLink.Status != presence.TapLinkStatusLinked {
		t.Fatalf("facility.Current.TapLink.Status = %q, want %q", facility.Current.TapLink.Status, presence.TapLinkStatusLinked)
	}
	if facility.Current.TapLink.LinkedAt.IsZero() {
		t.Fatal("facility.Current.TapLink.LinkedAt = zero, want linked timestamp")
	}
	if !facility.Current.ArrivedAt.Equal(base) {
		t.Fatalf("facility.Current.ArrivedAt = %s, want %s", facility.Current.ArrivedAt.UTC().Format(time.RFC3339), base.Format(time.RFC3339))
	}
	if got, want := len(facility.RecentVisits), 1; got != want {
		t.Fatalf("len(facility.RecentVisits) = %d, want %d", got, want)
	}
	if facility.RecentVisits[0].ID != facility.Current.ID {
		t.Fatalf("facility.RecentVisits[0].ID = %s, want current visit %s", facility.RecentVisits[0].ID, facility.Current.ID)
	}
	if facility.RecentVisits[0].TapLink.Status != presence.TapLinkStatusLinked {
		t.Fatalf("facility.RecentVisits[0].TapLink.Status = %q, want %q", facility.RecentVisits[0].TapLink.Status, presence.TapLinkStatusLinked)
	}
	if !facility.RecentVisits[0].ArrivedAt.Equal(base) {
		t.Fatalf("facility.RecentVisits[0].ArrivedAt = %s, want %s", facility.RecentVisits[0].ArrivedAt.UTC().Format(time.RFC3339), base.Format(time.RFC3339))
	}
}

func TestMemberFacilityRuntimeComposesFacilityHoursAndMetaWithoutScheduleLeakage(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-presence-facility-001", "presence-facility-001@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)
	memberCookie, member := createVerifiedSessionViaHTTP(t, env, "student-presence-facility-002", "presence-facility-002@example.com")

	if response := env.doRequest(t, http.MethodGet, "/api/v1/presence/facilities", nil); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated GET /api/v1/presence/facilities = %d, want %d", response.Code, http.StatusUnauthorized)
	}

	base := time.Now().UTC().Truncate(time.Second)
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.claimed_tags (user_id, tag_hash, label) VALUES ($1, $2, $3)", member.ID, "presence-facility-tag", "facility tag"); err != nil {
		t.Fatalf("Exec(insert claimed tag) error = %v", err)
	}
	presenceService := presence.NewService(presence.NewRepository(env.db.DB), visits.NewService(visits.NewRepository(env.db.DB)))
	if _, err := presenceService.RecordArrival(context.Background(), visits.ArrivalInput{
		SourceEventID:        "presence-facility-arrival-001",
		FacilityKey:          "annex",
		ExternalIdentityHash: "presence-facility-tag",
		ArrivedAt:            base.Add(-2 * time.Hour),
	}); err != nil {
		t.Fatalf("RecordArrival(annex) error = %v", err)
	}

	operatingHours := env.doJSONRequest(t, http.MethodPost, "/api/v1/schedule/blocks", fmt.Sprintf(`{
		"facility_key":"ashtonbee",
		"scope":"facility",
		"kind":"operating_hours",
		"effect":"informational",
		"visibility":"public_labeled",
		"one_off":{
			"starts_at":"%s",
			"ends_at":"%s"
		}
	}`, base.Add(time.Hour).Format(time.RFC3339), base.Add(2*time.Hour).Format(time.RFC3339)), ownerCookie)
	if operatingHours.Code != http.StatusCreated {
		t.Fatalf("operatingHours.Code = %d, want %d body=%s", operatingHours.Code, http.StatusCreated, operatingHours.Body.String())
	}

	publicClosure := env.doJSONRequest(t, http.MethodPost, "/api/v1/schedule/blocks", fmt.Sprintf(`{
		"facility_key":"ashtonbee",
		"scope":"facility",
		"kind":"closure",
		"effect":"closed",
		"visibility":"public_busy",
		"one_off":{
			"starts_at":"%s",
			"ends_at":"%s"
		}
	}`, base.Add(24*time.Hour).Format(time.RFC3339), base.Add(25*time.Hour).Format(time.RFC3339)), ownerCookie)
	if publicClosure.Code != http.StatusCreated {
		t.Fatalf("publicClosure.Code = %d, want %d body=%s", publicClosure.Code, http.StatusCreated, publicClosure.Body.String())
	}

	internalEvent := env.doJSONRequest(t, http.MethodPost, "/api/v1/schedule/blocks", fmt.Sprintf(`{
		"facility_key":"ashtonbee",
		"scope":"facility",
		"kind":"event",
		"effect":"informational",
		"visibility":"internal",
		"one_off":{
			"starts_at":"%s",
			"ends_at":"%s"
		}
	}`, base.Add(3*time.Hour).Format(time.RFC3339), base.Add(4*time.Hour).Format(time.RFC3339)), ownerCookie)
	if internalEvent.Code != http.StatusCreated {
		t.Fatalf("internalEvent.Code = %d, want %d body=%s", internalEvent.Code, http.StatusCreated, internalEvent.Body.String())
	}

	response := env.doRequest(t, http.MethodGet, "/api/v1/presence/facilities", nil, memberCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("response.Code = %d, want %d body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "created_by_user_id") || strings.Contains(response.Body.String(), "\"conflicts\"") {
		t.Fatalf("member facility response leaked schedule internals: %s", response.Body.String())
	}
	if strings.Contains(response.Body.String(), "\"visibility\":\"internal\"") {
		t.Fatalf("member facility response leaked internal schedule visibility: %s", response.Body.String())
	}

	facilities := decodeMemberFacilitiesResponse(t, response)
	ashtonbee := findMemberFacility(t, facilities, "ashtonbee")
	if got, want := len(ashtonbee.SupportedSports), 2; got != want {
		t.Fatalf("len(ashtonbee.SupportedSports) = %d, want %d", got, want)
	}
	if got, want := len(ashtonbee.Hours), 1; got != want {
		t.Fatalf("len(ashtonbee.Hours) = %d, want %d", got, want)
	}
	if got, want := len(ashtonbee.Closures), 1; got != want {
		t.Fatalf("len(ashtonbee.Closures) = %d, want %d", got, want)
	}

	annex := findMemberFacility(t, facilities, "annex")
	if len(annex.SupportedSports) != 0 {
		t.Fatalf("len(annex.SupportedSports) = %d, want 0", len(annex.SupportedSports))
	}
	if len(annex.Hours) != 0 || len(annex.Closures) != 0 {
		t.Fatalf("annex facility windows = %+v, want no derived schedule windows", annex)
	}
}

func decodePresenceClaimResponse(t *testing.T, response *httptest.ResponseRecorder) presence.Claim {
	t.Helper()

	var payload presence.Claim
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(presence claim) error = %v", err)
	}
	return payload
}

func decodePresenceClaimsResponse(t *testing.T, response *httptest.ResponseRecorder) []presence.Claim {
	t.Helper()

	var payload []presence.Claim
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(presence claims) error = %v", err)
	}
	return payload
}

func decodeMemberFacilitiesResponse(t *testing.T, response *httptest.ResponseRecorder) []presence.MemberFacility {
	t.Helper()

	var payload []presence.MemberFacility
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(member facilities) error = %v", err)
	}
	return payload
}

func findMemberFacility(t *testing.T, facilities []presence.MemberFacility, facilityKey string) presence.MemberFacility {
	t.Helper()

	for _, facility := range facilities {
		if facility.FacilityKey == facilityKey {
			return facility
		}
	}
	t.Fatalf("facility %q missing from %#v", facilityKey, facilities)
	return presence.MemberFacility{}
}
