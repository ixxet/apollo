package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/ixxet/apollo/internal/presence"
	"github.com/ixxet/apollo/internal/visits"
)

func TestPresenceRuntimeExposesFacilityScopedSummaryAndStaysDeterministic(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	cookie, user := createVerifiedSessionViaHTTP(t, env, "student-presence-rt-001", "presence-rt-001@example.com")
	base := time.Now().UTC().Truncate(time.Second)
	presenceService := presence.NewService(presence.NewRepository(env.db.DB), visits.NewService(visits.NewRepository(env.db.DB)))

	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.claimed_tags (user_id, tag_hash, label) VALUES ($1, $2, $3)", user.ID, "presence-tag-ashtonbee", "ashtonbee tag"); err != nil {
		t.Fatalf("Exec(insert ashtonbee tag) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.claimed_tags (user_id, tag_hash, label) VALUES ($1, $2, $3)", user.ID, "presence-tag-annex", "annex tag"); err != nil {
		t.Fatalf("Exec(insert annex tag) error = %v", err)
	}

	if _, err := presenceService.RecordArrival(context.Background(), visits.ArrivalInput{
		SourceEventID:        "presence-ashtonbee-arrival-001",
		FacilityKey:          "ashtonbee",
		ZoneKey:              stringPtr("gym-floor"),
		ExternalIdentityHash: "presence-tag-ashtonbee",
		ArrivedAt:            base.AddDate(0, 0, -1).Add(-3 * time.Hour),
	}); err != nil {
		t.Fatalf("RecordArrival(ashtonbee day1) error = %v", err)
	}
	if _, err := presenceService.RecordDeparture(context.Background(), visits.DepartureInput{
		SourceEventID:        "presence-ashtonbee-departure-001",
		FacilityKey:          "ashtonbee",
		ExternalIdentityHash: "presence-tag-ashtonbee",
		DepartedAt:           base.AddDate(0, 0, -1).Add(-2 * time.Hour),
	}); err != nil {
		t.Fatalf("RecordDeparture(ashtonbee day1) error = %v", err)
	}
	if _, err := presenceService.RecordArrival(context.Background(), visits.ArrivalInput{
		SourceEventID:        "presence-ashtonbee-arrival-002",
		FacilityKey:          "ashtonbee",
		ZoneKey:              stringPtr("gym-floor"),
		ExternalIdentityHash: "presence-tag-ashtonbee",
		ArrivedAt:            base.Add(-2 * time.Hour),
	}); err != nil {
		t.Fatalf("RecordArrival(ashtonbee day2) error = %v", err)
	}
	if _, err := presenceService.RecordArrival(context.Background(), visits.ArrivalInput{
		SourceEventID:        "presence-annex-arrival-001",
		FacilityKey:          "annex",
		ExternalIdentityHash: "presence-tag-annex",
		ArrivedAt:            base.AddDate(0, 0, -3).Add(-90 * time.Minute),
	}); err != nil {
		t.Fatalf("RecordArrival(annex day1) error = %v", err)
	}
	if _, err := presenceService.RecordDeparture(context.Background(), visits.DepartureInput{
		SourceEventID:        "presence-annex-departure-001",
		FacilityKey:          "annex",
		ExternalIdentityHash: "presence-tag-annex",
		DepartedAt:           base.AddDate(0, 0, -3).Add(-30 * time.Minute),
	}); err != nil {
		t.Fatalf("RecordDeparture(annex day1) error = %v", err)
	}

	firstResponse := env.doRequest(t, http.MethodGet, "/api/v1/presence", nil, cookie)
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("firstResponse.Code = %d, want %d", firstResponse.Code, http.StatusOK)
	}
	firstSummary := decodePresenceSummaryResponse(t, firstResponse)

	secondResponse := env.doRequest(t, http.MethodGet, "/api/v1/presence", nil, cookie)
	if secondResponse.Code != http.StatusOK {
		t.Fatalf("secondResponse.Code = %d, want %d", secondResponse.Code, http.StatusOK)
	}
	secondSummary := decodePresenceSummaryResponse(t, secondResponse)

	if !reflect.DeepEqual(firstSummary, secondSummary) {
		t.Fatalf("presence summary changed across rerun:\nfirst=%#v\nsecond=%#v", firstSummary, secondSummary)
	}
	if len(firstSummary.Facilities) != 2 {
		t.Fatalf("len(firstSummary.Facilities) = %d, want 2", len(firstSummary.Facilities))
	}

	annex := firstSummary.Facilities[0]
	if annex.FacilityKey != "annex" {
		t.Fatalf("annex.FacilityKey = %q, want annex", annex.FacilityKey)
	}
	if annex.Status != presence.StatusNotPresent {
		t.Fatalf("annex.Status = %q, want %q", annex.Status, presence.StatusNotPresent)
	}
	if annex.Current != nil {
		t.Fatalf("annex.Current = %#v, want nil", annex.Current)
	}
	if len(annex.RecentVisits) != 1 {
		t.Fatalf("len(annex.RecentVisits) = %d, want 1", len(annex.RecentVisits))
	}
	if annex.RecentVisits[0].TapLink.Status != presence.TapLinkStatusLinked {
		t.Fatalf("annex tap-link status = %q, want %q", annex.RecentVisits[0].TapLink.Status, presence.TapLinkStatusLinked)
	}
	if annex.Streak.Status != presence.StreakStatusInactive {
		t.Fatalf("annex streak status = %q, want %q", annex.Streak.Status, presence.StreakStatusInactive)
	}
	if annex.Streak.CurrentCount != 1 {
		t.Fatalf("annex streak current_count = %d, want 1", annex.Streak.CurrentCount)
	}
	if annex.Streak.LatestEvent == nil || annex.Streak.LatestEvent.Kind != "started" {
		t.Fatalf("annex latest event = %#v, want started", annex.Streak.LatestEvent)
	}

	ashtonbee := firstSummary.Facilities[1]
	if ashtonbee.FacilityKey != "ashtonbee" {
		t.Fatalf("ashtonbee.FacilityKey = %q, want ashtonbee", ashtonbee.FacilityKey)
	}
	if ashtonbee.Status != presence.StatusPresent {
		t.Fatalf("ashtonbee.Status = %q, want %q", ashtonbee.Status, presence.StatusPresent)
	}
	if ashtonbee.Current == nil {
		t.Fatal("ashtonbee.Current = nil, want open linked visit")
	}
	if ashtonbee.Current.ZoneKey == nil || *ashtonbee.Current.ZoneKey != "gym-floor" {
		t.Fatalf("ashtonbee.Current.ZoneKey = %#v, want gym-floor", ashtonbee.Current.ZoneKey)
	}
	if ashtonbee.Current.TapLink.Status != presence.TapLinkStatusLinked {
		t.Fatalf("ashtonbee current tap-link status = %q, want %q", ashtonbee.Current.TapLink.Status, presence.TapLinkStatusLinked)
	}
	if len(ashtonbee.RecentVisits) != 2 {
		t.Fatalf("len(ashtonbee.RecentVisits) = %d, want 2", len(ashtonbee.RecentVisits))
	}
	if ashtonbee.Streak.Status != presence.StreakStatusActive {
		t.Fatalf("ashtonbee streak status = %q, want %q", ashtonbee.Streak.Status, presence.StreakStatusActive)
	}
	if ashtonbee.Streak.CurrentCount != 2 {
		t.Fatalf("ashtonbee streak current_count = %d, want 2", ashtonbee.Streak.CurrentCount)
	}
	if ashtonbee.Streak.LatestEvent == nil || ashtonbee.Streak.LatestEvent.Kind != "continued" {
		t.Fatalf("ashtonbee latest event = %#v, want continued", ashtonbee.Streak.LatestEvent)
	}
}

func TestPresenceRouteRequiresAuthenticatedSession(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	response := env.doRequest(t, http.MethodGet, "/api/v1/presence", nil)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("response.Code = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func decodePresenceSummaryResponse(t *testing.T, response *httptest.ResponseRecorder) presence.Summary {
	t.Helper()

	var summary presence.Summary
	if err := json.Unmarshal(response.Body.Bytes(), &summary); err != nil {
		t.Fatalf("json.Unmarshal(presence summary) error = %v", err)
	}
	return summary
}
