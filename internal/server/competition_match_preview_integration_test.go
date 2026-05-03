package server

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/ares"
	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/competition"
	"github.com/ixxet/apollo/internal/rating"
)

func TestCompetitionCommandEndpointGeneratesARESMatchPreviewFromTrustedFacts(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-ares-preview-001", "ares-preview-001@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)
	memberCookie, member := createVerifiedSessionViaHTTP(t, env, "student-ares-preview-002", "ares-preview-002@example.com")

	for _, cookie := range []*http.Cookie{ownerCookie, memberCookie} {
		makeEligibleForLobby(t, env, cookie)
		joinLobbyMembership(t, env, cookie)
	}

	ratingSeed := createStartedCompetitionSession(t, env, ownerCookie, "ARES v2 Rating Seed", "badminton", "gym-floor", 1, []uuid.UUID{owner.ID, member.ID})
	completedSeed := recordCompetitionResult(t, env, ownerCookie, ratingSeed.ID.String(), ratingSeed.Matches[0].ID.String(), ratingSeed.Matches[0].SideSlots, []string{"win", "loss"})
	assertLegacyRatingProjection(t, env, owner.ID, "badminton", "head_to_head:s2-p1", completedSeed.Matches[0].Result.ID)
	assertOpenSkillComparison(t, env, owner.ID, "badminton", "head_to_head:s2-p1", completedSeed.Matches[0].Result.ID, false)

	createSessionResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/sessions", `{
		"display_name":"ARES v2 Preview Session",
		"sport_key":"badminton",
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"participants_per_side":1
	}`, ownerCookie)
	if createSessionResponse.Code != http.StatusCreated {
		t.Fatalf("createSessionResponse.Code = %d, want %d body=%s", createSessionResponse.Code, http.StatusCreated, createSessionResponse.Body.String())
	}
	session := decodeCompetitionSession(t, createSessionResponse)
	session = openCompetitionQueue(t, env, ownerCookie, session.ID.String())
	session = queueCompetitionMemberWithTier(t, env, ownerCookie, session.ID.String(), owner.ID, "competitive")
	session = queueCompetitionMemberWithTier(t, env, ownerCookie, session.ID.String(), member.ID, "open")

	updateOutcome := executeCompetitionCommand(t, env, ownerCookie, fmt.Sprintf(`{
		"name":"update_queue_intent",
		"session_id":"%s",
		"expected_version":%d,
		"queue_member":{"user_id":"%s","tier":"competitive"}
	}`, session.ID, session.QueueVersion, member.ID), http.StatusOK)
	if updateOutcome.Status != competition.CommandStatusSucceeded || !updateOutcome.Mutated {
		t.Fatalf("updateOutcome = %#v, want succeeded mutating queue intent command", updateOutcome)
	}
	if updateOutcome.ActualVersion == nil || *updateOutcome.ActualVersion != session.QueueVersion+1 {
		t.Fatalf("updateOutcome.ActualVersion = %v, want %d", updateOutcome.ActualVersion, session.QueueVersion+1)
	}
	currentQueueVersion := *updateOutcome.ActualVersion

	assertQueueIntentFacts(t, env, session.ID, owner.ID, "competitive")
	assertQueueIntentFacts(t, env, session.ID, member.ID, "competitive")
	assertQueueIntentEventCount(t, env, session.ID, 3)

	deniedOutcome := executeCompetitionCommand(t, env, memberCookie, fmt.Sprintf(`{
		"name":"generate_match_preview",
		"session_id":"%s",
		"expected_version":%d
	}`, session.ID, currentQueueVersion), http.StatusForbidden)
	if deniedOutcome.Status != competition.CommandStatusDenied {
		t.Fatalf("deniedOutcome.Status = %q, want %q", deniedOutcome.Status, competition.CommandStatusDenied)
	}

	staleOutcome := executeCompetitionCommand(t, env, ownerCookie, fmt.Sprintf(`{
		"name":"generate_match_preview",
		"session_id":"%s",
		"expected_version":%d
	}`, session.ID, currentQueueVersion-1), http.StatusConflict)
	if staleOutcome.Status != competition.CommandStatusRejected || staleOutcome.Error != competition.ErrQueueStateStale.Error() {
		t.Fatalf("staleOutcome = %#v, want stale queue rejection", staleOutcome)
	}

	beforeMatches := countTableRows(t, env, "apollo.competition_matches")
	beforeResults := countTableRows(t, env, "apollo.competition_match_results")
	beforeRatings := countTableRows(t, env, "apollo.competition_member_ratings")
	beforeComparisons := countTableRows(t, env, "apollo.competition_rating_comparisons")
	beforePreviews := countTableRows(t, env, "apollo.competition_match_previews")
	beforePreviewMembers := countTableRows(t, env, "apollo.competition_match_preview_members")
	beforePreviewEvents := countTableRows(t, env, "apollo.competition_match_preview_events")

	firstOutcome := executeCompetitionCommand(t, env, ownerCookie, fmt.Sprintf(`{
		"name":"generate_match_preview",
		"session_id":"%s",
		"expected_version":%d
	}`, session.ID, currentQueueVersion), http.StatusOK)
	if firstOutcome.Status != competition.CommandStatusSucceeded || !firstOutcome.Mutated {
		t.Fatalf("firstOutcome = %#v, want succeeded mutating preview command", firstOutcome)
	}
	if firstOutcome.ActualVersion == nil || *firstOutcome.ActualVersion != currentQueueVersion {
		t.Fatalf("firstOutcome.ActualVersion = %v, want %d", firstOutcome.ActualVersion, currentQueueVersion)
	}
	firstPreview := commandResultMap(t, firstOutcome)
	assertARESPreviewResult(t, firstPreview, session.ID, currentQueueVersion)
	assertStoredARESPreview(t, env, session.ID, currentQueueVersion)

	if afterMatches := countTableRows(t, env, "apollo.competition_matches"); afterMatches != beforeMatches {
		t.Fatalf("competition_matches changed from %d to %d during preview generation", beforeMatches, afterMatches)
	}
	if afterResults := countTableRows(t, env, "apollo.competition_match_results"); afterResults != beforeResults {
		t.Fatalf("competition_match_results changed from %d to %d during preview generation", beforeResults, afterResults)
	}
	if afterRatings := countTableRows(t, env, "apollo.competition_member_ratings"); afterRatings != beforeRatings {
		t.Fatalf("competition_member_ratings changed from %d to %d during preview generation", beforeRatings, afterRatings)
	}
	if afterComparisons := countTableRows(t, env, "apollo.competition_rating_comparisons"); afterComparisons != beforeComparisons {
		t.Fatalf("competition_rating_comparisons changed from %d to %d during preview generation", beforeComparisons, afterComparisons)
	}
	if afterPreviews := countTableRows(t, env, "apollo.competition_match_previews"); afterPreviews != beforePreviews+1 {
		t.Fatalf("competition_match_previews = %d, want %d", afterPreviews, beforePreviews+1)
	}
	if afterPreviewMembers := countTableRows(t, env, "apollo.competition_match_preview_members"); afterPreviewMembers != beforePreviewMembers+2 {
		t.Fatalf("competition_match_preview_members = %d, want %d", afterPreviewMembers, beforePreviewMembers+2)
	}
	if afterPreviewEvents := countTableRows(t, env, "apollo.competition_match_preview_events"); afterPreviewEvents != beforePreviewEvents+1 {
		t.Fatalf("competition_match_preview_events = %d, want %d", afterPreviewEvents, beforePreviewEvents+1)
	}

	secondOutcome := executeCompetitionCommand(t, env, ownerCookie, fmt.Sprintf(`{
		"name":"generate_match_preview",
		"session_id":"%s",
		"expected_version":%d
	}`, session.ID, currentQueueVersion), http.StatusOK)
	secondPreview := commandResultMap(t, secondOutcome)
	if !reflect.DeepEqual(firstPreview, secondPreview) {
		t.Fatalf("preview changed between deterministic regenerations\nfirst=%#v\nsecond=%#v", firstPreview, secondPreview)
	}
	if afterPreviews := countTableRows(t, env, "apollo.competition_match_previews"); afterPreviews != beforePreviews+1 {
		t.Fatalf("competition_match_previews after replay = %d, want %d", afterPreviews, beforePreviews+1)
	}
	if afterPreviewMembers := countTableRows(t, env, "apollo.competition_match_preview_members"); afterPreviewMembers != beforePreviewMembers+2 {
		t.Fatalf("competition_match_preview_members after replay = %d, want %d", afterPreviewMembers, beforePreviewMembers+2)
	}
	if afterPreviewEvents := countTableRows(t, env, "apollo.competition_match_preview_events"); afterPreviewEvents != beforePreviewEvents+1 {
		t.Fatalf("competition_match_preview_events after replay = %d, want %d", afterPreviewEvents, beforePreviewEvents+1)
	}
}

func TestCompetitionQueueIntentRejectsInvalidTier(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	ownerCookie, owner := createVerifiedSessionViaHTTP(t, env, "student-ares-preview-guard-001", "ares-preview-guard-001@example.com")
	setUserRole(t, env, owner.ID, authz.RoleOwner)
	makeEligibleForLobby(t, env, ownerCookie)
	joinLobbyMembership(t, env, ownerCookie)

	createSessionResponse := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/sessions", `{
		"display_name":"ARES v2 Tier Guard",
		"sport_key":"badminton",
		"facility_key":"ashtonbee",
		"zone_key":"gym-floor",
		"participants_per_side":1
	}`, ownerCookie)
	if createSessionResponse.Code != http.StatusCreated {
		t.Fatalf("createSessionResponse.Code = %d, want %d body=%s", createSessionResponse.Code, http.StatusCreated, createSessionResponse.Body.String())
	}
	session := openCompetitionQueue(t, env, ownerCookie, decodeCompetitionSession(t, createSessionResponse).ID.String())

	response := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/queue/members", session.ID), fmt.Sprintf(`{
		"user_id":"%s",
		"tier":"bad tier!"
	}`, owner.ID), ownerCookie)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("response.Code = %d, want %d body=%s", response.Code, http.StatusBadRequest, response.Body.String())
	}

	var intentCount int
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT count(*)
FROM apollo.competition_queue_intents
WHERE competition_session_id = $1
`, session.ID).Scan(&intentCount); err != nil {
		t.Fatalf("count queue intents error = %v", err)
	}
	if intentCount != 0 {
		t.Fatalf("queue intent count = %d, want 0 after invalid tier rejection", intentCount)
	}
}

