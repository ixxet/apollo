package server

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/competition"
)

func TestCompetitionScaleCeilingAPIReadSmoke(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-scale-api-001", "scale-api-001@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)
	memberCookie, member := createVerifiedSessionViaHTTP(t, env, "student-scale-api-002", "scale-api-002@example.com")

	for _, cookie := range []*http.Cookie{ownerCookie, memberCookie} {
		makeEligibleForLobby(t, env, cookie)
		joinLobbyMembership(t, env, cookie)
	}

	session := createStartedCompetitionSession(t, env, ownerCookie, "Scale API Smoke", "badminton", "gym-floor", 1, []uuid.UUID{owner.ID, member.ID})
	recordCompetitionResult(t, env, ownerCookie, session.ID.String(), session.Matches[0].ID.String(), session.Matches[0].SideSlots, []string{"win", "loss"})

	smokeStartedAt := time.Now()
	readinessDurations := measureAPISmokeDurations(t, 7, func(t *testing.T) {
		response := env.doRequest(t, http.MethodGet, "/api/v1/public/competition/readiness", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("readiness response = %d body=%s", response.Code, response.Body.String())
		}
	})
	leaderboardDurations := measureAPISmokeDurations(t, 7, func(t *testing.T) {
		response := env.doRequest(t, http.MethodGet, "/api/v1/public/competition/leaderboards?sport_key=badminton&mode_key=head_to_head:s2-p1&stat_type=wins&team_scope=all&limit=1000", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("leaderboard response = %d body=%s", response.Code, response.Body.String())
		}
		var leaderboard competition.PublicCompetitionLeaderboard
		if err := json.Unmarshal(response.Body.Bytes(), &leaderboard); err != nil {
			t.Fatalf("json.Unmarshal(leaderboard) error = %v", err)
		}
		if len(leaderboard.Leaderboard) > 100 {
			t.Fatalf("leaderboard response rows = %d, want <= 100", len(leaderboard.Leaderboard))
		}
	})
	publicIdentityDurations := measureAPISmokeDurations(t, 7, func(t *testing.T) {
		response := env.doRequest(t, http.MethodGet, "/api/v1/public/competition/game-identity?sport_key=badminton&mode_key=head_to_head:s2-p1&limit=1000", nil)
		if response.Code != http.StatusOK {
			t.Fatalf("public game identity response = %d body=%s", response.Code, response.Body.String())
		}
		var identity competition.GameIdentityProjection
		if err := json.Unmarshal(response.Body.Bytes(), &identity); err != nil {
			t.Fatalf("json.Unmarshal(public identity) error = %v", err)
		}
		if len(identity.CP) > 50 {
			t.Fatalf("public game identity rows = %d, want <= 50", len(identity.CP))
		}
	})
	memberIdentityDurations := measureAPISmokeDurations(t, 7, func(t *testing.T) {
		response := env.doRequest(t, http.MethodGet, "/api/v1/competition/game-identity?limit=1000", nil, ownerCookie)
		if response.Code != http.StatusOK {
			t.Fatalf("member game identity response = %d body=%s", response.Code, response.Body.String())
		}
		var identity competition.GameIdentityProjection
		if err := json.Unmarshal(response.Body.Bytes(), &identity); err != nil {
			t.Fatalf("json.Unmarshal(member identity) error = %v", err)
		}
		if len(identity.CP) > 50 {
			t.Fatalf("member game identity rows = %d, want <= 50", len(identity.CP))
		}
	})
	commandReadinessDurations := measureAPISmokeDurations(t, 7, func(t *testing.T) {
		response := env.doRequest(t, http.MethodGet, "/api/v1/competition/commands/readiness", nil, ownerCookie)
		if response.Code != http.StatusOK {
			t.Fatalf("command readiness response = %d body=%s", response.Code, response.Body.String())
		}
	})
	smokeDuration := time.Since(smokeStartedAt)

	assertAPISmokeCeiling(t, "public_readiness", readinessDurations, 150*time.Millisecond, 300*time.Millisecond)
	assertAPISmokeCeiling(t, "public_leaderboard", leaderboardDurations, 300*time.Millisecond, 750*time.Millisecond)
	assertAPISmokeCeiling(t, "public_game_identity", publicIdentityDurations, 500*time.Millisecond, time.Second)
	assertAPISmokeCeiling(t, "member_game_identity", memberIdentityDurations, 500*time.Millisecond, time.Second)
	if smokeDuration > 30*time.Second {
		t.Fatalf("API smoke duration = %s, ceiling 30s", smokeDuration)
	}

	t.Logf("scale_ceiling path=api_smoke sequence_duration=%s hard_ceiling=%s command_readiness_p95=%s",
		smokeDuration,
		30*time.Second,
		apiSmokePercentileDuration(commandReadinessDurations, 0.95),
	)
}

func measureAPISmokeDurations(t *testing.T, samples int, run func(*testing.T)) []time.Duration {
	t.Helper()
	durations := make([]time.Duration, 0, samples)
	for range samples {
		startedAt := time.Now()
		run(t)
		durations = append(durations, time.Since(startedAt))
	}
	return durations
}

func assertAPISmokeCeiling(t *testing.T, path string, durations []time.Duration, p95Ceiling time.Duration, hardCeiling time.Duration) {
	t.Helper()
	p95 := apiSmokePercentileDuration(durations, 0.95)
	hardMax := apiSmokeMaxDuration(durations)
	if p95 > p95Ceiling {
		t.Fatalf("%s p95 = %s, ceiling %s", path, p95, p95Ceiling)
	}
	if hardMax > hardCeiling {
		t.Fatalf("%s hard max = %s, ceiling %s", path, hardMax, hardCeiling)
	}
	t.Logf("scale_ceiling path=%s samples=%d p95=%s hard_max=%s p95_ceiling=%s hard_ceiling=%s", path, len(durations), p95, hardMax, p95Ceiling, hardCeiling)
}

func apiSmokePercentileDuration(values []time.Duration, percentile float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	index := int(float64(len(sorted))*percentile + 0.5)
	if index < 1 {
		index = 1
	}
	if index > len(sorted) {
		index = len(sorted)
	}
	return sorted[index-1]
}

func apiSmokeMaxDuration(values []time.Duration) time.Duration {
	var maxValue time.Duration
	for _, value := range values {
		if value > maxValue {
			maxValue = value
		}
	}
	return maxValue
}
