package competition

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/store"
)

const (
	competitionAnalyticsProjectionVersion = "competition_analytics_v1"
	analyticsDimensionAll                 = "all"
	analyticsTeamScopeSolo                = "solo"
	analyticsTeamScopeTeam                = "team"

	analyticsStatMatchesPlayed    = "matches_played"
	analyticsStatWins             = "wins"
	analyticsStatLosses           = "losses"
	analyticsStatDraws            = "draws"
	analyticsStatCurrentStreak    = "current_streak"
	analyticsStatRatingMovement   = "rating_movement"
	analyticsStatOpponentStrength = "opponent_strength"
	analyticsStatTeamVsSoloDelta  = "team_vs_solo_delta"
)

type analyticsParticipant struct {
	userID           uuid.UUID
	ratingEventFound bool
	ratingDeltaMu    float64
	ratingMu         float64
}

type analyticsSide struct {
	teamID       uuid.UUID
	sideIndex    int
	outcome      string
	participants []analyticsParticipant
}

type analyticsMatch struct {
	sourceMatchID       uuid.UUID
	sourceResultID      uuid.UUID
	sportKey            string
	facilityKey         string
	modeKey             string
	participantsPerSide int
	recordedAt          time.Time
	sides               []analyticsSide
}

type analyticsProjectionKey struct {
	userID      uuid.UUID
	sportKey    string
	facilityKey string
	modeKey     string
	teamScope   string
}

type analyticsAggregate struct {
	key                     analyticsProjectionKey
	matchesPlayed           int
	wins                    int
	losses                  int
	draws                   int
	currentStreak           int
	ratingMovement          float64
	ratingMovementSamples   int
	opponentStrengthTotal   float64
	opponentStrengthSamples int
	sourceMatchID           uuid.UUID
	sourceResultID          uuid.UUID
	lastRecordedAt          time.Time
}

type analyticsProjectionFact struct {
	key            analyticsProjectionKey
	statType       string
	statValue      float64
	sourceMatchID  uuid.UUID
	sourceResultID uuid.UUID
	sampleSize     int
	confidence     float64
}

func recomputeCompetitionAnalyticsTx(ctx context.Context, queries *store.Queries, sportKey string, computedAt time.Time) error {
	rows, err := queries.ListCompetitionAnalyticsParticipantsBySport(ctx, sportKey)
	if err != nil {
		return err
	}

	matches, err := buildAnalyticsMatches(rows)
	if err != nil {
		return err
	}
	projectionWatermark := analyticsProjectionWatermark(matches)

	if _, err := queries.DeleteCompetitionAnalyticsProjectionsBySportVersion(ctx, store.DeleteCompetitionAnalyticsProjectionsBySportVersionParams{
		SportKey:          sportKey,
		ProjectionVersion: competitionAnalyticsProjectionVersion,
	}); err != nil {
		return err
	}
	if _, err := queries.DeleteCompetitionAnalyticsStatEventsBySportVersion(ctx, store.DeleteCompetitionAnalyticsStatEventsBySportVersionParams{
		SportKey:          sportKey,
		ProjectionVersion: competitionAnalyticsProjectionVersion,
	}); err != nil {
		return err
	}

	aggregates := make(map[analyticsProjectionKey]*analyticsAggregate)
	for _, match := range matches {
		for _, side := range match.sides {
			for _, participant := range side.participants {
				opponentStrength, hasOpponentStrength := averageOpponentRating(match, side.teamID)
				teamScope := analyticsTeamScope(match.participantsPerSide)
				if err := recordAnalyticsStatEventsTx(ctx, queries, match, side, participant, teamScope, opponentStrength, hasOpponentStrength, projectionWatermark, computedAt); err != nil {
					return err
				}
				for _, key := range analyticsProjectionKeys(participant.userID, match.sportKey, match.facilityKey, match.modeKey, teamScope) {
					aggregate := ensureAnalyticsAggregate(aggregates, key)
					aggregate.apply(match, side.outcome, participant, opponentStrength, hasOpponentStrength)
				}
			}
		}
	}

	facts := analyticsProjectionFacts(aggregates)
	for _, fact := range facts {
		if analyticsProjectionFactEmitsStatEvent(fact) {
			if err := createAnalyticsProjectionFactStatEventTx(ctx, queries, fact, projectionWatermark, computedAt); err != nil {
				return err
			}
		}
		statValue, err := numericFromFloat64(fact.statValue)
		if err != nil {
			return err
		}
		confidence, err := numericFromFloat64(fact.confidence)
		if err != nil {
			return err
		}
		if _, err := queries.UpsertCompetitionAnalyticsProjection(ctx, store.UpsertCompetitionAnalyticsProjectionParams{
			UserID:              fact.key.userID,
			SportKey:            fact.key.sportKey,
			FacilityKey:         fact.key.facilityKey,
			ModeKey:             fact.key.modeKey,
			TeamScope:           fact.key.teamScope,
			StatType:            fact.statType,
			StatValue:           statValue,
			SourceMatchID:       optionalUUID(&fact.sourceMatchID),
			SourceResultID:      optionalUUID(&fact.sourceResultID),
			SampleSize:          int32(fact.sampleSize),
			Confidence:          confidence,
			ComputedAt:          timestamptz(computedAt),
			ProjectionVersion:   competitionAnalyticsProjectionVersion,
			ProjectionWatermark: projectionWatermark,
			UpdatedAt:           timestamptz(computedAt),
		}); err != nil {
			return err
		}
	}

	confidence, err := numericFromFloat64(analyticsConfidence(len(matches)))
	if err != nil {
		return err
	}
	sourceMatchID, sourceResultID := analyticsProjectionSource(matches)
	_, err = queries.CreateCompetitionAnalyticsProjectionRebuiltEvent(ctx, store.CreateCompetitionAnalyticsProjectionRebuiltEventParams{
		ProjectionVersion:   competitionAnalyticsProjectionVersion,
		ProjectionWatermark: projectionWatermark,
		SportKey:            sportKey,
		SourceMatchID:       optionalUUID(sourceMatchID),
		SourceResultID:      optionalUUID(sourceResultID),
		SampleSize:          int32(len(matches)),
		Confidence:          confidence,
		ComputedAt:          timestamptz(computedAt),
	})
	return err
}

