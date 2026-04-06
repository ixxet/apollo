package ares

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

type stubCandidateSource struct {
	rows []JoinedLobbyCandidate
	err  error
}

func (s stubCandidateSource) ListJoinedLobbyMatchPreviewCandidates(context.Context) ([]JoinedLobbyCandidate, error) {
	if s.err != nil {
		return nil, s.err
	}

	return slices.Clone(s.rows), nil
}

func TestGetLobbyMatchPreviewReturnsRepositoryErrors(t *testing.T) {
	svc := NewService(stubCandidateSource{err: errors.New("boom")})

	if _, err := svc.GetLobbyMatchPreview(context.Background()); err == nil || err.Error() != "boom" {
		t.Fatalf("GetLobbyMatchPreview() error = %v, want boom", err)
	}
}

func TestBuildPreviewReturnsEmptyPreviewCleanly(t *testing.T) {
	preview := buildPreview(nil)

	if preview.GeneratedAt != nil {
		t.Fatalf("preview.GeneratedAt = %#v, want nil", preview.GeneratedAt)
	}
	if preview.CandidateCount != 0 {
		t.Fatalf("preview.CandidateCount = %d, want 0", preview.CandidateCount)
	}
	if preview.PreviewVersion != PreviewVersion {
		t.Fatalf("preview.PreviewVersion = %q, want %q", preview.PreviewVersion, PreviewVersion)
	}
	if len(preview.Matches) != 0 {
		t.Fatalf("len(preview.Matches) = %d, want 0", len(preview.Matches))
	}
	if len(preview.UnmatchedMemberIDs) != 0 {
		t.Fatalf("len(preview.UnmatchedMemberIDs) = %d, want 0", len(preview.UnmatchedMemberIDs))
	}
}

func TestBuildPreviewHandlesSingleCandidateExplicitly(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	preview := buildPreview([]JoinedLobbyCandidate{joinedCandidate(userID, time.Date(2026, 4, 6, 11, 0, 0, 0, time.UTC))})

	if preview.CandidateCount != 1 {
		t.Fatalf("preview.CandidateCount = %d, want 1", preview.CandidateCount)
	}
	if len(preview.Matches) != 0 {
		t.Fatalf("len(preview.Matches) = %d, want 0", len(preview.Matches))
	}
	if got, want := preview.UnmatchedMemberIDs, []uuid.UUID{userID}; !slices.Equal(got, want) {
		t.Fatalf("preview.UnmatchedMemberIDs = %v, want %v", got, want)
	}
	if got, want := preview.UnmatchedLabels, []string{"11111111"}; !slices.Equal(got, want) {
		t.Fatalf("preview.UnmatchedLabels = %v, want %v", got, want)
	}
}

