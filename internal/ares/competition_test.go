package ares

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBuildCompetitionMatchPreviewIsDeterministicAndExplainable(t *testing.T) {
	input := CompetitionPreviewInput{
		SessionID:           uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		QueueVersion:        5,
		FacilityKey:         "ashtonbee",
		SportKey:            "badminton",
		ModeKey:             "head_to_head:s2-p1",
		SidesPerMatch:       2,
		ParticipantsPerSide: 1,
		Candidates: []CompetitionPreviewCandidate{
			previewCandidate("22222222-2222-2222-2222-222222222222", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", 30, RatingSourceLegacyProjection, "competitive", time.Date(2026, 4, 18, 12, 2, 0, 0, time.UTC)),
			previewCandidate("11111111-1111-1111-1111-111111111111", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", 28.5, RatingSourceLegacyProjection, "competitive", time.Date(2026, 4, 18, 12, 1, 0, 0, time.UTC)),
		},
	}

	first := BuildCompetitionMatchPreview(input)
	reversed := input
	reversed.Candidates = slices.Clone(input.Candidates)
	slices.Reverse(reversed.Candidates)
	second := BuildCompetitionMatchPreview(reversed)

	if first.MatchQuality != 0.9063 {
		t.Fatalf("MatchQuality = %.4f, want 0.9063", first.MatchQuality)
	}
	if first.PredictedWinProbability <= 0.5 {
		t.Fatalf("PredictedWinProbability = %.4f, want side-one favorite", first.PredictedWinProbability)
	}
	if first.ExplanationCode != ExplanationBalancedLegacyRating {
		t.Fatalf("ExplanationCode = %q, want %q", first.ExplanationCode, ExplanationBalancedLegacyRating)
	}
	if first.PolicyVersion != CompetitionPreviewPolicy || first.PreviewVersion != CompetitionPreviewVersion {
		t.Fatalf("preview policy/version = %q/%q", first.PolicyVersion, first.PreviewVersion)
	}
	if first.ActiveRatingReadPath != "apollo_legacy_rating_v1" {
		t.Fatalf("ActiveRatingReadPath = %q, want legacy policy", first.ActiveRatingReadPath)
	}
	if first.Sides[0].Members[0].UserID != uuid.MustParse("22222222-2222-2222-2222-222222222222") {
		t.Fatalf("side one user = %s, want higher rated candidate", first.Sides[0].Members[0].UserID)
	}
	if !slices.EqualFunc(first.Sides, second.Sides, func(left, right CompetitionPreviewSide) bool {
		return left.SideIndex == right.SideIndex &&
			left.AverageMu == right.AverageMu &&
			slices.EqualFunc(left.Members, right.Members, func(leftMember, rightMember CompetitionPreviewMember) bool {
				return leftMember.QueueIntentID == rightMember.QueueIntentID &&
					leftMember.UserID == rightMember.UserID &&
					leftMember.RatingMu == rightMember.RatingMu &&
					leftMember.RatingSource == rightMember.RatingSource
			})
	}) {
		t.Fatalf("preview sides changed between equivalent inputs\nfirst=%#v\nsecond=%#v", first.Sides, second.Sides)
	}
}

func TestBuildCompetitionMatchPreviewExplainsProvisionalAndMixedTierInputs(t *testing.T) {
	preview := BuildCompetitionMatchPreview(CompetitionPreviewInput{
		SessionID:           uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		QueueVersion:        2,
		FacilityKey:         "ashtonbee",
		SportKey:            "badminton",
		ModeKey:             "head_to_head:s2-p1",
		SidesPerMatch:       2,
		ParticipantsPerSide: 1,
		Candidates: []CompetitionPreviewCandidate{
			previewCandidate("11111111-1111-1111-1111-111111111111", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", 25, RatingSourceInitialRating, "open", time.Date(2026, 4, 18, 12, 1, 0, 0, time.UTC)),
			previewCandidate("22222222-2222-2222-2222-222222222222", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", 25, RatingSourceLegacyProjection, "competitive", time.Date(2026, 4, 18, 12, 2, 0, 0, time.UTC)),
		},
	})

	if preview.ExplanationCode != ExplanationProvisionalRatingsUsed {
		t.Fatalf("ExplanationCode = %q, want %q", preview.ExplanationCode, ExplanationProvisionalRatingsUsed)
	}
	if !slices.Contains(preview.ExplanationCodes, ExplanationMixedTierPreview) {
		t.Fatalf("ExplanationCodes = %#v, want mixed tier code", preview.ExplanationCodes)
	}
	if preview.Tier != "mixed" {
		t.Fatalf("Tier = %q, want mixed", preview.Tier)
	}
	if preview.MatchQuality != 1 {
		t.Fatalf("MatchQuality = %.4f, want perfect balance with equal inputs", preview.MatchQuality)
	}
}

func previewCandidate(userID string, intentID string, mu float64, source string, tier string, joinedAt time.Time) CompetitionPreviewCandidate {
	return CompetitionPreviewCandidate{
		QueueIntentID:       uuid.MustParse(intentID),
		UserID:              uuid.MustParse(userID),
		Tier:                tier,
		RatingMu:            mu,
		RatingSigma:         8.3333,
		RatingMatchesPlayed: 1,
		RatingSource:        source,
		JoinedAt:            joinedAt,
		InputWatermark:      joinedAt,
	}
}