func analyticsProjectionFactEmitsStatEvent(fact analyticsProjectionFact) bool {
	return fact.statType == analyticsStatCurrentStreak || fact.statType == analyticsStatTeamVsSoloDelta
}

func buildAnalyticsMatches(rows []store.ListCompetitionAnalyticsParticipantsBySportRow) ([]analyticsMatch, error) {
	matches := make([]analyticsMatch, 0, len(rows))
	matchIndex := make(map[uuid.UUID]int, len(rows))
	sideIndexByMatch := make(map[uuid.UUID]map[uuid.UUID]int, len(rows))

	for _, row := range rows {
		index, exists := matchIndex[row.SourceResultID]
		if !exists {
			index = len(matches)
			matchIndex[row.SourceResultID] = index
			matches = append(matches, analyticsMatch{
				sourceMatchID:       row.SourceMatchID,
				sourceResultID:      row.SourceResultID,
				sportKey:            row.SportKey,
				facilityKey:         row.FacilityKey,
				modeKey:             buildModeKey(row.CompetitionMode, int(row.SidesPerMatch), int(row.ParticipantsPerSide)),
				participantsPerSide: int(row.ParticipantsPerSide),
				recordedAt:          row.RecordedAt.Time.UTC(),
			})
			sideIndexByMatch[row.SourceResultID] = make(map[uuid.UUID]int)
		}

		sideMap := sideIndexByMatch[row.SourceResultID]
		sidePosition, sideExists := sideMap[row.CompetitionSessionTeamID]
		if !sideExists {
			sidePosition = len(matches[index].sides)
			sideMap[row.CompetitionSessionTeamID] = sidePosition
			matches[index].sides = append(matches[index].sides, analyticsSide{
				teamID:    row.CompetitionSessionTeamID,
				sideIndex: int(row.SideIndex),
				outcome:   row.Outcome,
			})
		}

		ratingDeltaMu, err := float64FromNumeric(row.RatingDeltaMu)
		if err != nil {
			return nil, err
		}
		ratingMu, err := float64FromNumeric(row.RatingMu)
		if err != nil {
			return nil, err
		}
		matches[index].sides[sidePosition].participants = append(matches[index].sides[sidePosition].participants, analyticsParticipant{
			userID:           row.UserID,
			ratingEventFound: row.RatingEventFound,
			ratingDeltaMu:    ratingDeltaMu,
			ratingMu:         ratingMu,
		})
	}

	return matches, nil
}

