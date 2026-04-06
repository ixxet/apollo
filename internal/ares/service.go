package ares

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/eligibility"
	"github.com/ixxet/apollo/internal/profile"
)

const PreviewVersion = "v1"

type CandidateSource interface {
	ListJoinedLobbyMatchPreviewCandidates(ctx context.Context) ([]JoinedLobbyCandidate, error)
}

type JoinedLobbyCandidate struct {
	UserID              uuid.UUID
	Preferences         []byte
	UserUpdatedAt       time.Time
	JoinedAt            time.Time
	MembershipUpdatedAt time.Time
}

type Service struct {
	repository CandidateSource
}

type MatchPreview struct {
	GeneratedAt        *time.Time  `json:"generated_at"`
	CandidateCount     int         `json:"candidate_count"`
	PreviewVersion     string      `json:"preview_version"`
	Matches            []Match     `json:"matches"`
	UnmatchedMemberIDs []uuid.UUID `json:"unmatched_member_ids,omitempty"`
	UnmatchedLabels    []string    `json:"unmatched_labels,omitempty"`
}

type Match struct {
	MemberIDs    []uuid.UUID `json:"member_ids"`
	MemberLabels []string    `json:"member_labels"`
	Score        int         `json:"score"`
	Reasons      []Reason    `json:"reasons"`
}

type Reason struct {
	Code  string `json:"code"`
	Value string `json:"value,omitempty"`
}

func NewService(repository CandidateSource) *Service {
	return &Service{repository: repository}
}

func (s *Service) GetLobbyMatchPreview(ctx context.Context) (MatchPreview, error) {
	rows, err := s.repository.ListJoinedLobbyMatchPreviewCandidates(ctx)
	if err != nil {
		return MatchPreview{}, err
	}

	return buildPreview(rows), nil
}

func buildPreview(rows []JoinedLobbyCandidate) MatchPreview {
	candidates := selectEligibleCandidates(rows)
	slices.SortFunc(candidates, func(left, right candidate) int {
		if cmp := slices.Compare(left.UserID[:], right.UserID[:]); cmp != 0 {
			return cmp
		}
		return 0
	})

	preview := MatchPreview{
		GeneratedAt:    latestCandidateTime(candidates),
		CandidateCount: len(candidates),
		PreviewVersion: PreviewVersion,
		Matches:        make([]Match, 0, len(candidates)/2),
	}

	for index := 0; index+1 < len(candidates); index += 2 {
		left := candidates[index]
		right := candidates[index+1]
		preview.Matches = append(preview.Matches, Match{
			MemberIDs:    []uuid.UUID{left.UserID, right.UserID},
			MemberLabels: []string{memberLabel(left.UserID), memberLabel(right.UserID)},
			Score:        2,
			Reasons: []Reason{
				{Code: "explicit_joined_membership"},
				{Code: "compatible_visibility_mode", Value: left.VisibilityMode},
				{Code: "compatible_availability_mode", Value: left.AvailabilityMode},
				{Code: "stable_pair_order", Value: "user_id_asc"},
			},
		})
	}

	if len(candidates)%2 == 1 {
		last := candidates[len(candidates)-1]
		preview.UnmatchedMemberIDs = []uuid.UUID{last.UserID}
		preview.UnmatchedLabels = []string{memberLabel(last.UserID)}
	}

	return preview
}

type candidate struct {
	UserID           uuid.UUID
	VisibilityMode   string
	AvailabilityMode string
	Watermark        time.Time
}

func selectEligibleCandidates(rows []JoinedLobbyCandidate) []candidate {
	candidates := make([]candidate, 0, len(rows))
	for _, row := range rows {
		modes := profile.ReadPreferenceModes(row.Preferences)
		lobbyEligibility := eligibility.FromPreferenceModes(modes)
		if !lobbyEligibility.Eligible {
			continue
		}

		candidates = append(candidates, candidate{
			UserID:           row.UserID,
			VisibilityMode:   lobbyEligibility.VisibilityMode,
			AvailabilityMode: lobbyEligibility.AvailabilityMode,
			Watermark:        latestRowTime(row),
		})
	}

	return candidates
}

func latestCandidateTime(candidates []candidate) *time.Time {
	var latest time.Time
	for _, candidate := range candidates {
		if candidate.Watermark.After(latest) {
			latest = candidate.Watermark.UTC()
		}
	}

	if latest.IsZero() {
		return nil
	}

	return &latest
}

func latestRowTime(row JoinedLobbyCandidate) time.Time {
	latest := row.UserUpdatedAt.UTC()
	if row.MembershipUpdatedAt.After(latest) {
		latest = row.MembershipUpdatedAt.UTC()
	}

	return latest
}

func memberLabel(userID uuid.UUID) string {
	value := userID.String()
	if len(value) <= 8 {
		return value
	}

	return value[:8]
}
