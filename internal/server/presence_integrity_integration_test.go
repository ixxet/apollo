package server

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/ixxet/apollo/internal/presence"
	"github.com/ixxet/apollo/internal/visits"
)

func TestPresenceReadsDoNotMutatePresenceOrUnrelatedStateDomains(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	cookie, user := createVerifiedSessionViaHTTP(t, env, "student-presence-int-001", "presence-int-001@example.com")
	base := time.Now().UTC().Truncate(time.Second)
	presenceService := presence.NewService(presence.NewRepository(env.db.DB), visits.NewService(visits.NewRepository(env.db.DB)))

	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.claimed_tags (user_id, tag_hash, label) VALUES ($1, $2, $3)", user.ID, "presence-int-tag", "presence integrity tag"); err != nil {
		t.Fatalf("Exec(insert claimed tag) error = %v", err)
	}
	if _, err := presenceService.RecordArrival(context.Background(), visits.ArrivalInput{
		SourceEventID:        "presence-int-arrival-001",
		FacilityKey:          "ashtonbee",
		ExternalIdentityHash: "presence-int-tag",
		ArrivedAt:            base.AddDate(0, 0, -1).Add(-2 * time.Hour),
	}); err != nil {
		t.Fatalf("RecordArrival(day1) error = %v", err)
	}
	if _, err := presenceService.RecordDeparture(context.Background(), visits.DepartureInput{
		SourceEventID:        "presence-int-departure-001",
		FacilityKey:          "ashtonbee",
		ExternalIdentityHash: "presence-int-tag",
		DepartedAt:           base.AddDate(0, 0, -1).Add(-time.Hour),
	}); err != nil {
		t.Fatalf("RecordDeparture(day1) error = %v", err)
	}
	if _, err := presenceService.RecordArrival(context.Background(), visits.ArrivalInput{
		SourceEventID:        "presence-int-arrival-002",
		FacilityKey:          "ashtonbee",
		ExternalIdentityHash: "presence-int-tag",
		ArrivedAt:            base.Add(-90 * time.Minute),
	}); err != nil {
		t.Fatalf("RecordArrival(day2) error = %v", err)
	}

	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.workouts (user_id, started_at, status, finished_at) VALUES ($1, $2, $3, $4)", user.ID, base.Add(-4*time.Hour), "finished", base.Add(-3*time.Hour)); err != nil {
		t.Fatalf("Exec(insert workout) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.recommendations (user_id, content, model_used) VALUES ($1, '{}'::jsonb, 'presence-integrity-fixture')", user.ID); err != nil {
		t.Fatalf("Exec(insert recommendation) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.lobby_memberships (user_id, status, joined_at) VALUES ($1, 'joined', $2)", user.ID, base.Add(-30*time.Minute)); err != nil {
		t.Fatalf("Exec(insert membership) error = %v", err)
	}

	beforeVisits := countRows(t, env, "apollo.visits", user.ID)
	beforeTapLinks := countTotalRows(t, env, "apollo.visit_tap_links")
	beforeStreaks := countTotalRows(t, env, "apollo.member_presence_streaks")
	beforeStreakEvents := countTotalRows(t, env, "apollo.member_presence_streak_events")
	beforeWorkouts := countRows(t, env, "apollo.workouts", user.ID)
	beforeRecommendations := countTotalRows(t, env, "apollo.recommendations")
	beforeMembershipRows := countTotalRows(t, env, "apollo.lobby_memberships")
	beforeClaimedTags := countRows(t, env, "apollo.claimed_tags", user.ID)
	tagHashBefore := lookupTagHash(t, env, user.ID)

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

	if afterVisits := countRows(t, env, "apollo.visits", user.ID); afterVisits != beforeVisits {
		t.Fatalf("visit count changed from %d to %d", beforeVisits, afterVisits)
	}
	if afterTapLinks := countTotalRows(t, env, "apollo.visit_tap_links"); afterTapLinks != beforeTapLinks {
		t.Fatalf("tap-link row count changed from %d to %d", beforeTapLinks, afterTapLinks)
	}
	if afterStreaks := countTotalRows(t, env, "apollo.member_presence_streaks"); afterStreaks != beforeStreaks {
		t.Fatalf("streak row count changed from %d to %d", beforeStreaks, afterStreaks)
	}
	if afterStreakEvents := countTotalRows(t, env, "apollo.member_presence_streak_events"); afterStreakEvents != beforeStreakEvents {
		t.Fatalf("streak event row count changed from %d to %d", beforeStreakEvents, afterStreakEvents)
	}
	if afterWorkouts := countRows(t, env, "apollo.workouts", user.ID); afterWorkouts != beforeWorkouts {
		t.Fatalf("workout row count changed from %d to %d", beforeWorkouts, afterWorkouts)
	}
	if afterRecommendations := countTotalRows(t, env, "apollo.recommendations"); afterRecommendations != beforeRecommendations {
		t.Fatalf("recommendation row count changed from %d to %d", beforeRecommendations, afterRecommendations)
	}
	if afterMembershipRows := countTotalRows(t, env, "apollo.lobby_memberships"); afterMembershipRows != beforeMembershipRows {
		t.Fatalf("membership row count changed from %d to %d", beforeMembershipRows, afterMembershipRows)
	}
	if afterClaimedTags := countRows(t, env, "apollo.claimed_tags", user.ID); afterClaimedTags != beforeClaimedTags {
		t.Fatalf("claimed tag row count changed from %d to %d", beforeClaimedTags, afterClaimedTags)
	}
	if tagHashAfter := lookupTagHash(t, env, user.ID); tagHashAfter != tagHashBefore {
		t.Fatalf("tag hash changed from %q to %q", tagHashBefore, tagHashAfter)
	}
}