func queueCompetitionMemberWithTier(t *testing.T, env *authProfileServerEnv, cookie *http.Cookie, sessionID string, userID uuid.UUID, tier string) competition.Session {
	t.Helper()

	response := env.doJSONRequest(t, http.MethodPost, fmt.Sprintf("/api/v1/competition/sessions/%s/queue/members", sessionID), fmt.Sprintf(`{
		"user_id":"%s",
		"tier":%q
	}`, userID, tier), cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("queue member response code = %d, want %d body=%s", response.Code, http.StatusOK, response.Body.String())
	}

	return decodeCompetitionSession(t, response)
}

func executeCompetitionCommand(t *testing.T, env *authProfileServerEnv, cookie *http.Cookie, body string, wantStatus int) competition.CompetitionCommandOutcome {
	t.Helper()

	response := env.doJSONRequest(t, http.MethodPost, "/api/v1/competition/commands", body, cookie)
	if response.Code != wantStatus {
		t.Fatalf("command response code = %d, want %d body=%s", response.Code, wantStatus, response.Body.String())
	}
	return decodeCompetitionCommandOutcome(t, response.Body.Bytes())
}

func commandResultMap(t *testing.T, outcome competition.CompetitionCommandOutcome) map[string]any {
	t.Helper()

	result, ok := outcome.Result.(map[string]any)
	if !ok {
		t.Fatalf("outcome.Result = %#v, want object", outcome.Result)
	}
	return result
}

