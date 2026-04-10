package server

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/authz"
)

func TestCompetitionHistoryRuntimeDoesNotMutateAresMembershipOrOtherDomains(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-competition-history-integrity-001", "competition-history-integrity-001@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)
	memberTwoCookie, memberTwo := createVerifiedSessionViaHTTP(t, env, "student-competition-history-integrity-002", "competition-history-integrity-002@example.com")

	for _, cookie := range []*http.Cookie{ownerCookie, memberTwoCookie} {
		makeEligibleForLobby(t, env, cookie)
		joinLobbyMembership(t, env, cookie)
	}

	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.claimed_tags (user_id, tag_hash, label) VALUES ($1, $2, $3)", owner.ID, "tag-competition-history-integrity-001", "competition history"); err != nil {
		t.Fatalf("Exec(insert claimed tag) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.visits (user_id, facility_key, source_event_id, arrived_at) VALUES ($1, $2, $3, $4)", owner.ID, "ashtonbee", "visit-competition-history-integrity-001", time.Date(2026, 4, 8, 18, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Exec(insert visit) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.workouts (user_id, started_at, status) VALUES ($1, $2, $3)", owner.ID, time.Date(2026, 4, 8, 18, 5, 0, 0, time.UTC), "in_progress"); err != nil {
		t.Fatalf("Exec(insert workout) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.recommendations (user_id, content, model_used) VALUES ($1, $2, $3)", owner.ID, `{"type":"history-check"}`, "test"); err != nil {
		t.Fatalf("Exec(insert recommendation) error = %v", err)
	}

	beforeVisits := countRows(t, env, "apollo.visits", owner.ID)
	beforeWorkouts := countRows(t, env, "apollo.workouts", owner.ID)
	beforeRecommendations := countRows(t, env, "apollo.recommendations", owner.ID)
	beforeClaimedTags := countRows(t, env, "apollo.claimed_tags", owner.ID)
	beforePreferences := lookupPreferences(t, env, owner.ID)
	beforeMembership := lookupMembershipSnapshot(t, env)
	beforeAresMatches := countTableRows(t, env, "apollo.ares_matches")
	beforeAresPlayers := countTableRows(t, env, "apollo.ares_match_players")

	session := createStartedCompetitionSession(t, env, ownerCookie, "Tracer 22 Integrity", "badminton", "gym-floor", 1, []uuid.UUID{owner.ID, memberTwo.ID})
	recordCompetitionResult(t, env, ownerCookie, session.ID.String(), session.Matches[0].ID.String(), session.Matches[0].SideSlots, []string{"win", "loss"})

	if afterVisits := countRows(t, env, "apollo.visits", owner.ID); afterVisits != beforeVisits {
		t.Fatalf("visit count changed from %d to %d after result capture", beforeVisits, afterVisits)
	}
	if afterWorkouts := countRows(t, env, "apollo.workouts", owner.ID); afterWorkouts != beforeWorkouts {
		t.Fatalf("workout count changed from %d to %d after result capture", beforeWorkouts, afterWorkouts)
	}
	if afterRecommendations := countRows(t, env, "apollo.recommendations", owner.ID); afterRecommendations != beforeRecommendations {
		t.Fatalf("recommendation count changed from %d to %d after result capture", beforeRecommendations, afterRecommendations)
	}
	if afterClaimedTags := countRows(t, env, "apollo.claimed_tags", owner.ID); afterClaimedTags != beforeClaimedTags {
		t.Fatalf("claimed tag count changed from %d to %d after result capture", beforeClaimedTags, afterClaimedTags)
	}
	if afterPreferences := lookupPreferences(t, env, owner.ID); afterPreferences != beforePreferences {
		t.Fatalf("preferences changed from %q to %q after result capture", beforePreferences, afterPreferences)
	}
	if afterMembership := lookupMembershipSnapshot(t, env); afterMembership != beforeMembership {
		t.Fatalf("membership snapshot changed from %q to %q after result capture", beforeMembership, afterMembership)
	}
	if afterAresMatches := countTableRows(t, env, "apollo.ares_matches"); afterAresMatches != beforeAresMatches {
		t.Fatalf("ares match count changed from %d to %d after result capture", beforeAresMatches, afterAresMatches)
	}
	if afterAresPlayers := countTableRows(t, env, "apollo.ares_match_players"); afterAresPlayers != beforeAresPlayers {
		t.Fatalf("ares match player count changed from %d to %d after result capture", beforeAresPlayers, afterAresPlayers)
	}
}
