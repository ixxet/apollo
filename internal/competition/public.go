package competition

import (
	"context"
	"slices"
	"strconv"
	"strings"
)

const (
	publicCompetitionContractVersion = "apollo_public_competition_v1"
	publicCompetitionStatusAvailable = "available"
	publicCompetitionStatusEmpty     = "unavailable"
	publicCompetitionResultSource    = "finalized_or_corrected_canonical_results"
	publicCompetitionRatingSource    = "legacy_elo_like_active_projection"
	defaultPublicLeaderboardLimit    = 25
	maxPublicLeaderboardLimit        = 100
)

var (
	publicCompetitionDeferred = []string{
		"messaging_chat",
		"public_social_graph",
		"rating_read_path_switch",
		"project_wide_semver",
	}
	publicLeaderboardStatTypes = []string{
		analyticsStatMatchesPlayed,
		analyticsStatWins,
		analyticsStatLosses,
		analyticsStatDraws,
		analyticsStatRatingMovement,
	}
	publicLeaderboardTeamScopes = []string{
		analyticsDimensionAll,
		analyticsTeamScopeSolo,
		analyticsTeamScopeTeam,
	}
)

func (s *Service) PublicCompetitionReadiness(ctx context.Context) (PublicCompetitionReadiness, error) {
	record, err := s.repository.GetPublicCompetitionReadiness(ctx)
	if err != nil {
		return PublicCompetitionReadiness{}, err
	}

	status := publicCompetitionStatusEmpty
	if record.AvailableLeaderboards > 0 && record.AvailableCanonicalResults > 0 {
		status = publicCompetitionStatusAvailable
	}

	return PublicCompetitionReadiness{
		ContractVersion:           publicCompetitionContractVersion,
		ProjectionVersion:         competitionAnalyticsProjectionVersion,
		Status:                    status,
		ResultSource:              publicCompetitionResultSource,
		RatingSource:              publicCompetitionRatingSource,
		AvailableLeaderboards:     record.AvailableLeaderboards,
		AvailableCanonicalResults: record.AvailableCanonicalResults,
		Deferred:                  slices.Clone(publicCompetitionDeferred),
	}, nil
}

func (s *Service) ListPublicCompetitionLeaderboard(ctx context.Context, input PublicCompetitionLeaderboardInput) (PublicCompetitionLeaderboard, error) {
	normalized := normalizePublicCompetitionLeaderboardInput(input)
	rows, err := s.repository.ListPublicCompetitionLeaderboard(ctx, normalized)
	if err != nil {
		return PublicCompetitionLeaderboard{}, err
	}

	leaderboard := make([]PublicCompetitionLeaderboardRow, 0, len(rows))
	for index, row := range rows {
		leaderboard = append(leaderboard, PublicCompetitionLeaderboardRow{
			Rank:         index + 1,
			Participant:  publicCompetitionParticipantLabel(index + 1),
			SportKey:     row.SportKey,
			ModeKey:      row.ModeKey,
			FacilityKey:  row.FacilityKey,
			TeamScope:    row.TeamScope,
			StatType:     row.StatType,
			StatValue:    row.StatValue,
			LastResultAt: row.LastResultAt,
			ComputedAt:   row.ComputedAt,
		})
	}

	return PublicCompetitionLeaderboard{
		ContractVersion:   publicCompetitionContractVersion,
		ProjectionVersion: competitionAnalyticsProjectionVersion,
		ResultSource:      publicCompetitionResultSource,
		RatingSource:      publicCompetitionRatingSource,
		Leaderboard:       leaderboard,
	}, nil
}

func normalizePublicCompetitionLeaderboardInput(input PublicCompetitionLeaderboardInput) PublicCompetitionLeaderboardInput {
	input.SportKey = strings.TrimSpace(input.SportKey)
	input.ModeKey = strings.TrimSpace(input.ModeKey)
	input.StatType = strings.TrimSpace(input.StatType)
	input.TeamScope = strings.TrimSpace(input.TeamScope)
	if !slices.Contains(publicLeaderboardStatTypes, input.StatType) {
		input.StatType = analyticsStatWins
	}
	if !slices.Contains(publicLeaderboardTeamScopes, input.TeamScope) {
		input.TeamScope = analyticsDimensionAll
	}
	if input.Limit <= 0 {
		input.Limit = defaultPublicLeaderboardLimit
	}
	if input.Limit > maxPublicLeaderboardLimit {
		input.Limit = maxPublicLeaderboardLimit
	}
	return input
}

func publicCompetitionParticipantLabel(rank int) string {
	return "participant_" + strconv.Itoa(rank)
}