func TestBuildPreviewReturnsDeterministicPairingsAndOrdering(t *testing.T) {
	rows := []JoinedLobbyCandidate{
		joinedCandidate(uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd"), time.Date(2026, 4, 6, 11, 0, 0, 0, time.UTC)),
		joinedCandidate(uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"), time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)),
		joinedCandidate(uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), time.Date(2026, 4, 6, 13, 0, 0, 0, time.UTC)),
		joinedCandidate(uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"), time.Date(2026, 4, 6, 14, 0, 0, 0, time.UTC)),
	}

	preview := buildPreview(rows)

	if preview.CandidateCount != 4 {
		t.Fatalf("preview.CandidateCount = %d, want 4", preview.CandidateCount)
	}
	if len(preview.Matches) != 2 {
		t.Fatalf("len(preview.Matches) = %d, want 2", len(preview.Matches))
	}

	first := preview.Matches[0]
	second := preview.Matches[1]
	assertMatchMembers(t, first, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	assertMatchMembers(t, second, "cccccccc-cccc-cccc-cccc-cccccccccccc", "dddddddd-dddd-dddd-dddd-dddddddddddd")
	assertDeterministicReasons(t, first.Reasons)
	assertDeterministicReasons(t, second.Reasons)
	if preview.GeneratedAt == nil || !preview.GeneratedAt.Equal(time.Date(2026, 4, 6, 14, 0, 0, 0, time.UTC)) {
		t.Fatalf("preview.GeneratedAt = %#v, want latest input timestamp", preview.GeneratedAt)
	}
}

func TestBuildPreviewExcludesIneligibleJoinedMembersPerPolicy(t *testing.T) {
	rows := []JoinedLobbyCandidate{
		joinedCandidate(uuid.MustParse("11111111-1111-1111-1111-111111111111"), time.Date(2026, 4, 6, 11, 0, 0, 0, time.UTC)),
		joinedCandidateWithPreferences(
			uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
			`{"visibility_mode":"ghost","availability_mode":"available_now"}`,
		),
		joinedCandidate(uuid.MustParse("33333333-3333-3333-3333-333333333333"), time.Date(2026, 4, 6, 13, 0, 0, 0, time.UTC)),
	}

	preview := buildPreview(rows)

	if preview.CandidateCount != 2 {
		t.Fatalf("preview.CandidateCount = %d, want 2", preview.CandidateCount)
	}
	if len(preview.Matches) != 1 {
		t.Fatalf("len(preview.Matches) = %d, want 1", len(preview.Matches))
	}
	assertMatchMembers(t, preview.Matches[0], "11111111-1111-1111-1111-111111111111", "33333333-3333-3333-3333-333333333333")
	if len(preview.UnmatchedMemberIDs) != 0 {
		t.Fatalf("len(preview.UnmatchedMemberIDs) = %d, want 0", len(preview.UnmatchedMemberIDs))
	}
}

func TestBuildPreviewIgnoresIneligibleJoinedMembersWhenComputingGeneratedAt(t *testing.T) {
	rows := []JoinedLobbyCandidate{
		joinedCandidate(uuid.MustParse("11111111-1111-1111-1111-111111111111"), time.Date(2026, 4, 6, 11, 0, 0, 0, time.UTC)),
		joinedCandidate(uuid.MustParse("33333333-3333-3333-3333-333333333333"), time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)),
		joinedCandidateWithPreferences(
			uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			time.Date(2026, 4, 6, 14, 0, 0, 0, time.UTC),
			`{"visibility_mode":"ghost","availability_mode":"available_now"}`,
		),
	}

	preview := buildPreview(rows)

	if preview.GeneratedAt == nil || !preview.GeneratedAt.Equal(time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("preview.GeneratedAt = %#v, want latest eligible candidate timestamp", preview.GeneratedAt)
	}
	assertMatchMembers(t, preview.Matches[0], "11111111-1111-1111-1111-111111111111", "33333333-3333-3333-3333-333333333333")
}

func TestBuildPreviewResolvesTieSituationsDeterministicallyByUserID(t *testing.T) {
	rows := []JoinedLobbyCandidate{
		joinedCandidate(uuid.MustParse("99999999-9999-9999-9999-999999999999"), time.Date(2026, 4, 6, 11, 0, 0, 0, time.UTC)),
		joinedCandidate(uuid.MustParse("11111111-1111-1111-1111-111111111111"), time.Date(2026, 4, 6, 11, 0, 0, 0, time.UTC)),
		joinedCandidate(uuid.MustParse("55555555-5555-5555-5555-555555555555"), time.Date(2026, 4, 6, 11, 0, 0, 0, time.UTC)),
	}

	first := buildPreview(rows)
	reversed := slices.Clone(rows)
	slices.Reverse(reversed)
	second := buildPreview(reversed)

	if first.CandidateCount != 3 {
		t.Fatalf("first.CandidateCount = %d, want 3", first.CandidateCount)
	}
	assertMatchMembers(t, first.Matches[0], "11111111-1111-1111-1111-111111111111", "55555555-5555-5555-5555-555555555555")
	if got, want := first.UnmatchedMemberIDs, []uuid.UUID{uuid.MustParse("99999999-9999-9999-9999-999999999999")}; !slices.Equal(got, want) {
		t.Fatalf("first.UnmatchedMemberIDs = %v, want %v", got, want)
	}
	if !slices.EqualFunc(first.Matches, second.Matches, func(left, right Match) bool {
		return slices.Equal(left.MemberIDs, right.MemberIDs) &&
			slices.Equal(left.MemberLabels, right.MemberLabels) &&
			left.Score == right.Score &&
			slices.Equal(left.Reasons, right.Reasons)
	}) {
		t.Fatalf("preview matches changed between repeated runs: first=%#v second=%#v", first.Matches, second.Matches)
	}
}

func joinedCandidate(userID uuid.UUID, updatedAt time.Time) JoinedLobbyCandidate {
	return joinedCandidateWithPreferences(userID, updatedAt, `{"visibility_mode":"discoverable","availability_mode":"available_now"}`)
}

func joinedCandidateWithPreferences(userID uuid.UUID, updatedAt time.Time, preferences string) JoinedLobbyCandidate {
	return JoinedLobbyCandidate{
		UserID:              userID,
		Preferences:         []byte(preferences),
		UserUpdatedAt:       updatedAt.UTC(),
		JoinedAt:            updatedAt.UTC(),
		MembershipUpdatedAt: updatedAt.UTC(),
	}
}

func assertMatchMembers(t *testing.T, match Match, left string, right string) {
	t.Helper()

	want := []uuid.UUID{uuid.MustParse(left), uuid.MustParse(right)}
	if !slices.Equal(match.MemberIDs, want) {
		t.Fatalf("match.MemberIDs = %v, want %v", match.MemberIDs, want)
	}
	if match.Score != 2 {
		t.Fatalf("match.Score = %d, want 2", match.Score)
	}
}

func assertDeterministicReasons(t *testing.T, reasons []Reason) {
	t.Helper()

	want := []Reason{
		{Code: "explicit_joined_membership"},
		{Code: "compatible_visibility_mode", Value: "discoverable"},
		{Code: "compatible_availability_mode", Value: "available_now"},
		{Code: "stable_pair_order", Value: "user_id_asc"},
	}
	if !slices.Equal(reasons, want) {
		t.Fatalf("reasons = %#v, want %#v", reasons, want)
	}
}
