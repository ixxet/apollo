package competition

import (
	"context"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/telemetry"
)

const (
	gameIdentityContractVersion      = "apollo_game_identity_v1"
	gameIdentityProjectionVersion    = "competition_game_identity_v1"
	gameIdentityCPPolicyVersion      = "apollo_cp_v1"
	gameIdentityBadgePolicyVersion   = "apollo_badge_awards_v1"
	gameIdentityRivalryPolicyVersion = "apollo_rivalry_state_v1"
	gameIdentitySquadPolicyVersion   = "apollo_squad_identity_v1"

	gameIdentityCPMatchesPlayedPoints = 10
	gameIdentityCPWinPoints           = 30
	gameIdentityCPDrawPoints          = 15
	gameIdentityCPLossPoints          = 5

	defaultGameIdentityLimit = 10
	maxGameIdentityLimit     = 50
)

var gameIdentityStatTypes = []string{
	analyticsStatMatchesPlayed,
	analyticsStatWins,
	analyticsStatLosses,
	analyticsStatDraws,
}

func (s *Service) PublicGameIdentity(ctx context.Context, input PublicGameIdentityInput) (GameIdentityProjection, error) {
	return s.gameIdentityProjection(ctx, nil, input, "participant")
}

func (s *Service) MemberGameIdentity(ctx context.Context, userID uuid.UUID, input PublicGameIdentityInput) (GameIdentityProjection, error) {
	return s.gameIdentityProjection(ctx, &userID, input, "member_self")
}

func (s *Service) gameIdentityProjection(ctx context.Context, userID *uuid.UUID, input PublicGameIdentityInput, participantPrefix string) (GameIdentityProjection, error) {
	startedAt := time.Now()
	defer func() {
		telemetry.ObserveGameIdentityProjection(time.Since(startedAt))
	}()

	normalized := normalizeGameIdentityInput(input)
	rows, err := s.repository.ListGameIdentityProjectionRows(ctx, GameIdentityProjectionInput{
		UserID:      userID,
		SportKey:    normalized.SportKey,
		ModeKey:     normalized.ModeKey,
		FacilityKey: normalized.FacilityKey,
		TeamScope:   normalized.TeamScope,
		Limit:       normalized.Limit,
	})
	if err != nil {
		return GameIdentityProjection{}, err
	}

	sortGameIdentityRows(rows)
	if len(rows) > normalized.Limit {
		rows = rows[:normalized.Limit]
	}

	status := publicCompetitionStatusEmpty
	if len(rows) > 0 {
		status = publicCompetitionStatusAvailable
	}

	cp := make([]GameIdentityCPProjection, 0, len(rows))
	labels := make(map[gameIdentityLabelKey]string, len(rows))
	for index, row := range rows {
		label := gameIdentityParticipantLabel(participantPrefix, index+1)
		labels[gameIdentityRowLabelKey(row)] = label
		cp = append(cp, buildGameIdentityCPProjection(row, index+1, label))
	}

	return GameIdentityProjection{
		ContractVersion:      gameIdentityContractVersion,
		ProjectionVersion:    gameIdentityProjectionVersion,
		Status:               status,
		ResultSource:         publicCompetitionResultSource,
		RatingSource:         publicCompetitionRatingSource,
		CPPolicyVersion:      gameIdentityCPPolicyVersion,
		BadgePolicyVersion:   gameIdentityBadgePolicyVersion,
		RivalryPolicyVersion: gameIdentityRivalryPolicyVersion,
		SquadPolicyVersion:   gameIdentitySquadPolicyVersion,
		CP:                   cp,
		BadgeAwards:          buildGameIdentityBadgeAwards(rows, labels),
		RivalryStates:        buildGameIdentityRivalryStates(rows, labels),
		SquadIdentities:      buildGameIdentitySquadIdentities(rows),
	}, nil
}

func normalizeGameIdentityInput(input PublicGameIdentityInput) PublicGameIdentityInput {
	input.SportKey = strings.TrimSpace(input.SportKey)
	input.ModeKey = strings.TrimSpace(input.ModeKey)
	input.FacilityKey = strings.TrimSpace(input.FacilityKey)
	input.TeamScope = strings.TrimSpace(input.TeamScope)
	if input.ModeKey == "" {
		input.ModeKey = analyticsDimensionAll
	}
	if input.FacilityKey == "" {
		input.FacilityKey = analyticsDimensionAll
	}
	if !stringsIn(input.TeamScope, publicLeaderboardTeamScopes) {
		input.TeamScope = analyticsDimensionAll
	}
	if input.Limit <= 0 {
		input.Limit = defaultGameIdentityLimit
	}
	if input.Limit > maxGameIdentityLimit {
		input.Limit = maxGameIdentityLimit
	}
	return input
}

