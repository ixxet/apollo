package server

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestLobbyMatchPreviewRuntimeIsReadOnlyAcrossDomains(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	viewerCookie, viewer := createVerifiedSessionViaHTTP(t, env, "student-preview-integrity-001", "preview-integrity-001@example.com")
	memberTwoCookie, _ := createVerifiedSessionViaHTTP(t, env, "student-preview-integrity-002", "preview-integrity-002@example.com")

	for _, cookie := range []*http.Cookie{viewerCookie, memberTwoCookie} {
		makeEligibleForLobby(t, env, cookie)
		joinLobbyMembership(t, env, cookie)
	}

	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.claimed_tags (user_id, tag_hash, label) VALUES ($1, $2, $3)", viewer.ID, "tag-preview-integrity-001", "preview tag"); err != nil {
		t.Fatalf("Exec(insert claimed tag) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.visits (user_id, facility_key, source_event_id, arrived_at) VALUES ($1, $2, $3, $4)", viewer.ID, "ashtonbee", "visit-preview-integrity-001", time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Exec(insert visit) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.workouts (user_id, started_at, status) VALUES ($1, $2, $3)", viewer.ID, time.Date(2026, 4, 6, 12, 5, 0, 0, time.UTC), "in_progress"); err != nil {
		t.Fatalf("Exec(insert workout) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.recommendations (user_id, content, model_used) VALUES ($1, $2, $3)", viewer.ID, `{"type":"seeded"}`, "test"); err != nil {
		t.Fatalf("Exec(insert recommendation) error = %v", err)
	}

	beforeMembership := lookupMembershipSnapshot(t, env)
	beforeVisits := countRows(t, env, "apollo.visits", viewer.ID)
	beforeWorkouts := countRows(t, env, "apollo.workouts", viewer.ID)
	beforeRecommendations := countRows(t, env, "apollo.recommendations", viewer.ID)
	beforeClaimedTags := countRows(t, env, "apollo.claimed_tags", viewer.ID)
	beforeAresMatches := countTableRows(t, env, "apollo.ares_matches")
	beforeAresMatchPlayers := countTableRows(t, env, "apollo.ares_match_players")
	beforeCompetitionSessions := countTableRows(t, env, "apollo.competition_sessions")
	beforeCompetitionQueueMembers := countTableRows(t, env, "apollo.competition_session_queue_members")
	beforeCompetitionTeams := countTableRows(t, env, "apollo.competition_session_teams")
	beforeCompetitionRosterMembers := countTableRows(t, env, "apollo.competition_team_roster_members")
	beforeCompetitionMatches := countTableRows(t, env, "apollo.competition_matches")
	beforeCompetitionMatchSides := countTableRows(t, env, "apollo.competition_match_side_slots")

	firstResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/match-preview", nil, viewerCookie)
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("firstResponse.Code = %d, want %d", firstResponse.Code, http.StatusOK)
	}
	secondResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/match-preview", nil, viewerCookie)
	if secondResponse.Code != http.StatusOK {
		t.Fatalf("secondResponse.Code = %d, want %d", secondResponse.Code, http.StatusOK)
	}

	if afterMembership := lookupMembershipSnapshot(t, env); afterMembership != beforeMembership {
		t.Fatalf("membership snapshot changed from %q to %q after preview reads", beforeMembership, afterMembership)
	}
	if afterVisits := countRows(t, env, "apollo.visits", viewer.ID); afterVisits != beforeVisits {
		t.Fatalf("visit count changed from %d to %d after preview reads", beforeVisits, afterVisits)
	}
	if afterWorkouts := countRows(t, env, "apollo.workouts", viewer.ID); afterWorkouts != beforeWorkouts {
		t.Fatalf("workout count changed from %d to %d after preview reads", beforeWorkouts, afterWorkouts)
	}
	if afterRecommendations := countRows(t, env, "apollo.recommendations", viewer.ID); afterRecommendations != beforeRecommendations {
		t.Fatalf("recommendation count changed from %d to %d after preview reads", beforeRecommendations, afterRecommendations)
	}
	if afterClaimedTags := countRows(t, env, "apollo.claimed_tags", viewer.ID); afterClaimedTags != beforeClaimedTags {
		t.Fatalf("claimed tag count changed from %d to %d after preview reads", beforeClaimedTags, afterClaimedTags)
	}
	if afterAresMatches := countTableRows(t, env, "apollo.ares_matches"); afterAresMatches != beforeAresMatches {
		t.Fatalf("ares match count changed from %d to %d after preview reads", beforeAresMatches, afterAresMatches)
	}
	if afterAresMatchPlayers := countTableRows(t, env, "apollo.ares_match_players"); afterAresMatchPlayers != beforeAresMatchPlayers {
		t.Fatalf("ares match player count changed from %d to %d after preview reads", beforeAresMatchPlayers, afterAresMatchPlayers)
	}
	if afterCompetitionSessions := countTableRows(t, env, "apollo.competition_sessions"); afterCompetitionSessions != beforeCompetitionSessions {
		t.Fatalf("competition session count changed from %d to %d after preview reads", beforeCompetitionSessions, afterCompetitionSessions)
	}
	if afterCompetitionQueueMembers := countTableRows(t, env, "apollo.competition_session_queue_members"); afterCompetitionQueueMembers != beforeCompetitionQueueMembers {
		t.Fatalf("competition queue member count changed from %d to %d after preview reads", beforeCompetitionQueueMembers, afterCompetitionQueueMembers)
	}
	if afterCompetitionTeams := countTableRows(t, env, "apollo.competition_session_teams"); afterCompetitionTeams != beforeCompetitionTeams {
		t.Fatalf("competition team count changed from %d to %d after preview reads", beforeCompetitionTeams, afterCompetitionTeams)
	}
	if afterCompetitionRosterMembers := countTableRows(t, env, "apollo.competition_team_roster_members"); afterCompetitionRosterMembers != beforeCompetitionRosterMembers {
		t.Fatalf("competition roster member count changed from %d to %d after preview reads", beforeCompetitionRosterMembers, afterCompetitionRosterMembers)
	}
	if afterCompetitionMatches := countTableRows(t, env, "apollo.competition_matches"); afterCompetitionMatches != beforeCompetitionMatches {
		t.Fatalf("competition match count changed from %d to %d after preview reads", beforeCompetitionMatches, afterCompetitionMatches)
	}
	if afterCompetitionMatchSides := countTableRows(t, env, "apollo.competition_match_side_slots"); afterCompetitionMatchSides != beforeCompetitionMatchSides {
		t.Fatalf("competition match side count changed from %d to %d after preview reads", beforeCompetitionMatchSides, afterCompetitionMatchSides)
	}

	if string(firstResponse.Body.Bytes()) != string(secondResponse.Body.Bytes()) {
		t.Fatalf("preview changed between repeated reads without input changes\nfirst=%s\nsecond=%s", firstResponse.Body.Bytes(), secondResponse.Body.Bytes())
	}
}