func assertQueueIntentFacts(t *testing.T, env *authProfileServerEnv, sessionID uuid.UUID, userID uuid.UUID, wantTier string) {
	t.Helper()

	var facilityKey string
	var sportKey string
	var modeKey string
	var tier string
	var status string
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT facility_key, sport_key, mode_key, tier, status
FROM apollo.competition_queue_intents
WHERE competition_session_id = $1
  AND user_id = $2
`, sessionID, userID).Scan(&facilityKey, &sportKey, &modeKey, &tier, &status); err != nil {
		t.Fatalf("read queue intent facts error = %v", err)
	}
	if facilityKey != "ashtonbee" || sportKey != "badminton" || modeKey != "head_to_head:s2-p1" || tier != wantTier || status != "active" {
		t.Fatalf("queue intent facts = %s/%s/%s/%s/%s, want ashtonbee/badminton/head_to_head:s2-p1/%s/active", facilityKey, sportKey, modeKey, tier, status, wantTier)
	}
}

func assertQueueIntentEventCount(t *testing.T, env *authProfileServerEnv, sessionID uuid.UUID, want int) {
	t.Helper()

	var count int
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT count(*)
FROM apollo.competition_queue_intent_events
WHERE competition_session_id = $1
  AND event_type = 'competition.queue_intent.updated'
`, sessionID).Scan(&count); err != nil {
		t.Fatalf("count queue intent events error = %v", err)
	}
	if count != want {
		t.Fatalf("queue intent event count = %d, want %d", count, want)
	}
}