func sortGameIdentityRows(rows []gameIdentityProjectionRowRecord) {
	sort.SliceStable(rows, func(i, j int) bool {
		leftCP := gameIdentityCP(rows[i])
		rightCP := gameIdentityCP(rows[j])
		if leftCP != rightCP {
			return leftCP > rightCP
		}
		if !rows[i].ComputedAt.Equal(rows[j].ComputedAt) {
			return rows[i].ComputedAt.After(rows[j].ComputedAt)
		}
		return rows[i].UserID.String() < rows[j].UserID.String()
	})
}

func buildGameIdentityCPProjection(row gameIdentityProjectionRowRecord, rank int, participant string) GameIdentityCPProjection {
	return GameIdentityCPProjection{
		Rank:        rank,
		Participant: participant,
		SportKey:    row.SportKey,
		ModeKey:     row.ModeKey,
		FacilityKey: row.FacilityKey,
		TeamScope:   row.TeamScope,
		CP:          gameIdentityCP(row),
		Components: []GameIdentityCPComponent{
			gameIdentityCPComponent(analyticsStatMatchesPlayed, row.MatchesPlayed, gameIdentityCPMatchesPlayedPoints),
			gameIdentityCPComponent(analyticsStatWins, row.Wins, gameIdentityCPWinPoints),
			gameIdentityCPComponent(analyticsStatDraws, row.Draws, gameIdentityCPDrawPoints),
			gameIdentityCPComponent(analyticsStatLosses, row.Losses, gameIdentityCPLossPoints),
		},
		ComputedAt: row.ComputedAt.UTC(),
	}
}

func gameIdentityCP(row gameIdentityProjectionRowRecord) int {
	return pointsFor(row.MatchesPlayed, gameIdentityCPMatchesPlayedPoints) +
		pointsFor(row.Wins, gameIdentityCPWinPoints) +
		pointsFor(row.Draws, gameIdentityCPDrawPoints) +
		pointsFor(row.Losses, gameIdentityCPLossPoints)
}

func gameIdentityCPComponent(metric string, value float64, pointsPerUnit int) GameIdentityCPComponent {
	return GameIdentityCPComponent{
		Metric:        metric,
		UnitValue:     value,
		PointsPerUnit: pointsPerUnit,
		Points:        pointsFor(value, pointsPerUnit),
	}
}

func pointsFor(value float64, pointsPerUnit int) int {
	if value <= 0 {
		return 0
	}
	return int(math.Round(value)) * pointsPerUnit
}

func buildGameIdentityBadgeAwards(rows []gameIdentityProjectionRowRecord, labels map[gameIdentityLabelKey]string) []GameIdentityBadgeAward {
	awards := make([]GameIdentityBadgeAward, 0)
	for _, row := range rows {
		participant := labels[gameIdentityRowLabelKey(row)]
		if row.MatchesPlayed >= 1 {
			awards = append(awards, gameIdentityBadgeAward(row, participant, "first_match", "First match", analyticsStatMatchesPlayed, row.MatchesPlayed, 1))
		}
		if row.Wins >= 1 {
			awards = append(awards, gameIdentityBadgeAward(row, participant, "first_win", "First win", analyticsStatWins, row.Wins, 1))
		}
		if row.MatchesPlayed >= 5 {
			awards = append(awards, gameIdentityBadgeAward(row, participant, "regular_competitor", "Regular competitor", analyticsStatMatchesPlayed, row.MatchesPlayed, 5))
		}
	}

	sort.SliceStable(awards, func(i, j int) bool {
		if awards[i].Participant != awards[j].Participant {
			return awards[i].Participant < awards[j].Participant
		}
		return awards[i].BadgeKey < awards[j].BadgeKey
	})
	return awards
}

func gameIdentityBadgeAward(row gameIdentityProjectionRowRecord, participant string, badgeKey string, badgeName string, metric string, value float64, threshold float64) GameIdentityBadgeAward {
	return GameIdentityBadgeAward{
		Participant:   participant,
		BadgeKey:      badgeKey,
		BadgeName:     badgeName,
		PolicyVersion: gameIdentityBadgePolicyVersion,
		SportKey:      row.SportKey,
		ModeKey:       row.ModeKey,
		FacilityKey:   row.FacilityKey,
		TeamScope:     row.TeamScope,
		Evidence: []GameIdentityBadgeEvidence{{
			Metric:    metric,
			Value:     value,
			Threshold: threshold,
		}},
		AwardedAt:  row.LastResultAt,
		ComputedAt: row.ComputedAt.UTC(),
	}
}

