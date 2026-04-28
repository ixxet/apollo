package ares

import (
	"math"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/rating"
)

const (
	CompetitionPreviewVersion = "v2"
	CompetitionPreviewPolicy  = "apollo_ares_match_preview_v2"

	ExplanationBalancedLegacyRating      = "balanced_legacy_rating"
	ExplanationAcceptableLegacyRatingGap = "acceptable_legacy_rating_gap"
	ExplanationWideLegacyRatingGap       = "wide_legacy_rating_gap"
	ExplanationProvisionalRatingsUsed    = "provisional_ratings_used"
	ExplanationMixedTierPreview          = "mixed_tier_preview"

	RatingSourceLegacyProjection = "legacy_projection"
	RatingSourceInitialRating    = "initial_rating"
)

type CompetitionPreviewInput struct {
	SessionID           uuid.UUID
	QueueVersion        int
	FacilityKey         string
	SportKey            string
	ModeKey             string
	SidesPerMatch       int
	ParticipantsPerSide int
	Candidates          []CompetitionPreviewCandidate
}

type CompetitionPreviewCandidate struct {
	QueueIntentID       uuid.UUID
	UserID              uuid.UUID
	DisplayName         string
	Tier                string
	RatingMu            float64
	RatingSigma         float64
	RatingMatchesPlayed int
	RatingSource        string
	JoinedAt            time.Time
	InputWatermark      time.Time
}

type CompetitionMatchPreview struct {
	InputWatermark            time.Time                `json:"input_watermark"`
	CompetitionSessionID      uuid.UUID                `json:"competition_session_id"`
	QueueVersion              int                      `json:"queue_version"`
	ProposalIndex             int                      `json:"proposal_index"`
	PreviewVersion            string                   `json:"preview_version"`
	PolicyVersion             string                   `json:"policy_version"`
	RatingEngine              string                   `json:"rating_engine"`
	RatingPolicyVersion       string                   `json:"rating_policy_version"`
	FacilityKey               string                   `json:"facility_key"`
	SportKey                  string                   `json:"sport_key"`
	ModeKey                   string                   `json:"mode_key"`
	Tier                      string                   `json:"tier"`
	MatchQuality              float64                  `json:"match_quality"`
	PredictedWinProbability   float64                  `json:"predicted_win_probability"`
	ExplanationCode           string                   `json:"explanation_code"`
	ExplanationCodes          []string                 `json:"explanation_codes"`
	Sides                     []CompetitionPreviewSide `json:"sides"`
	CandidateCount            int                      `json:"candidate_count"`
	MissingRatingCount        int                      `json:"missing_rating_count"`
	AverageRatingDelta        float64                  `json:"average_rating_delta"`
	ActiveRatingReadPath      string                   `json:"active_rating_read_path"`
	OpenSkillComparisonPolicy string                   `json:"openskill_comparison_policy,omitempty"`
}

type CompetitionPreviewSide struct {
	SideIndex int                        `json:"side_index"`
	AverageMu float64                    `json:"average_mu"`
	Members   []CompetitionPreviewMember `json:"members"`
}

type CompetitionPreviewMember struct {
	QueueIntentID       uuid.UUID `json:"queue_intent_id"`
	UserID              uuid.UUID `json:"user_id"`
	MemberLabel         string    `json:"member_label"`
	Tier                string    `json:"tier"`
	RatingMu            float64   `json:"rating_mu"`
	RatingSigma         float64   `json:"rating_sigma"`
	RatingMatchesPlayed int       `json:"rating_matches_played"`
	RatingSource        string    `json:"rating_source"`
}