func recordAnalyticsStatEventsTx(ctx context.Context, queries *store.Queries, match analyticsMatch, side analyticsSide, participant analyticsParticipant, teamScope string, opponentStrength float64, hasOpponentStrength bool, projectionWatermark string, computedAt time.Time) error {
	statTypes := []string{analyticsStatMatchesPlayed}
	switch side.outcome {
	case matchOutcomeWin:
		statTypes = append(statTypes, analyticsStatWins)
	case matchOutcomeLoss:
		statTypes = append(statTypes, analyticsStatLosses)
	case matchOutcomeDraw:
		statTypes = append(statTypes, analyticsStatDraws)
	}

	for _, statType := range statTypes {
		if err := createAnalyticsStatEventTx(ctx, queries, match, participant.userID, teamScope, statType, 1, 1, projectionWatermark, computedAt); err != nil {
			return err
		}
	}
	if participant.ratingEventFound {
		if err := createAnalyticsStatEventTx(ctx, queries, match, participant.userID, teamScope, analyticsStatRatingMovement, participant.ratingDeltaMu, 1, projectionWatermark, computedAt); err != nil {
			return err
		}
	}
	if hasOpponentStrength {
		if err := createAnalyticsStatEventTx(ctx, queries, match, participant.userID, teamScope, analyticsStatOpponentStrength, opponentStrength, 1, projectionWatermark, computedAt); err != nil {
			return err
		}
	}
	return nil
}

func createAnalyticsStatEventTx(ctx context.Context, queries *store.Queries, match analyticsMatch, userID uuid.UUID, teamScope string, statType string, value float64, sampleSize int, projectionWatermark string, computedAt time.Time) error {
	statValue, err := numericFromFloat64(value)
	if err != nil {
		return err
	}
	confidence, err := numericFromFloat64(analyticsConfidence(sampleSize))
	if err != nil {
		return err
	}
	_, err = queries.CreateCompetitionAnalyticsStatEvent(ctx, store.CreateCompetitionAnalyticsStatEventParams{
		ProjectionVersion:   competitionAnalyticsProjectionVersion,
		ProjectionWatermark: projectionWatermark,
		UserID:              optionalUUID(&userID),
		SportKey:            match.sportKey,
		FacilityKey:         match.facilityKey,
		ModeKey:             match.modeKey,
		TeamScope:           teamScope,
		StatType:            statType,
		StatValue:           statValue,
		SourceMatchID:       optionalUUID(&match.sourceMatchID),
		SourceResultID:      optionalUUID(&match.sourceResultID),
		SampleSize:          int32(sampleSize),
		Confidence:          confidence,
		ComputedAt:          timestamptz(computedAt),
	})
	return err
}

func createAnalyticsProjectionFactStatEventTx(ctx context.Context, queries *store.Queries, fact analyticsProjectionFact, projectionWatermark string, computedAt time.Time) error {
	statValue, err := numericFromFloat64(fact.statValue)
	if err != nil {
		return err
	}
	confidence, err := numericFromFloat64(fact.confidence)
	if err != nil {
		return err
	}
	_, err = queries.CreateCompetitionAnalyticsStatEvent(ctx, store.CreateCompetitionAnalyticsStatEventParams{
		ProjectionVersion:   competitionAnalyticsProjectionVersion,
		ProjectionWatermark: projectionWatermark,
		UserID:              optionalUUID(&fact.key.userID),
		SportKey:            fact.key.sportKey,
		FacilityKey:         fact.key.facilityKey,
		ModeKey:             fact.key.modeKey,
		TeamScope:           fact.key.teamScope,
		StatType:            fact.statType,
		StatValue:           statValue,
		SourceMatchID:       optionalUUID(&fact.sourceMatchID),
		SourceResultID:      optionalUUID(&fact.sourceResultID),
		SampleSize:          int32(fact.sampleSize),
		Confidence:          confidence,
		ComputedAt:          timestamptz(computedAt),
	})
	return err
}

func ensureAnalyticsAggregate(aggregates map[analyticsProjectionKey]*analyticsAggregate, key analyticsProjectionKey) *analyticsAggregate {
	aggregate, exists := aggregates[key]
	if !exists {
		aggregate = &analyticsAggregate{key: key}
		aggregates[key] = aggregate
	}
	return aggregate
}