func buildGameIdentityRivalryStates(rows []gameIdentityProjectionRowRecord, labels map[gameIdentityLabelKey]string) []GameIdentityRivalryState {
	groups := make(map[gameIdentitySquadKey][]gameIdentityProjectionRowRecord)
	for _, row := range rows {
		key := gameIdentityRowContextKey(row)
		groups[key] = append(groups[key], row)
	}

	keys := make([]gameIdentitySquadKey, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		return keys[i].String() < keys[j].String()
	})

	rivalries := make([]GameIdentityRivalryState, 0)
	for _, key := range keys {
		group := groups[key]
		if len(group) < 2 {
			continue
		}
		sortGameIdentityRows(group)

		left := group[0]
		right := group[1]
		leftCP := gameIdentityCP(left)
		rightCP := gameIdentityCP(right)
		gap := leftCP - rightCP
		state := "emerging"
		if gap <= 50 {
			state = "active"
		}
		computedAt := latestTime(left.ComputedAt, right.ComputedAt)
		rivalries = append(rivalries, GameIdentityRivalryState{
			RivalryKey:    gameIdentityKey("rivalry", key.sportKey, key.modeKey, key.facilityKey, key.teamScope),
			State:         state,
			PolicyVersion: gameIdentityRivalryPolicyVersion,
			SportKey:      key.sportKey,
			ModeKey:       key.modeKey,
			FacilityKey:   key.facilityKey,
			TeamScope:     key.teamScope,
			Participants:  []string{labels[gameIdentityRowLabelKey(left)], labels[gameIdentityRowLabelKey(right)]},
			Leader:        labels[gameIdentityRowLabelKey(left)],
			CPGap:         gap,
			ComputedAt:    computedAt.UTC(),
		})
	}
	return rivalries
}

func buildGameIdentitySquadIdentities(rows []gameIdentityProjectionRowRecord) []GameIdentitySquadIdentity {
	type squadAggregate struct {
		key        gameIdentitySquadKey
		count      int
		cpTotal    int
		computedAt time.Time
	}

	aggregates := make(map[gameIdentitySquadKey]*squadAggregate)
	for _, row := range rows {
		key := gameIdentityRowContextKey(row)
		aggregate, exists := aggregates[key]
		if !exists {
			aggregate = &squadAggregate{key: key}
			aggregates[key] = aggregate
		}
		aggregate.count++
		aggregate.cpTotal += gameIdentityCP(row)
		aggregate.computedAt = latestTime(aggregate.computedAt, row.ComputedAt)
	}

	keys := make([]gameIdentitySquadKey, 0, len(aggregates))
	for key := range aggregates {
		keys = append(keys, key)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		return keys[i].String() < keys[j].String()
	})

	squads := make([]GameIdentitySquadIdentity, 0, len(keys))
	for _, key := range keys {
		aggregate := aggregates[key]
		squads = append(squads, GameIdentitySquadIdentity{
			SquadKey:         gameIdentityKey("squad", key.sportKey, key.modeKey, key.facilityKey, key.teamScope),
			SquadName:        gameIdentitySquadName(key.teamScope),
			PolicyVersion:    gameIdentitySquadPolicyVersion,
			SportKey:         key.sportKey,
			ModeKey:          key.modeKey,
			FacilityKey:      key.facilityKey,
			TeamScope:        key.teamScope,
			ParticipantCount: aggregate.count,
			CPTotal:          aggregate.cpTotal,
			ComputedAt:       aggregate.computedAt.UTC(),
		})
	}
	return squads
}

type gameIdentitySquadKey struct {
	sportKey    string
	modeKey     string
	facilityKey string
	teamScope   string
}

type gameIdentityLabelKey struct {
	userID uuid.UUID
	key    gameIdentitySquadKey
}

func gameIdentityRowContextKey(row gameIdentityProjectionRowRecord) gameIdentitySquadKey {
	return gameIdentitySquadKey{
		sportKey:    row.SportKey,
		modeKey:     row.ModeKey,
		facilityKey: row.FacilityKey,
		teamScope:   row.TeamScope,
	}
}

func gameIdentityRowLabelKey(row gameIdentityProjectionRowRecord) gameIdentityLabelKey {
	return gameIdentityLabelKey{
		userID: row.UserID,
		key:    gameIdentityRowContextKey(row),
	}
}

func (k gameIdentitySquadKey) String() string {
	return strings.Join([]string{k.sportKey, k.modeKey, k.facilityKey, k.teamScope}, "|")
}

func gameIdentityParticipantLabel(prefix string, rank int) string {
	if prefix == "member_self" {
		return prefix
	}
	return prefix + "_" + strconv.Itoa(rank)
}

func gameIdentitySquadName(teamScope string) string {
	switch teamScope {
	case analyticsTeamScopeSolo:
		return "Solo cohort"
	case analyticsTeamScopeTeam:
		return "Team cohort"
	default:
		return "Competition cohort"
	}
}

func gameIdentityKey(prefix string, parts ...string) string {
	normalized := make([]string, 0, len(parts)+1)
	normalized = append(normalized, prefix)
	for _, part := range parts {
		clean := strings.TrimSpace(strings.ToLower(part))
		clean = strings.ReplaceAll(clean, ":", "_")
		clean = strings.ReplaceAll(clean, " ", "_")
		if clean == "" {
			clean = analyticsDimensionAll
		}
		normalized = append(normalized, clean)
	}
	return strings.Join(normalized, ":")
}

func stringsIn(value string, allowed []string) bool {
	for _, entry := range allowed {
		if value == entry {
			return true
		}
	}
	return false
}

func latestTime(left time.Time, right time.Time) time.Time {
	if right.After(left) {
		return right
	}
	return left
}