func TestLobbyMatchPreviewRuntimeDoesNotDependOnVisitChanges(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	viewerCookie, _ := createVerifiedSessionViaHTTP(t, env, "student-preview-visit-001", "preview-visit-001@example.com")
	memberTwoCookie, memberTwo := createVerifiedSessionViaHTTP(t, env, "student-preview-visit-002", "preview-visit-002@example.com")

	for _, cookie := range []*http.Cookie{viewerCookie, memberTwoCookie} {
		makeEligibleForLobby(t, env, cookie)
		joinLobbyMembership(t, env, cookie)
	}

	firstResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/match-preview", nil, viewerCookie)
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("firstResponse.Code = %d, want %d", firstResponse.Code, http.StatusOK)
	}

	if _, err := env.db.DB.Exec(context.Background(), "INSERT INTO apollo.visits (user_id, facility_key, source_event_id, arrived_at) VALUES ($1, $2, $3, $4)", memberTwo.ID, "ashtonbee", "visit-preview-visit-002", time.Date(2026, 4, 6, 13, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Exec(insert visit) error = %v", err)
	}
	if _, err := env.db.DB.Exec(context.Background(), "UPDATE apollo.visits SET departed_at = $2 WHERE user_id = $1 AND source_event_id = $3", memberTwo.ID, time.Date(2026, 4, 6, 13, 30, 0, 0, time.UTC), "visit-preview-visit-002"); err != nil {
		t.Fatalf("Exec(update departed_at) error = %v", err)
	}

	secondResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/match-preview", nil, viewerCookie)
	if secondResponse.Code != http.StatusOK {
		t.Fatalf("secondResponse.Code = %d, want %d", secondResponse.Code, http.StatusOK)
	}

	if string(firstResponse.Body.Bytes()) != string(secondResponse.Body.Bytes()) {
		t.Fatalf("preview changed after visit-only updates\nfirst=%s\nsecond=%s", firstResponse.Body.Bytes(), secondResponse.Body.Bytes())
	}
}

func countTableRows(t *testing.T, env *authProfileServerEnv, table string) int {
	t.Helper()

	var count int
	query := "SELECT COUNT(*) FROM " + table
	if err := env.db.DB.QueryRow(context.Background(), query).Scan(&count); err != nil {
		t.Fatalf("QueryRow(%s) error = %v", table, err)
	}

	return count
}

func lookupMembershipSnapshot(t *testing.T, env *authProfileServerEnv) string {
	t.Helper()

	var snapshot string
	if err := env.db.DB.QueryRow(
		context.Background(),
		`SELECT COALESCE(json_agg(json_build_object(
			'user_id', user_id,
			'status', status,
			'joined_at', joined_at,
			'left_at', left_at,
			'updated_at', updated_at
		) ORDER BY user_id)::text, '[]')
		FROM apollo.lobby_memberships`,
	).Scan(&snapshot); err != nil {
		t.Fatalf("QueryRow(lobby snapshot) error = %v", err)
	}

	return snapshot
}
