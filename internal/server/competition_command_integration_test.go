package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/competition"
)

func TestCompetitionCommandReadinessIsBackedByAPOLLOCapabilities(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	supervisorCookie, supervisor := createVerifiedSessionViaHTTP(t, env, "student-command-readiness-001", "command-readiness-001@example.com")
	memberCookie, _ := createVerifiedSessionViaHTTP(t, env, "student-command-readiness-002", "command-readiness-002@example.com")
	setUserRole(t, env, supervisor.ID, authz.RoleSupervisor)

	supervisorResponse := env.doRequest(t, http.MethodGet, "/api/v1/competition/commands/readiness", nil, supervisorCookie)
	if supervisorResponse.Code != http.StatusOK {
		t.Fatalf("supervisorResponse.Code = %d, want %d body=%s", supervisorResponse.Code, http.StatusOK, supervisorResponse.Body.String())
	}
	supervisorReadiness := decodeCompetitionReadiness(t, supervisorResponse.Body.Bytes())
	assertReadinessCommand(t, supervisorReadiness, competition.CommandOpenQueue, true)
	assertReadinessCommand(t, supervisorReadiness, competition.CommandCreateTeam, false)

	memberResponse := env.doRequest(t, http.MethodGet, "/api/v1/competition/commands/readiness", nil, memberCookie)
	if memberResponse.Code != http.StatusOK {
		t.Fatalf("memberResponse.Code = %d, want %d body=%s", memberResponse.Code, http.StatusOK, memberResponse.Body.String())
	}
	memberReadiness := decodeCompetitionReadiness(t, memberResponse.Body.Bytes())
	if memberReadiness.Status != "unsupported_role" {
		t.Fatalf("memberReadiness.Status = %q, want unsupported_role", memberReadiness.Status)
	}
	assertReadinessCommand(t, memberReadiness, competition.CommandOpenQueue, false)
}