func BuildCompetitionMatchPreview(input CompetitionPreviewInput) CompetitionMatchPreview {
	candidates := append([]CompetitionPreviewCandidate(nil), input.Candidates...)
	slices.SortFunc(candidates, func(left, right CompetitionPreviewCandidate) int {
		if left.RatingMu > right.RatingMu {
			return -1
		}
		if left.RatingMu < right.RatingMu {
			return 1
		}
		if cmp := left.JoinedAt.UTC().Compare(right.JoinedAt.UTC()); cmp != 0 {
			return cmp
		}
		return slices.Compare(left.UserID[:], right.UserID[:])
	})

	sides := make([]CompetitionPreviewSide, input.SidesPerMatch)
	for index := range sides {
		sides[index] = CompetitionPreviewSide{
			SideIndex: index + 1,
			Members:   make([]CompetitionPreviewMember, 0, input.ParticipantsPerSide),
		}
	}

	for _, candidate := range candidates {
		sideIndex := bestPreviewSide(sides, input.ParticipantsPerSide)
		member := CompetitionPreviewMember{
			QueueIntentID:       candidate.QueueIntentID,
			UserID:              candidate.UserID,
			MemberLabel:         memberLabel(candidate.UserID),
			Tier:                candidate.Tier,
			RatingMu:            round4(candidate.RatingMu),
			RatingSigma:         round4(candidate.RatingSigma),
			RatingMatchesPlayed: candidate.RatingMatchesPlayed,
			RatingSource:        candidate.RatingSource,
		}
		sides[sideIndex].Members = append(sides[sideIndex].Members, member)
		sides[sideIndex].AverageMu = round4(sideAverageMu(sides[sideIndex]))
	}

	for index := range sides {
		slices.SortFunc(sides[index].Members, func(left, right CompetitionPreviewMember) int {
			return slices.Compare(left.UserID[:], right.UserID[:])
		})
		sides[index].AverageMu = round4(sideAverageMu(sides[index]))
	}
	slices.SortFunc(sides, func(left, right CompetitionPreviewSide) int {
		return left.SideIndex - right.SideIndex
	})

	delta := averageRatingDelta(sides)
	missingRatingCount := countMissingRatings(candidates)
	tier := aggregateTier(candidates)
	explanationCodes := explanationCodes(delta, missingRatingCount, tier)

	return CompetitionMatchPreview{
		InputWatermark:            latestInputWatermark(candidates),
		CompetitionSessionID:      input.SessionID,
		QueueVersion:              input.QueueVersion,
		ProposalIndex:             1,
		PreviewVersion:            CompetitionPreviewVersion,
		PolicyVersion:             CompetitionPreviewPolicy,
		RatingEngine:              rating.EngineLegacyEloLike,
		RatingPolicyVersion:       rating.PolicyVersionLegacy,
		FacilityKey:               input.FacilityKey,
		SportKey:                  input.SportKey,
		ModeKey:                   input.ModeKey,
		Tier:                      tier,
		MatchQuality:              matchQuality(delta),
		PredictedWinProbability:   predictedWinProbability(sides),
		ExplanationCode:           explanationCodes[0],
		ExplanationCodes:          explanationCodes,
		Sides:                     sides,
		CandidateCount:            len(candidates),
		MissingRatingCount:        missingRatingCount,
		AverageRatingDelta:        round4(delta),
		ActiveRatingReadPath:      rating.PolicyVersionLegacy,
		OpenSkillComparisonPolicy: rating.PolicyVersionOpenSkill,
	}
}

func bestPreviewSide(sides []CompetitionPreviewSide, capacity int) int {
	best := -1
	for index, side := range sides {
		if len(side.Members) >= capacity {
			continue
		}
		if best == -1 {
			best = index
			continue
		}
		if side.AverageMu < sides[best].AverageMu {
			best = index
			continue
		}
		if side.AverageMu == sides[best].AverageMu && len(side.Members) < len(sides[best].Members) {
			best = index
		}
	}
	if best == -1 {
		return len(sides) - 1
	}
	return best
}

func sideAverageMu(side CompetitionPreviewSide) float64 {
	if len(side.Members) == 0 {
		return 0
	}
	total := 0.0
	for _, member := range side.Members {
		total += member.RatingMu
	}
	return total / float64(len(side.Members))
}

func averageRatingDelta(sides []CompetitionPreviewSide) float64 {
	if len(sides) < 2 {
		return 0
	}
	low := sides[0].AverageMu
	high := sides[0].AverageMu
	for _, side := range sides[1:] {
		low = math.Min(low, side.AverageMu)
		high = math.Max(high, side.AverageMu)
	}
	return high - low
}

func matchQuality(delta float64) float64 {
	return round4(math.Max(0, 1-(delta/16)))
}

func predictedWinProbability(sides []CompetitionPreviewSide) float64 {
	if len(sides) < 2 {
		return 0.5
	}
	return round4(rating.LegacyExpectedWinProbability(sides[0].AverageMu, sides[1].AverageMu))
}

func countMissingRatings(candidates []CompetitionPreviewCandidate) int {
	count := 0
	for _, candidate := range candidates {
		if candidate.RatingSource == RatingSourceInitialRating {
			count++
		}
	}
	return count
}

func aggregateTier(candidates []CompetitionPreviewCandidate) string {
	if len(candidates) == 0 {
		return "open"
	}
	tier := candidates[0].Tier
	for _, candidate := range candidates[1:] {
		if candidate.Tier != tier {
			return "mixed"
		}
	}
	return tier
}

func explanationCodes(delta float64, missingRatingCount int, tier string) []string {
	codes := make([]string, 0, 3)
	switch {
	case missingRatingCount > 0:
		codes = append(codes, ExplanationProvisionalRatingsUsed)
	case delta <= 2:
		codes = append(codes, ExplanationBalancedLegacyRating)
	case delta <= 6:
		codes = append(codes, ExplanationAcceptableLegacyRatingGap)
	default:
		codes = append(codes, ExplanationWideLegacyRatingGap)
	}
	if tier == "mixed" {
		codes = append(codes, ExplanationMixedTierPreview)
	}
	return codes
}

func latestInputWatermark(candidates []CompetitionPreviewCandidate) time.Time {
	var latest time.Time
	for _, candidate := range candidates {
		if candidate.InputWatermark.After(latest) {
			latest = candidate.InputWatermark.UTC()
		}
	}
	if latest.IsZero() {
		return time.Unix(0, 0).UTC()
	}
	return latest
}

func round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}