func assertARESPreviewResult(t *testing.T, preview map[string]any, sessionID uuid.UUID, queueVersion int) {
	t.Helper()

	if got := stringField(t, preview, "competition_session_id"); got != sessionID.String() {
		t.Fatalf("competition_session_id = %q, want %s", got, sessionID)
	}
	if got := intField(t, preview, "queue_version"); got != queueVersion {
		t.Fatalf("queue_version = %d, want %d", got, queueVersion)
	}
	if got := stringField(t, preview, "preview_version"); got != ares.CompetitionPreviewVersion {
		t.Fatalf("preview_version = %q, want %q", got, ares.CompetitionPreviewVersion)
	}
	if got := stringField(t, preview, "input_watermark"); got == "" {
		t.Fatal("input_watermark is empty, want deterministic input watermark")
	}
	if _, exists := preview["generated_at"]; exists {
		t.Fatal("generated_at exists on ARES v2 preview, want input_watermark plus event occurred_at")
	}
	if got := stringField(t, preview, "policy_version"); got != ares.CompetitionPreviewPolicy {
		t.Fatalf("policy_version = %q, want %q", got, ares.CompetitionPreviewPolicy)
	}
	if got := stringField(t, preview, "rating_engine"); got != rating.EngineLegacyEloLike {
		t.Fatalf("rating_engine = %q, want %q", got, rating.EngineLegacyEloLike)
	}
	if got := stringField(t, preview, "rating_policy_version"); got != rating.PolicyVersionActive {
		t.Fatalf("rating_policy_version = %q, want %q", got, rating.PolicyVersionActive)
	}
	if got := stringField(t, preview, "active_rating_read_path"); got != rating.PolicyVersionActive {
		t.Fatalf("active_rating_read_path = %q, want %q", got, rating.PolicyVersionActive)
	}
	if got := stringField(t, preview, "openskill_comparison_policy"); got != rating.PolicyVersionOpenSkill {
		t.Fatalf("openskill_comparison_policy = %q, want %q", got, rating.PolicyVersionOpenSkill)
	}
	if got := stringField(t, preview, "facility_key"); got != "ashtonbee" {
		t.Fatalf("facility_key = %q, want ashtonbee", got)
	}
	if got := stringField(t, preview, "sport_key"); got != "badminton" {
		t.Fatalf("sport_key = %q, want badminton", got)
	}
	if got := stringField(t, preview, "mode_key"); got != "head_to_head:s2-p1" {
		t.Fatalf("mode_key = %q, want head_to_head:s2-p1", got)
	}
	if got := stringField(t, preview, "tier"); got != "competitive" {
		t.Fatalf("tier = %q, want competitive", got)
	}
	if got := intField(t, preview, "candidate_count"); got != 2 {
		t.Fatalf("candidate_count = %d, want 2", got)
	}
	if got := intField(t, preview, "missing_rating_count"); got != 0 {
		t.Fatalf("missing_rating_count = %d, want 0", got)
	}
	matchQuality := floatField(t, preview, "match_quality")
	if matchQuality <= 0 || matchQuality > 1 {
		t.Fatalf("match_quality = %.4f, want within (0,1]", matchQuality)
	}
	winProbability := floatField(t, preview, "predicted_win_probability")
	if winProbability <= 0.5 || winProbability > 1 {
		t.Fatalf("predicted_win_probability = %.4f, want legacy favorite probability", winProbability)
	}

	explanationCode := stringField(t, preview, "explanation_code")
	allowedCodes := []string{
		ares.ExplanationBalancedLegacyRating,
		ares.ExplanationAcceptableLegacyRatingGap,
		ares.ExplanationWideLegacyRatingGap,
		ares.ExplanationProvisionalRatingsUsed,
		ares.ExplanationMixedTierPreview,
	}
	if !slices.Contains(allowedCodes, explanationCode) {
		t.Fatalf("explanation_code = %q, want explicit ARES code", explanationCode)
	}
	rawCodes, ok := preview["explanation_codes"].([]any)
	if !ok || len(rawCodes) == 0 {
		t.Fatalf("explanation_codes = %#v, want non-empty list", preview["explanation_codes"])
	}
	foundPrimary := false
	for _, raw := range rawCodes {
		if code, ok := raw.(string); ok && code == explanationCode {
			foundPrimary = true
			break
		}
	}
	if !foundPrimary {
		t.Fatalf("explanation_codes = %#v, want primary code %q", rawCodes, explanationCode)
	}

	rawSides, ok := preview["sides"].([]any)
	if !ok || len(rawSides) != 2 {
		t.Fatalf("sides = %#v, want two preview sides", preview["sides"])
	}
	for _, rawSide := range rawSides {
		side, ok := rawSide.(map[string]any)
		if !ok {
			t.Fatalf("side = %#v, want object", rawSide)
		}
		rawMembers, ok := side["members"].([]any)
		if !ok || len(rawMembers) != 1 {
			t.Fatalf("side members = %#v, want one member", side["members"])
		}
		member, ok := rawMembers[0].(map[string]any)
		if !ok {
			t.Fatalf("member = %#v, want object", rawMembers[0])
		}
		if got := stringField(t, member, "rating_source"); got != ares.RatingSourceLegacyProjection {
			t.Fatalf("rating_source = %q, want %q", got, ares.RatingSourceLegacyProjection)
		}
	}
}

