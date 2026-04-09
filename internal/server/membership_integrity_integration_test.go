package server

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestLobbyMembershipRuntimeDoesNotMutateVisitsWorkoutsRecommendationsOrClaimedTags(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	cookie, user := createVerifiedSessionViaHTTP(t, env, "student-membership-014", "membership-014@example.com")

	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.claimed_tags (user_id, tag_hash, label) VALUES ($1, $2, $3)", user.ID, "tag-membership-014", "membership tag"); err != nil {
		t.Fatalf("Exec(insert claimed tag) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.visits (user_id, facility_key, source_event_id, arrived_at) VALUES ($1, $2, $3, $4)", user.ID, "ashtonbee", "visit-membership-014", time.Date(2026, 4, 5, 18, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Exec(insert visit) error = %v", err)
	}

	patchEligibilityResponse := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{"visibility_mode":"discoverable","availability_mode":"available_now"}`, cookie)
	if patchEligibilityResponse.Code != http.StatusOK {
		t.Fatalf("patchEligibilityResponse.Code = %d, want %d", patchEligibilityResponse.Code, http.StatusOK)
	}

	recommendationResponse := env.doRequest(t, http.MethodGet, "/api/v1/recommendations/workout", nil, cookie)
	if recommendationResponse.Code != http.StatusOK {
		t.Fatalf("recommendationResponse.Code = %d, want %d", recommendationResponse.Code, http.StatusOK)
	}

	createWorkoutResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/workouts", `{"notes":"membership isolation"}`, cookie)
	if createWorkoutResponse.Code != http.StatusCreated {
		t.Fatalf("createWorkoutResponse.Code = %d, want %d", createWorkoutResponse.Code, http.StatusCreated)
	}

	initialMembershipResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/membership", nil, cookie)
	if initialMembershipResponse.Code != http.StatusOK {
		t.Fatalf("initialMembershipResponse.Code = %d, want %d", initialMembershipResponse.Code, http.StatusOK)
	}
	initialMembership := decodeMembershipResponse(t, initialMembershipResponse)
	if initialMembership.Status != "not_joined" {
		t.Fatalf("initialMembership.Status = %q, want %q", initialMembership.Status, "not_joined")
	}

	beforeVisits := countRows(t, env, "apollo.visits", user.ID)
	beforeWorkouts := countRows(t, env, "apollo.workouts", user.ID)
	beforeClaimedTags := countRows(t, env, "apollo.claimed_tags", user.ID)
	beforePreferences := lookupPreferences(t, env, user.ID)
	beforeCompetitionSessions := countTableRows(t, env, "apollo.competition_sessions")
	beforeCompetitionTeams := countTableRows(t, env, "apollo.competition_session_teams")
	beforeCompetitionRosterMembers := countTableRows(t, env, "apollo.competition_team_roster_members")
	beforeCompetitionMatches := countTableRows(t, env, "apollo.competition_matches")
	beforeCompetitionMatchSides := countTableRows(t, env, "apollo.competition_match_side_slots")

	joinResponse := env.doRequest(t, http.MethodPost, "/api/v1/lobby/membership/join", nil, cookie)
	if joinResponse.Code != http.StatusOK {
		t.Fatalf("joinResponse.Code = %d, want %d", joinResponse.Code, http.StatusOK)
	}

	leaveResponse := env.doRequest(t, http.MethodPost, "/api/v1/lobby/membership/leave", nil, cookie)
	if leaveResponse.Code != http.StatusOK {
		t.Fatalf("leaveResponse.Code = %d, want %d", leaveResponse.Code, http.StatusOK)
	}

	if afterVisits := countRows(t, env, "apollo.visits", user.ID); afterVisits != beforeVisits {
		t.Fatalf("visit count changed from %d to %d after lobby membership transitions", beforeVisits, afterVisits)
	}
	if afterWorkouts := countRows(t, env, "apollo.workouts", user.ID); afterWorkouts != beforeWorkouts {
		t.Fatalf("workout count changed from %d to %d after lobby membership transitions", beforeWorkouts, afterWorkouts)
	}
	if afterClaimedTags := countRows(t, env, "apollo.claimed_tags", user.ID); afterClaimedTags != beforeClaimedTags {
		t.Fatalf("claimed tag count changed from %d to %d after lobby membership transitions", beforeClaimedTags, afterClaimedTags)
	}
	if afterPreferences := lookupPreferences(t, env, user.ID); afterPreferences != beforePreferences {
		t.Fatalf("preferences changed from %q to %q after lobby membership transitions", beforePreferences, afterPreferences)
	}
	if afterCompetitionSessions := countTableRows(t, env, "apollo.competition_sessions"); afterCompetitionSessions != beforeCompetitionSessions {
		t.Fatalf("competition session count changed from %d to %d after lobby membership transitions", beforeCompetitionSessions, afterCompetitionSessions)
	}
	if afterCompetitionTeams := countTableRows(t, env, "apollo.competition_session_teams"); afterCompetitionTeams != beforeCompetitionTeams {
		t.Fatalf("competition team count changed from %d to %d after lobby membership transitions", beforeCompetitionTeams, afterCompetitionTeams)
	}
	if afterCompetitionRosterMembers := countTableRows(t, env, "apollo.competition_team_roster_members"); afterCompetitionRosterMembers != beforeCompetitionRosterMembers {
		t.Fatalf("competition roster member count changed from %d to %d after lobby membership transitions", beforeCompetitionRosterMembers, afterCompetitionRosterMembers)
	}
	if afterCompetitionMatches := countTableRows(t, env, "apollo.competition_matches"); afterCompetitionMatches != beforeCompetitionMatches {
		t.Fatalf("competition match count changed from %d to %d after lobby membership transitions", beforeCompetitionMatches, afterCompetitionMatches)
	}
	if afterCompetitionMatchSides := countTableRows(t, env, "apollo.competition_match_side_slots"); afterCompetitionMatchSides != beforeCompetitionMatchSides {
		t.Fatalf("competition match side count changed from %d to %d after lobby membership transitions", beforeCompetitionMatchSides, afterCompetitionMatchSides)
	}
}
