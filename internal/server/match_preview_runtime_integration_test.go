package server

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/ares"
)

func TestLobbyMatchPreviewRuntimeUsesOnlyExplicitJoinedEligibleMembersDeterministically(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	viewerCookie, viewer := createVerifiedSessionViaHTTP(t, env, "student-preview-001", "preview-001@example.com")
	memberTwoCookie, memberTwo := createVerifiedSessionViaHTTP(t, env, "student-preview-002", "preview-002@example.com")
	memberThreeCookie, memberThree := createVerifiedSessionViaHTTP(t, env, "student-preview-003", "preview-003@example.com")
	memberFourCookie, memberFour := createVerifiedSessionViaHTTP(t, env, "student-preview-004", "preview-004@example.com")
	ineligibleJoinedCookie, ineligibleJoined := createVerifiedSessionViaHTTP(t, env, "student-preview-005", "preview-005@example.com")
	notJoinedCookie, notJoined := createVerifiedSessionViaHTTP(t, env, "student-preview-006", "preview-006@example.com")

	for _, cookie := range []*http.Cookie{viewerCookie, memberTwoCookie, memberThreeCookie, memberFourCookie, ineligibleJoinedCookie, notJoinedCookie} {
		makeEligibleForLobby(t, env, cookie)
	}
	for _, cookie := range []*http.Cookie{viewerCookie, memberTwoCookie, memberThreeCookie, memberFourCookie, ineligibleJoinedCookie} {
		joinLobbyMembership(t, env, cookie)
	}

	patchIneligibleResponse := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{"visibility_mode":"ghost"}`, ineligibleJoinedCookie)
	if patchIneligibleResponse.Code != http.StatusOK {
		t.Fatalf("patchIneligibleResponse.Code = %d, want %d", patchIneligibleResponse.Code, http.StatusOK)
	}

	firstResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/match-preview", nil, viewerCookie)
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("firstResponse.Code = %d, want %d", firstResponse.Code, http.StatusOK)
	}
	secondResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/match-preview", nil, viewerCookie)
	if secondResponse.Code != http.StatusOK {
		t.Fatalf("secondResponse.Code = %d, want %d", secondResponse.Code, http.StatusOK)
	}

	firstPreview := decodeMatchPreviewResponse(t, firstResponse.Body.Bytes())
	secondPreview := decodeMatchPreviewResponse(t, secondResponse.Body.Bytes())

	if firstPreview.CandidateCount != 4 {
		t.Fatalf("firstPreview.CandidateCount = %d, want 4", firstPreview.CandidateCount)
	}
	if firstPreview.PreviewVersion != ares.PreviewVersion {
		t.Fatalf("firstPreview.PreviewVersion = %q, want %q", firstPreview.PreviewVersion, ares.PreviewVersion)
	}
	if firstPreview.GeneratedAt == nil {
		t.Fatal("firstPreview.GeneratedAt = nil, want deterministic snapshot timestamp")
	}
	if len(firstPreview.Matches) != 2 {
		t.Fatalf("len(firstPreview.Matches) = %d, want 2", len(firstPreview.Matches))
	}
	if len(firstPreview.UnmatchedMemberIDs) != 0 {
		t.Fatalf("len(firstPreview.UnmatchedMemberIDs) = %d, want 0", len(firstPreview.UnmatchedMemberIDs))
	}

	expectedCandidates := []uuid.UUID{viewer.ID, memberTwo.ID, memberThree.ID, memberFour.ID}
	slices.SortFunc(expectedCandidates, compareUUID)
	gotCandidates := previewMemberIDs(firstPreview)
	if !slices.Equal(gotCandidates, expectedCandidates) {
		t.Fatalf("previewMemberIDs(firstPreview) = %v, want %v", gotCandidates, expectedCandidates)
	}
	if slices.Contains(gotCandidates, ineligibleJoined.ID) {
		t.Fatalf("preview included ineligible joined member %s", ineligibleJoined.ID)
	}
	if slices.Contains(gotCandidates, notJoined.ID) {
		t.Fatalf("preview included non-joined member %s", notJoined.ID)
	}
	for _, match := range firstPreview.Matches {
		if len(match.MemberIDs) != 2 {
			t.Fatalf("match.MemberIDs = %v, want pair", match.MemberIDs)
		}
		if !slices.IsSortedFunc(match.MemberIDs, compareUUID) {
			t.Fatalf("match.MemberIDs = %v, want user-id asc order", match.MemberIDs)
		}
		if match.Score != 2 {
			t.Fatalf("match.Score = %d, want 2", match.Score)
		}
	}
	if !slices.EqualFunc(firstPreview.Matches, secondPreview.Matches, func(left, right ares.Match) bool {
		return slices.Equal(left.MemberIDs, right.MemberIDs) &&
			slices.Equal(left.MemberLabels, right.MemberLabels) &&
			left.Score == right.Score &&
			slices.Equal(left.Reasons, right.Reasons)
	}) {
		t.Fatalf("preview matches changed between repeated reads: first=%#v second=%#v", firstPreview.Matches, secondPreview.Matches)
	}
	if !equalOptionalTime(firstPreview.GeneratedAt, secondPreview.GeneratedAt) {
		t.Fatalf("preview generated_at changed between repeated reads: first=%v second=%v", firstPreview.GeneratedAt, secondPreview.GeneratedAt)
	}
	if string(firstResponse.Body.Bytes()) != string(secondResponse.Body.Bytes()) {
		t.Fatalf("preview response body changed between repeated reads\nfirst=%s\nsecond=%s", firstResponse.Body.Bytes(), secondResponse.Body.Bytes())
	}
}

func TestLobbyMatchPreviewRuntimeHandlesEmptyAndOddPoolsCleanly(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	viewerCookie, viewer := createVerifiedSessionViaHTTP(t, env, "student-preview-empty-001", "preview-empty-001@example.com")

	emptyResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/match-preview", nil, viewerCookie)
	if emptyResponse.Code != http.StatusOK {
		t.Fatalf("emptyResponse.Code = %d, want %d", emptyResponse.Code, http.StatusOK)
	}
	emptyPreview := decodeMatchPreviewResponse(t, emptyResponse.Body.Bytes())
	if emptyPreview.GeneratedAt != nil {
		t.Fatalf("emptyPreview.GeneratedAt = %#v, want nil", emptyPreview.GeneratedAt)
	}
	if emptyPreview.CandidateCount != 0 {
		t.Fatalf("emptyPreview.CandidateCount = %d, want 0", emptyPreview.CandidateCount)
	}
	if len(emptyPreview.Matches) != 0 {
		t.Fatalf("len(emptyPreview.Matches) = %d, want 0", len(emptyPreview.Matches))
	}

	memberTwoCookie, memberTwo := createVerifiedSessionViaHTTP(t, env, "student-preview-empty-002", "preview-empty-002@example.com")
	memberThreeCookie, memberThree := createVerifiedSessionViaHTTP(t, env, "student-preview-empty-003", "preview-empty-003@example.com")
	for _, cookie := range []*http.Cookie{viewerCookie, memberTwoCookie, memberThreeCookie} {
		makeEligibleForLobby(t, env, cookie)
		joinLobbyMembership(t, env, cookie)
	}

	oddResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/match-preview", nil, viewerCookie)
	if oddResponse.Code != http.StatusOK {
		t.Fatalf("oddResponse.Code = %d, want %d", oddResponse.Code, http.StatusOK)
	}
	oddPreview := decodeMatchPreviewResponse(t, oddResponse.Body.Bytes())
	if oddPreview.CandidateCount != 3 {
		t.Fatalf("oddPreview.CandidateCount = %d, want 3", oddPreview.CandidateCount)
	}
	if len(oddPreview.Matches) != 1 {
		t.Fatalf("len(oddPreview.Matches) = %d, want 1", len(oddPreview.Matches))
	}
	expectedCandidates := []uuid.UUID{viewer.ID, memberTwo.ID, memberThree.ID}
	slices.SortFunc(expectedCandidates, compareUUID)
	if got, want := oddPreview.UnmatchedMemberIDs, expectedCandidates[2:]; !slices.Equal(got, want) {
		t.Fatalf("oddPreview.UnmatchedMemberIDs = %v, want %v", got, want)
	}
}

func TestLobbyMatchPreviewRuntimeIgnoresIneligibleJoinedWatermarkChanges(t *testing.T) {
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)

	viewerCookie, viewer := createVerifiedSessionViaHTTP(t, env, "student-preview-watermark-001", "preview-watermark-001@example.com")
	memberTwoCookie, memberTwo := createVerifiedSessionViaHTTP(t, env, "student-preview-watermark-002", "preview-watermark-002@example.com")
	ineligibleJoinedCookie, ineligibleJoined := createVerifiedSessionViaHTTP(t, env, "student-preview-watermark-003", "preview-watermark-003@example.com")

	for _, cookie := range []*http.Cookie{viewerCookie, memberTwoCookie, ineligibleJoinedCookie} {
		makeEligibleForLobby(t, env, cookie)
		joinLobbyMembership(t, env, cookie)
	}

	firstIneligiblePatch := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{"visibility_mode":"ghost"}`, ineligibleJoinedCookie)
	if firstIneligiblePatch.Code != http.StatusOK {
		t.Fatalf("firstIneligiblePatch.Code = %d, want %d", firstIneligiblePatch.Code, http.StatusOK)
	}

	firstResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/match-preview", nil, viewerCookie)
	if firstResponse.Code != http.StatusOK {
		t.Fatalf("firstResponse.Code = %d, want %d", firstResponse.Code, http.StatusOK)
	}
	firstPreview := decodeMatchPreviewResponse(t, firstResponse.Body.Bytes())
	if firstPreview.GeneratedAt == nil {
		t.Fatal("firstPreview.GeneratedAt = nil, want eligible candidate watermark")
	}

	expectedCandidates := []uuid.UUID{viewer.ID, memberTwo.ID}
	slices.SortFunc(expectedCandidates, compareUUID)
	if got := previewMemberIDs(firstPreview); !slices.Equal(got, expectedCandidates) {
		t.Fatalf("previewMemberIDs(firstPreview) = %v, want %v", got, expectedCandidates)
	}
	if slices.Contains(previewMemberIDs(firstPreview), ineligibleJoined.ID) {
		t.Fatalf("preview included ineligible joined member %s", ineligibleJoined.ID)
	}

	secondIneligiblePatch := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{"visibility_mode":"ghost","availability_mode":"unavailable"}`, ineligibleJoinedCookie)
	if secondIneligiblePatch.Code != http.StatusOK {
		t.Fatalf("secondIneligiblePatch.Code = %d, want %d", secondIneligiblePatch.Code, http.StatusOK)
	}

	secondResponse := env.doRequest(t, http.MethodGet, "/api/v1/lobby/match-preview", nil, viewerCookie)
	if secondResponse.Code != http.StatusOK {
		t.Fatalf("secondResponse.Code = %d, want %d", secondResponse.Code, http.StatusOK)
	}
	secondPreview := decodeMatchPreviewResponse(t, secondResponse.Body.Bytes())

	if !slices.EqualFunc(firstPreview.Matches, secondPreview.Matches, func(left, right ares.Match) bool {
		return slices.Equal(left.MemberIDs, right.MemberIDs) &&
			slices.Equal(left.MemberLabels, right.MemberLabels) &&
			left.Score == right.Score &&
			slices.Equal(left.Reasons, right.Reasons)
	}) {
		t.Fatalf("preview matches changed after ineligible joined watermark update: first=%#v second=%#v", firstPreview.Matches, secondPreview.Matches)
	}
	if !slices.Equal(firstPreview.UnmatchedMemberIDs, secondPreview.UnmatchedMemberIDs) {
		t.Fatalf("preview unmatched ids changed after ineligible joined watermark update: first=%v second=%v", firstPreview.UnmatchedMemberIDs, secondPreview.UnmatchedMemberIDs)
	}
	if !slices.Equal(firstPreview.UnmatchedLabels, secondPreview.UnmatchedLabels) {
		t.Fatalf("preview unmatched labels changed after ineligible joined watermark update: first=%v second=%v", firstPreview.UnmatchedLabels, secondPreview.UnmatchedLabels)
	}
	if !equalOptionalTime(firstPreview.GeneratedAt, secondPreview.GeneratedAt) {
		t.Fatalf("preview generated_at changed after ineligible joined watermark update: first=%v second=%v", firstPreview.GeneratedAt, secondPreview.GeneratedAt)
	}
	if string(firstResponse.Body.Bytes()) != string(secondResponse.Body.Bytes()) {
		t.Fatalf("preview response body changed after ineligible joined watermark update\nfirst=%s\nsecond=%s", firstResponse.Body.Bytes(), secondResponse.Body.Bytes())
	}
}

func makeEligibleForLobby(t *testing.T, env *authProfileServerEnv, cookie *http.Cookie) {
	t.Helper()

	response := env.doJSONRequest(t, http.MethodPatch, "/api/v1/profile", `{"visibility_mode":"discoverable","availability_mode":"available_now"}`, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("eligibility patch status = %d, want %d", response.Code, http.StatusOK)
	}
}

func joinLobbyMembership(t *testing.T, env *authProfileServerEnv, cookie *http.Cookie) {
	t.Helper()

	response := env.doRequest(t, http.MethodPost, "/api/v1/lobby/membership/join", nil, cookie)
	if response.Code != http.StatusOK {
		t.Fatalf("join status = %d, want %d", response.Code, http.StatusOK)
	}
}

func decodeMatchPreviewResponse(t *testing.T, body []byte) ares.MatchPreview {
	t.Helper()

	var preview ares.MatchPreview
	if err := json.Unmarshal(body, &preview); err != nil {
		t.Fatalf("json.Unmarshal(match preview) error = %v", err)
	}

	return preview
}

func previewMemberIDs(preview ares.MatchPreview) []uuid.UUID {
	memberIDs := make([]uuid.UUID, 0, preview.CandidateCount)
	for _, match := range preview.Matches {
		memberIDs = append(memberIDs, match.MemberIDs...)
	}
	memberIDs = append(memberIDs, preview.UnmatchedMemberIDs...)
	slices.SortFunc(memberIDs, compareUUID)
	return memberIDs
}

func compareUUID(left uuid.UUID, right uuid.UUID) int {
	return slices.Compare(left[:], right[:])
}

func equalOptionalTime(left *time.Time, right *time.Time) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return left.Equal(*right)
	}
}