func assertStoredARESPreview(t *testing.T, env *authProfileServerEnv, sessionID uuid.UUID, queueVersion int) {
	t.Helper()

	var previewID uuid.UUID
	var previewVersion string
	var policyVersion string
	var ratingEngine string
	var ratingPolicyVersion string
	var explanationCode string
	var eventType string
	var matchQuality float64
	var winProbability float64
	var inputWatermark time.Time
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT p.id,
       p.preview_version,
       p.policy_version,
       p.rating_engine,
       p.rating_policy_version,
       p.explanation_code,
       p.match_quality::double precision,
       p.predicted_win_probability::double precision,
       p.input_watermark,
       e.event_type
FROM apollo.competition_match_previews AS p
INNER JOIN apollo.competition_match_preview_events AS e
  ON e.competition_match_preview_id = p.id
WHERE p.competition_session_id = $1
  AND p.queue_version = $2
`, sessionID, queueVersion).Scan(&previewID, &previewVersion, &policyVersion, &ratingEngine, &ratingPolicyVersion, &explanationCode, &matchQuality, &winProbability, &inputWatermark, &eventType); err != nil {
		t.Fatalf("read stored ARES preview error = %v", err)
	}
	if previewID == uuid.Nil {
		t.Fatal("previewID is nil, want stored preview id")
	}
	if previewVersion != ares.CompetitionPreviewVersion || policyVersion != ares.CompetitionPreviewPolicy {
		t.Fatalf("stored preview version/policy = %s/%s", previewVersion, policyVersion)
	}
	if ratingEngine != rating.EngineLegacyEloLike || ratingPolicyVersion != rating.PolicyVersionActive {
		t.Fatalf("stored rating policy = %s/%s, want active wrapper", ratingEngine, ratingPolicyVersion)
	}
	if explanationCode == "" {
		t.Fatal("stored explanation_code is empty")
	}
	if matchQuality <= 0 || matchQuality > 1 || winProbability <= 0 || winProbability > 1 {
		t.Fatalf("stored quality/probability = %.4f/%.4f, want bounded values", matchQuality, winProbability)
	}
	if inputWatermark.IsZero() {
		t.Fatal("stored input_watermark is zero")
	}
	if eventType != "competition.match_preview.generated" {
		t.Fatalf("event_type = %q, want competition.match_preview.generated", eventType)
	}

	var memberCount int
	var legacySourceCount int
	if err := env.db.DB.QueryRow(context.Background(), `
SELECT count(*),
       count(*) FILTER (WHERE rating_source = 'legacy_projection')
FROM apollo.competition_match_preview_members
WHERE competition_match_preview_id = $1
`, previewID).Scan(&memberCount, &legacySourceCount); err != nil {
		t.Fatalf("count preview members error = %v", err)
	}
	if memberCount != 2 || legacySourceCount != 2 {
		t.Fatalf("preview member counts = %d/%d, want 2/2 legacy projection members", memberCount, legacySourceCount)
	}
}

func stringField(t *testing.T, payload map[string]any, key string) string {
	t.Helper()

	value, ok := payload[key].(string)
	if !ok {
		t.Fatalf("%s = %#v, want string", key, payload[key])
	}
	return value
}

func intField(t *testing.T, payload map[string]any, key string) int {
	t.Helper()

	value, ok := payload[key].(float64)
	if !ok {
		t.Fatalf("%s = %#v, want number", key, payload[key])
	}
	return int(value)
}

func floatField(t *testing.T, payload map[string]any, key string) float64 {
	t.Helper()

	value, ok := payload[key].(float64)
	if !ok {
		t.Fatalf("%s = %#v, want number", key, payload[key])
	}
	return value
}