func (a *analyticsAggregate) apply(match analyticsMatch, outcome string, participant analyticsParticipant, opponentStrength float64, hasOpponentStrength bool) {
	a.matchesPlayed++
	switch outcome {
	case matchOutcomeWin:
		a.wins++
		if a.currentStreak > 0 {
			a.currentStreak++
		} else {
			a.currentStreak = 1
		}
	case matchOutcomeLoss:
		a.losses++
		if a.currentStreak < 0 {
			a.currentStreak--
		} else {
			a.currentStreak = -1
		}
	case matchOutcomeDraw:
		a.draws++
		a.currentStreak = 0
	}
	if participant.ratingEventFound {
		a.ratingMovement += participant.ratingDeltaMu
		a.ratingMovementSamples++
	}
	if hasOpponentStrength {
		a.opponentStrengthTotal += opponentStrength
		a.opponentStrengthSamples++
	}
	a.sourceMatchID = match.sourceMatchID
	a.sourceResultID = match.sourceResultID
	a.lastRecordedAt = match.recordedAt
}

func analyticsProjectionFacts(aggregates map[analyticsProjectionKey]*analyticsAggregate) []analyticsProjectionFact {
	facts := make([]analyticsProjectionFact, 0, len(aggregates)*7)
	keys := make([]analyticsProjectionKey, 0, len(aggregates))
	for key := range aggregates {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, compareAnalyticsProjectionKey)

	for _, key := range keys {
		aggregate := aggregates[key]
		facts = append(facts,
			aggregate.fact(analyticsStatMatchesPlayed, float64(aggregate.matchesPlayed), aggregate.matchesPlayed),
			aggregate.fact(analyticsStatWins, float64(aggregate.wins), aggregate.matchesPlayed),
			aggregate.fact(analyticsStatLosses, float64(aggregate.losses), aggregate.matchesPlayed),
			aggregate.fact(analyticsStatDraws, float64(aggregate.draws), aggregate.matchesPlayed),
			aggregate.fact(analyticsStatCurrentStreak, float64(aggregate.currentStreak), aggregate.matchesPlayed),
		)
		if aggregate.ratingMovementSamples > 0 {
			facts = append(facts, aggregate.fact(analyticsStatRatingMovement, aggregate.ratingMovement, aggregate.ratingMovementSamples))
		}
		if aggregate.opponentStrengthSamples > 0 {
			facts = append(facts, aggregate.fact(analyticsStatOpponentStrength, aggregate.opponentStrengthTotal/float64(aggregate.opponentStrengthSamples), aggregate.opponentStrengthSamples))
		}
	}

	facts = append(facts, analyticsTeamVsSoloFacts(aggregates)...)
	slices.SortFunc(facts, compareAnalyticsProjectionFact)
	return facts
}

func (a analyticsAggregate) fact(statType string, value float64, sampleSize int) analyticsProjectionFact {
	return analyticsProjectionFact{
		key:            a.key,
		statType:       statType,
		statValue:      value,
		sourceMatchID:  a.sourceMatchID,
		sourceResultID: a.sourceResultID,
		sampleSize:     sampleSize,
		confidence:     analyticsConfidence(sampleSize),
	}
}

func analyticsTeamVsSoloFacts(aggregates map[analyticsProjectionKey]*analyticsAggregate) []analyticsProjectionFact {
	facts := make([]analyticsProjectionFact, 0)
	for key, solo := range aggregates {
		if key.teamScope != analyticsTeamScopeSolo || key.modeKey != analyticsDimensionAll {
			continue
		}
		teamKey := key
		teamKey.teamScope = analyticsTeamScopeTeam
		team, exists := aggregates[teamKey]
		if !exists || solo.matchesPlayed == 0 || team.matchesPlayed == 0 {
			continue
		}

		projectionKey := key
		projectionKey.teamScope = analyticsDimensionAll
		source := laterAnalyticsAggregate(solo, team)
		sampleSize := solo.matchesPlayed + team.matchesPlayed
		teamWinRate := float64(team.wins) / float64(team.matchesPlayed)
		soloWinRate := float64(solo.wins) / float64(solo.matchesPlayed)
		facts = append(facts, analyticsProjectionFact{
			key:            projectionKey,
			statType:       analyticsStatTeamVsSoloDelta,
			statValue:      teamWinRate - soloWinRate,
			sourceMatchID:  source.sourceMatchID,
			sourceResultID: source.sourceResultID,
			sampleSize:     sampleSize,
			confidence:     analyticsConfidence(sampleSize),
		})
	}
	return facts
}

