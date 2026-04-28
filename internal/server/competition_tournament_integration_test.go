package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/competition"
)

func TestCompetitionTournamentRuntimeSupportsInternalBracketAdvancementFromFinalizedResult(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-tournament-runtime-001", "tournament-runtime-001@example.com")
	memberCookie, member := createVerifiedSessionViaHTTP(t, env, "student-tournament-runtime-002", "tournament-runtime-002@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)
	for _, cookie := range []*http.Cookie{ownerCookie, memberCookie} {
		makeEligibleForLobby(t, env, cookie)
		joinLobbyMembership(t, env, cookie)
	}

	session := createStartedCompetitionSession(t, env, ownerCookie, "Tournament Runtime Session", "badminton", "gym-floor", 1, []uuid.UUID{owner.ID, member.ID})
	match := session.Matches[0]
	tournament := createBoundTournament(t, env, ownerCookie, "Internal Runtime Tournament", session)

	finalized := recordCompetitionResult(t, env, ownerCookie, session.ID.String(), match.ID.String(), match.SideSlots, []string{"win", "loss"})
	finalizedMatch := finalized.Matches[0]

	beforeResults := countTableRows(t, env, "apollo.competition_match_results")
	beforeResultSides := countTableRows(t, env, "apollo.competition_match_result_sides")
	beforeRatingEvents := countTableRows(t, env, "apollo.competition_rating_events")
	beforeRatings := countTableRows(t, env, "apollo.competition_member_ratings")
	beforeAnalyticsEvents := countTableRows(t, env, "apollo.competition_analytics_events")
	beforeAnalyticsProjections := countTableRows(t, env, "apollo.competition_analytics_projections")
	beforeAresMatches := countTableRows(t, env, "apollo.ares_matches")
	beforeAresPlayers := countTableRows(t, env, "apollo.ares_match_players")
	beforePreviews := countTableRows(t, env, "apollo.competition_match_previews")
	beforePreviewEvents := countTableRows(t, env, "apollo.competition_match_preview_events")

	advanceResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/tournaments/%s/rounds/advance", tournament.ID), fmt.Sprintf(`{
		"expected_tournament_version":%d,
		"match_binding_id":"%s",
		"advance_reason":"canonical_result_win"
	}`, tournament.TournamentVersion, tournament.Brackets[0].MatchBindings[0].ID), ownerCookie)
	if advanceResponse.Code != http.StatusOK {
		t.Fatalf("advanceResponse.Code = %d, want %d body=%s", advanceResponse.Code, http.StatusOK, advanceResponse.Body.String())
	}
	advanced := decodeCompetitionTournament(t, advanceResponse)
	if advanced.Status != competition.TournamentStatusCompleted {
		t.Fatalf("advanced.Status = %q, want completed", advanced.Status)
	}
	if len(advanced.Brackets[0].Advancements) != 1 {
		t.Fatalf("advancement count = %d, want 1", len(advanced.Brackets[0].Advancements))
	}
	advancement := advanced.Brackets[0].Advancements[0]
	if advancement.CanonicalResultID != *finalizedMatch.CanonicalResultID {
		t.Fatalf("advancement canonical result = %s, want %s", advancement.CanonicalResultID, *finalizedMatch.CanonicalResultID)
	}
	if advancement.WinningTeamSnapshotID != snapshotBySeed(t, advanced, 1).ID {
		t.Fatalf("winner snapshot = %s, want seed 1 snapshot %s", advancement.WinningTeamSnapshotID, snapshotBySeed(t, advanced, 1).ID)
	}

	assertTableRowsUnchanged(t, env, "apollo.competition_match_results", beforeResults)
	assertTableRowsUnchanged(t, env, "apollo.competition_match_result_sides", beforeResultSides)
	assertTableRowsUnchanged(t, env, "apollo.competition_rating_events", beforeRatingEvents)
	assertTableRowsUnchanged(t, env, "apollo.competition_member_ratings", beforeRatings)
	assertTableRowsUnchanged(t, env, "apollo.competition_analytics_events", beforeAnalyticsEvents)
	assertTableRowsUnchanged(t, env, "apollo.competition_analytics_projections", beforeAnalyticsProjections)
	assertTableRowsUnchanged(t, env, "apollo.ares_matches", beforeAresMatches)
	assertTableRowsUnchanged(t, env, "apollo.ares_match_players", beforeAresPlayers)
	assertTableRowsUnchanged(t, env, "apollo.competition_match_previews", beforePreviews)
	assertTableRowsUnchanged(t, env, "apollo.competition_match_preview_events", beforePreviewEvents)

	if eventCount := countTournamentEvents(t, env, advanced.ID); eventCount != 7 {
		t.Fatalf("tournament event count = %d, want 7", eventCount)
	}
}

func TestCompetitionTournamentRuntimeRejectsNonFinalDisputedAndVoidedResultAdvancement(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-tournament-guards-001", "tournament-guards-001@example.com")
	memberCookie, member := createVerifiedSessionViaHTTP(t, env, "student-tournament-guards-002", "tournament-guards-002@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)
	for _, cookie := range []*http.Cookie{ownerCookie, memberCookie} {
		makeEligibleForLobby(t, env, cookie)
		joinLobbyMembership(t, env, cookie)
	}

	session := createStartedCompetitionSession(t, env, ownerCookie, "Tournament Guard Session", "badminton", "gym-floor", 1, []uuid.UUID{owner.ID, member.ID})
	match := session.Matches[0]
	tournament := createBoundTournament(t, env, ownerCookie, "Internal Guard Tournament", session)

	assertTournamentAdvanceConflict(t, env, ownerCookie, tournament, competition.ErrTournamentAdvanceResultRequired.Error())

	recordResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result", session.ID, match.ID), buildResultRequestBodyWithVersion(match.SideSlots, []string{"win", "loss"}, match.ResultVersion), ownerCookie)
	if recordResponse.Code != http.StatusOK {
		t.Fatalf("recordResponse.Code = %d, want %d body=%s", recordResponse.Code, http.StatusOK, recordResponse.Body.String())
	}
	recorded := decodeCompetitionSession(t, recordResponse)
	recordedMatch := recorded.Matches[0]
	assertTournamentAdvanceConflict(t, env, ownerCookie, tournament, competition.ErrMatchResultNotFinal.Error())

	finalizeResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result/finalize", session.ID, match.ID), fmt.Sprintf(`{"expected_result_version":%d}`, recordedMatch.ResultVersion), ownerCookie)
	if finalizeResponse.Code != http.StatusOK {
		t.Fatalf("finalizeResponse.Code = %d, want %d body=%s", finalizeResponse.Code, http.StatusOK, finalizeResponse.Body.String())
	}
	finalized := decodeCompetitionSession(t, finalizeResponse)
	finalizedMatch := finalized.Matches[0]

	disputeResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result/dispute", session.ID, match.ID), fmt.Sprintf(`{"expected_result_version":%d}`, finalizedMatch.ResultVersion), ownerCookie)
	if disputeResponse.Code != http.StatusOK {
		t.Fatalf("disputeResponse.Code = %d, want %d body=%s", disputeResponse.Code, http.StatusOK, disputeResponse.Body.String())
	}
	disputed := decodeCompetitionSession(t, disputeResponse)
	disputedMatch := disputed.Matches[0]
	assertTournamentAdvanceConflict(t, env, ownerCookie, tournament, competition.ErrMatchResultNotFinal.Error())

	voidResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result/void", session.ID, match.ID), fmt.Sprintf(`{"expected_result_version":%d}`, disputedMatch.ResultVersion), ownerCookie)
	if voidResponse.Code != http.StatusOK {
		t.Fatalf("voidResponse.Code = %d, want %d body=%s", voidResponse.Code, http.StatusOK, voidResponse.Body.String())
	}
	assertTournamentAdvanceConflict(t, env, ownerCookie, tournament, competition.ErrMatchResultNotFinal.Error())
}

func TestCompetitionTournamentRuntimeAdvancesFromCorrectedCanonicalResult(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-tournament-corrected-001", "tournament-corrected-001@example.com")
	memberCookie, member := createVerifiedSessionViaHTTP(t, env, "student-tournament-corrected-002", "tournament-corrected-002@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)
	for _, cookie := range []*http.Cookie{ownerCookie, memberCookie} {
		makeEligibleForLobby(t, env, cookie)
		joinLobbyMembership(t, env, cookie)
	}

	session := createStartedCompetitionSession(t, env, ownerCookie, "Tournament Corrected Session", "badminton", "gym-floor", 1, []uuid.UUID{owner.ID, member.ID})
	match := session.Matches[0]
	tournament := createBoundTournament(t, env, ownerCookie, "Internal Corrected Tournament", session)

	finalized := recordCompetitionResult(t, env, ownerCookie, session.ID.String(), match.ID.String(), match.SideSlots, []string{"win", "loss"})
	finalizedMatch := finalized.Matches[0]
	disputeResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result/dispute", session.ID, match.ID), fmt.Sprintf(`{"expected_result_version":%d}`, finalizedMatch.ResultVersion), ownerCookie)
	if disputeResponse.Code != http.StatusOK {
		t.Fatalf("disputeResponse.Code = %d, want %d body=%s", disputeResponse.Code, http.StatusOK, disputeResponse.Body.String())
	}
	disputed := decodeCompetitionSession(t, disputeResponse)
	disputedMatch := disputed.Matches[0]
	correctResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/matches/%s/result/correct", session.ID, match.ID), buildResultRequestBodyWithVersion(disputedMatch.SideSlots, []string{"loss", "win"}, disputedMatch.ResultVersion), ownerCookie)
	if correctResponse.Code != http.StatusOK {
		t.Fatalf("correctResponse.Code = %d, want %d body=%s", correctResponse.Code, http.StatusOK, correctResponse.Body.String())
	}
	corrected := decodeCompetitionSession(t, correctResponse)
	correctedMatch := corrected.Matches[0]

	advanceResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/tournaments/%s/rounds/advance", tournament.ID), fmt.Sprintf(`{
		"expected_tournament_version":%d,
		"match_binding_id":"%s",
		"advance_reason":"canonical_result_win"
	}`, tournament.TournamentVersion, tournament.Brackets[0].MatchBindings[0].ID), ownerCookie)
	if advanceResponse.Code != http.StatusOK {
		t.Fatalf("advanceResponse.Code = %d, want %d body=%s", advanceResponse.Code, http.StatusOK, advanceResponse.Body.String())
	}
	advanced := decodeCompetitionTournament(t, advanceResponse)
	advancement := advanced.Brackets[0].Advancements[0]
	if advancement.CanonicalResultID != *correctedMatch.CanonicalResultID {
		t.Fatalf("advancement canonical result = %s, want corrected result %s", advancement.CanonicalResultID, *correctedMatch.CanonicalResultID)
	}
	if advancement.WinningTeamSnapshotID != snapshotBySeed(t, advanced, 2).ID {
		t.Fatalf("winner snapshot = %s, want corrected seed 2 snapshot %s", advancement.WinningTeamSnapshotID, snapshotBySeed(t, advanced, 2).ID)
	}
}

func TestCompetitionTournamentCommandsAreCapabilityGated(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	supervisorCookie, supervisor := createVerifiedSessionViaHTTP(t, env, "student-tournament-command-001", "tournament-command-001@example.com")
	memberCookie, _ := createVerifiedSessionViaHTTP(t, env, "student-tournament-command-002", "tournament-command-002@example.com")
	setUserRole(t, env, supervisor.ID, authz.RoleSupervisor)

	supervisorResponse := env.doRequest(t, http.MethodGet, "/api/v1/competition/commands/readiness", nil, supervisorCookie)
	if supervisorResponse.Code != http.StatusOK {
		t.Fatalf("supervisorResponse.Code = %d, want %d body=%s", supervisorResponse.Code, http.StatusOK, supervisorResponse.Body.String())
	}
	readiness := decodeCompetitionReadiness(t, supervisorResponse.Body.Bytes())
	assertReadinessCommand(t, readiness, competition.CommandCreateTournament, false)
	assertReadinessCommand(t, readiness, competition.CommandAdvanceTournamentRound, true)

	memberResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/commands", `{
		"name":"create_tournament",
		"dry_run":true,
		"create_tournament":{
			"display_name":"Denied Tournament",
			"format":"single_elimination",
			"sport_key":"badminton",
			"facility_key":"ashtonbee",
			"zone_key":"gym-floor",
			"participants_per_side":1
		}
	}`, memberCookie)
	if memberResponse.Code != http.StatusForbidden {
		t.Fatalf("memberResponse.Code = %d, want %d body=%s", memberResponse.Code, http.StatusForbidden, memberResponse.Body.String())
	}
	outcome := decodeCompetitionCommandOutcome(t, memberResponse.Body.Bytes())
	if outcome.Status != competition.CommandStatusDenied {
		t.Fatalf("outcome.Status = %q, want denied", outcome.Status)
	}
}

func createBoundTournament(t *testing.T, env *authProfileServerEnv, ownerCookie *http.Cookie, displayName string, session competition.Session) competition.Tournament {
	t.Helper()

	createResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/tournaments", fmt.Sprintf(`{
		"display_name":"%s",
		"format":"single_elimination",
		"sport_key":"badminton",
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"participants_per_side":1
	}`, displayName), ownerCookie)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create tournament code = %d, want %d body=%s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}
	tournament := decodeCompetitionTournament(t, createResponse)
	match := session.Matches[0]

	seedResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/tournaments/%s/seed", tournament.ID), fmt.Sprintf(`{
		"expected_tournament_version":%d,
		"seeds":[
			{"seed":1,"competition_session_team_id":"%s"},
			{"seed":2,"competition_session_team_id":"%s"}
		]
	}`, tournament.TournamentVersion, match.SideSlots[0].TeamID, match.SideSlots[1].TeamID), ownerCookie)
	if seedResponse.Code != http.StatusOK {
		t.Fatalf("seed tournament code = %d, want %d body=%s", seedResponse.Code, http.StatusOK, seedResponse.Body.String())
	}
	tournament = decodeCompetitionTournament(t, seedResponse)

	for seed := 1; seed <= 2; seed++ {
		lockResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/tournaments/%s/teams/lock", tournament.ID), fmt.Sprintf(`{
			"expected_tournament_version":%d,
			"seed":%d
		}`, tournament.TournamentVersion, seed), ownerCookie)
		if lockResponse.Code != http.StatusOK {
			t.Fatalf("lock seed %d code = %d, want %d body=%s", seed, lockResponse.Code, http.StatusOK, lockResponse.Body.String())
		}
		tournament = decodeCompetitionTournament(t, lockResponse)
	}

	bindResponse := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/tournaments/%s/matches/bind", tournament.ID), fmt.Sprintf(`{
		"expected_tournament_version":%d,
		"round":1,
		"match_number":1,
		"competition_match_id":"%s",
		"team_snapshot_ids":["%s","%s"]
	}`, tournament.TournamentVersion, match.ID, snapshotBySeed(t, tournament, 1).ID, snapshotBySeed(t, tournament, 2).ID), ownerCookie)
	if bindResponse.Code != http.StatusOK {
		t.Fatalf("bind tournament code = %d, want %d body=%s", bindResponse.Code, http.StatusOK, bindResponse.Body.String())
	}
	return decodeCompetitionTournament(t, bindResponse)
}

func decodeCompetitionTournament(t *testing.T, response *httptest.ResponseRecorder) competition.Tournament {
	t.Helper()

	var payload competition.Tournament
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal(competition tournament) error = %v body=%s", err, response.Body.String())
	}
	return payload
}

func snapshotBySeed(t *testing.T, tournament competition.Tournament, seed int) competition.TournamentTeamSnapshot {
	t.Helper()

	for _, snapshot := range tournament.Brackets[0].TeamSnapshots {
		if snapshot.Seed == seed {
			return snapshot
		}
	}
	t.Fatalf("missing tournament snapshot for seed %d", seed)
	return competition.TournamentTeamSnapshot{}
}

func assertTournamentAdvanceConflict(t *testing.T, env *authProfileServerEnv, ownerCookie *http.Cookie, tournament competition.Tournament, wantError string) {
	t.Helper()

	response := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/tournaments/%s/rounds/advance", tournament.ID), fmt.Sprintf(`{
		"expected_tournament_version":%d,
		"match_binding_id":"%s",
		"advance_reason":"canonical_result_win"
	}`, tournament.TournamentVersion, tournament.Brackets[0].MatchBindings[0].ID), ownerCookie)
	if response.Code != http.StatusConflict {
		t.Fatalf("advance conflict code = %d, want %d body=%s", response.Code, http.StatusConflict, response.Body.String())
	}
	if got := response.Body.String(); !containsJSONText(got, wantError) {
		t.Fatalf("advance conflict body = %s, want %q", got, wantError)
	}
}

func assertTableRowsUnchanged(t *testing.T, env *authProfileServerEnv, table string, before int) {
	t.Helper()

	if after := countTableRows(t, env, table); after != before {
		t.Fatalf("%s rows changed from %d to %d", table, before, after)
	}
}

func countTournamentEvents(t *testing.T, env *authProfileServerEnv, tournamentID uuid.UUID) int {
	t.Helper()

	var count int
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT count(*)
FROM apollo.competition_tournament_events
WHERE tournament_id = $1
`, tournamentID).Scan(&count); err != nil {
		t.Fatalf("count tournament events error = %v", err)
	}
	return count
}