func TestCompetitionCommandEndpointDryRunDoesNotMutate(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	managerCookie, manager := createVerifiedSessionViaHTTP(t, env, "student-command-dry-run-001", "command-dry-run-001@example.com")
	setUserRole(t, env, manager.ID, authz.RoleManager)

	beforeRows := countTableRows(t, env, "apollo.competition_sessions")
	response := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/commands", `{
		"name":"create_session",
		"dry_run":true,
		"create_session":{
			"display_name":"Command Dry Run",
			"sport_key":"badminton",
			"facility_key":"ashtonbee",
			"zone_key":"gym-floor",
			"participants_per_side":1
		}
	}`, managerCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("response.Code = %d, want %d body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	outcome := decodeCompetitionCommandOutcome(t, response.Body.Bytes())
	if outcome.Status != competition.CommandStatusPlanned {
		t.Fatalf("outcome.Status = %q, want %q", outcome.Status, competition.CommandStatusPlanned)
	}
	if outcome.Mutated {
		t.Fatal("outcome.Mutated = true, want false")
	}
	if afterRows := countTableRows(t, env, "apollo.competition_sessions"); afterRows != beforeRows {
		t.Fatalf("competition_sessions changed from %d to %d after dry-run", beforeRows, afterRows)
	}
}

func TestCompetitionCommandEndpointCreatesExistingSessionBehavior(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	managerCookie, manager := createVerifiedSessionViaHTTP(t, env, "student-command-create-001", "command-create-001@example.com")
	setUserRole(t, env, manager.ID, authz.RoleManager)

	response := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/commands", `{
		"name":"create_session",
		"create_session":{
			"display_name":"Command Created Session",
			"sport_key":"badminton",
			"facility_key":"ashtonbee",
			"zone_key":"gym-floor",
			"participants_per_side":1
		}
	}`, managerCookie)
	if response.Code != http.StatusOK {
		t.Fatalf("response.Code = %d, want %d body=%s", response.Code, http.StatusOK, response.Body.String())
	}
	outcome := decodeCompetitionCommandOutcome(t, response.Body.Bytes())
	if outcome.Status != competition.CommandStatusSucceeded {
		t.Fatalf("outcome.Status = %q, want %q", outcome.Status, competition.CommandStatusSucceeded)
	}
	if !outcome.Mutated {
		t.Fatal("outcome.Mutated = false, want true")
	}

	listResponse := env.doRequest(t, http.MethodGet, "/api/v1/competition/sessions", nil, managerCookie)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("listResponse.Code = %d, want %d body=%s", listResponse.Code, http.StatusOK, listResponse.Body.String())
	}
	if got := listResponse.Body.String(); !containsJSONText(got, "Command Created Session") {
		t.Fatalf("competition session list missing command-created session: %s", got)
	}
}

func TestCompetitionCommandEndpointDeniesUnsupportedRole(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	memberCookie, _ := createVerifiedSessionViaHTTP(t, env, "student-command-denied-001", "command-denied-001@example.com")

	response := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/commands", `{
		"name":"create_session",
		"dry_run":true,
		"create_session":{
			"display_name":"Denied Member Session",
			"sport_key":"badminton",
			"facility_key":"ashtonbee",
			"participants_per_side":1
		}
	}`, memberCookie)
	if response.Code != http.StatusForbidden {
		t.Fatalf("response.Code = %d, want %d body=%s", response.Code, http.StatusForbidden, response.Body.String())
	}
	outcome := decodeCompetitionCommandOutcome(t, response.Body.Bytes())
	if outcome.Status != competition.CommandStatusDenied {
		t.Fatalf("outcome.Status = %q, want %q", outcome.Status, competition.CommandStatusDenied)
	}
}

func TestCompetitionCommandEndpointKeepsResultApplyDeferred(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-command-result-001", "command-result-001@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)

	createResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/sessions", `{
		"display_name":"Result Deferred Session",
		"sport_key":"badminton",
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"participants_per_side":1
	}`, ownerCookie)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("createResponse.Code = %d, want %d body=%s", createResponse.Code, http.StatusCreated, createResponse.Body.String())
	}
	session := decodeCompetitionSession(t, createResponse)

	response := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/commands", fmt.Sprintf(`{
		"name":"record_match_result",
		"session_id":"%s",
		"match_id":"11111111-1111-1111-1111-111111111111",
		"match_result":{
			"sides":[
				{"side_index":1,"competition_session_team_id":"22222222-2222-2222-2222-222222222222","outcome":"win"},
				{"side_index":2,"competition_session_team_id":"33333333-3333-3333-3333-333333333333","outcome":"loss"}
			]
		}
	}`, session.ID), ownerCookie)
	if response.Code != http.StatusConflict {
		t.Fatalf("response.Code = %d, want %d body=%s", response.Code, http.StatusConflict, response.Body.String())
	}
	outcome := decodeCompetitionCommandOutcome(t, response.Body.Bytes())
	if outcome.Error != competition.ErrCommandApplyUnsupported.Error() {
		t.Fatalf("outcome.Error = %q, want %q", outcome.Error, competition.ErrCommandApplyUnsupported.Error())
	}
}

func decodeCompetitionReadiness(t *testing.T, raw []byte) competition.CompetitionCommandReadiness {
	t.Helper()

	var payload competition.CompetitionCommandReadiness
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal(readiness) error = %v raw=%s", err, string(raw))
	}
	return payload
}

func decodeCompetitionCommandOutcome(t *testing.T, raw []byte) competition.CompetitionCommandOutcome {
	t.Helper()

	var payload competition.CompetitionCommandOutcome
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal(command outcome) error = %v raw=%s", err, string(raw))
	}
	return payload
}

func assertReadinessCommand(t *testing.T, readiness competition.CompetitionCommandReadiness, name competition.CommandName, available bool) {
	t.Helper()

	for _, command := range readiness.Commands {
		if command.Name == name {
			if command.Available != available {
				t.Fatalf("command %s available = %t, want %t", name, command.Available, available)
			}
			return
		}
	}
	t.Fatalf("readiness missing command %s", name)
}

func containsJSONText(body string, value string) bool {
	return strings.Contains(body, value)
}