func analyticsProjectionKeys(userID uuid.UUID, sportKey string, facilityKey string, modeKey string, teamScope string) []analyticsProjectionKey {
	facilityKeys := []string{analyticsDimensionAll, facilityKey}
	modeKeys := []string{analyticsDimensionAll, modeKey}
	teamScopes := []string{analyticsDimensionAll, teamScope}
	keys := make([]analyticsProjectionKey, 0, len(facilityKeys)*len(modeKeys)*len(teamScopes))
	for _, facility := range facilityKeys {
		for _, mode := range modeKeys {
			for _, scope := range teamScopes {
				keys = append(keys, analyticsProjectionKey{
					userID:      userID,
					sportKey:    sportKey,
					facilityKey: facility,
					modeKey:     mode,
					teamScope:   scope,
				})
			}
		}
	}
	return keys
}

func averageOpponentRating(match analyticsMatch, teamID uuid.UUID) (float64, bool) {
	total := 0.0
	count := 0
	for _, side := range match.sides {
		if side.teamID == teamID {
			continue
		}
		for _, participant := range side.participants {
			if !participant.ratingEventFound {
				continue
			}
			total += participant.ratingMu
			count++
		}
	}
	if count == 0 {
		return 0, false
	}
	return total / float64(count), true
}

func analyticsTeamScope(participantsPerSide int) string {
	if participantsPerSide <= 1 {
		return analyticsTeamScopeSolo
	}
	return analyticsTeamScopeTeam
}

func analyticsConfidence(sampleSize int) float64 {
	if sampleSize <= 0 {
		return 0
	}
	confidence := float64(sampleSize) / 10.0
	if confidence > 1 {
		return 1
	}
	return confidence
}

func analyticsProjectionWatermark(matches []analyticsMatch) string {
	sourceMatchID, sourceResultID := analyticsProjectionSource(matches)
	if sourceResultID == nil {
		return "no_results"
	}
	last := matches[len(matches)-1]
	return last.recordedAt.UTC().Format(time.RFC3339Nano) + "#" + sourceMatchID.String() + "#" + sourceResultID.String()
}

func analyticsProjectionSource(matches []analyticsMatch) (*uuid.UUID, *uuid.UUID) {
	if len(matches) == 0 {
		return nil, nil
	}
	last := matches[len(matches)-1]
	return &last.sourceMatchID, &last.sourceResultID
}

func laterAnalyticsAggregate(left *analyticsAggregate, right *analyticsAggregate) *analyticsAggregate {
	if right.lastRecordedAt.After(left.lastRecordedAt) {
		return right
	}
	if right.lastRecordedAt.Equal(left.lastRecordedAt) && right.sourceResultID.String() > left.sourceResultID.String() {
		return right
	}
	return left
}

func compareAnalyticsProjectionFact(left analyticsProjectionFact, right analyticsProjectionFact) int {
	if compare := compareAnalyticsProjectionKey(left.key, right.key); compare != 0 {
		return compare
	}
	if left.statType < right.statType {
		return -1
	}
	if left.statType > right.statType {
		return 1
	}
	return 0
}

func compareAnalyticsProjectionKey(left analyticsProjectionKey, right analyticsProjectionKey) int {
	if compare := compareAnalyticsUUID(left.userID, right.userID); compare != 0 {
		return compare
	}
	if left.sportKey != right.sportKey {
		if left.sportKey < right.sportKey {
			return -1
		}
		return 1
	}
	if left.facilityKey != right.facilityKey {
		if left.facilityKey < right.facilityKey {
			return -1
		}
		return 1
	}
	if left.modeKey != right.modeKey {
		if left.modeKey < right.modeKey {
			return -1
		}
		return 1
	}
	if left.teamScope != right.teamScope {
		if left.teamScope < right.teamScope {
			return -1
		}
		return 1
	}
	return 0
}

func compareAnalyticsUUID(left uuid.UUID, right uuid.UUID) int {
	return slices.Compare(left[:], right[:])
}
