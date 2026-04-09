package competition

import (
	"context"
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ixxet/apollo/internal/store"
	"slices"
)

const (
	initialRatingMu    = 25.0
	initialRatingSigma = 8.3333
	minRatingSigma     = 2.5
	ratingSigmaDecay   = 0.96
	ratingKFactor      = 4.0
	ratingScale        = 8.0
)

func (r *Repository) GetMatchResultByMatchID(ctx context.Context, matchID uuid.UUID) (*matchResultRecord, error) {
	row, err := store.New(r.db).GetCompetitionMatchResultByMatchID(ctx, matchID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	recordedAt := row.RecordedAt.Time.UTC()
	return &matchResultRecord{
		CompetitionMatchID: row.CompetitionMatchID,
		RecordedByUserID:   row.RecordedByUserID,
		RecordedAt:         recordedAt,
	}, nil
}

func (r *Repository) ListMatchResultsBySessionID(ctx context.Context, sessionID uuid.UUID) ([]matchResultSideRecord, error) {
	rows, err := store.New(r.db).ListCompetitionMatchResultRowsBySessionID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	results := make([]matchResultSideRecord, 0, len(rows))
	for _, row := range rows {
		results = append(results, matchResultSideRecord{
			CompetitionMatchID:       row.CompetitionMatchID,
			RecordedByUserID:         row.RecordedByUserID,
			RecordedAt:               row.RecordedAt.Time.UTC(),
			SideIndex:                int(row.SideIndex),
			CompetitionSessionTeamID: row.CompetitionSessionTeamID,
			Outcome:                  row.Outcome,
		})
	}

	return results, nil
}

func (r *Repository) ListMemberRatingsByUserID(ctx context.Context, userID uuid.UUID) ([]memberRatingRecord, error) {
	rows, err := store.New(r.db).ListCompetitionMemberRatingsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	ratings := make([]memberRatingRecord, 0, len(rows))
	for _, row := range rows {
		mu, err := float64FromNumeric(row.Mu)
		if err != nil {
			return nil, err
		}
		sigma, err := float64FromNumeric(row.Sigma)
		if err != nil {
			return nil, err
		}

		var lastPlayedAt *time.Time
		if row.LastPlayed.Valid {
			recordedAt := row.LastPlayed.Time.UTC()
			lastPlayedAt = &recordedAt
		}

		ratings = append(ratings, memberRatingRecord{
			UserID:        row.UserID,
			SportKey:      row.SportKey,
			ModeKey:       row.ModeKey,
			Mu:            mu,
			Sigma:         sigma,
			MatchesPlayed: int(row.MatchesPlayed),
			LastPlayedAt:  lastPlayedAt,
			UpdatedAt:     row.UpdatedAt.Time.UTC(),
		})
	}

	return ratings, nil
}

func (r *Repository) ListMemberStatRowsByUserID(ctx context.Context, userID uuid.UUID) ([]memberStatRowRecord, error) {
	rows, err := store.New(r.db).ListCompetitionMemberStatRowsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	stats := make([]memberStatRowRecord, 0, len(rows))
	for _, row := range rows {
		stats = append(stats, memberStatRowRecord{
			SportKey:            row.SportKey,
			CompetitionMode:     row.CompetitionMode,
			SidesPerMatch:       int(row.SidesPerMatch),
			ParticipantsPerSide: int(row.ParticipantsPerSide),
			RecordedAt:          row.RecordedAt.Time.UTC(),
			Outcome:             row.Outcome,
		})
	}

	return stats, nil
}

func (r *Repository) RecordMatchResult(ctx context.Context, ownerUserID uuid.UUID, session sessionRecord, sport SportConfig, match matchRecord, input RecordMatchResultInput, recordedAt time.Time) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	queries := store.New(tx)
	if _, err := queries.CreateCompetitionMatchResult(ctx, store.CreateCompetitionMatchResultParams{
		CompetitionMatchID: match.ID,
		RecordedByUserID:   ownerUserID,
		RecordedAt:         timestamptz(recordedAt),
	}); err != nil {
		return err
	}

	for _, side := range input.Sides {
		if _, err := queries.CreateCompetitionMatchResultSide(ctx, store.CreateCompetitionMatchResultSideParams{
			CompetitionMatchID:       match.ID,
			SideIndex:                int32(side.SideIndex),
			CompetitionSessionTeamID: side.CompetitionSessionTeamID,
			Outcome:                  side.Outcome,
		}); err != nil {
			return err
		}
	}

	if _, err := queries.CompleteCompetitionMatch(ctx, store.CompleteCompetitionMatchParams{
		ID:        match.ID,
		UpdatedAt: timestamptz(recordedAt),
	}); err != nil {
		return err
	}

	incompleteCount, err := queries.CountCompetitionIncompleteActiveMatchesBySessionID(ctx, session.ID)
	if err != nil {
		return err
	}
	if incompleteCount == 0 {
		if _, err := queries.CompleteCompetitionSession(ctx, store.CompleteCompetitionSessionParams{
			ID:          session.ID,
			OwnerUserID: ownerUserID,
			UpdatedAt:   timestamptz(recordedAt),
		}); err != nil {
			return err
		}
	}

	if err := recomputeCompetitionRatingsTx(ctx, queries, sport.SportKey, recordedAt); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

type ratingMatchRecord struct {
	CompetitionMatchID uuid.UUID
	ModeKey            string
	RecordedAt         time.Time
	Sides              []*ratingSideRecord
}

type ratingSideRecord struct {
	CompetitionSessionTeamID uuid.UUID
	SideIndex                int
	Outcome                  string
	UserIDs                  []uuid.UUID
}

type ratingState struct {
	Mu            float64
	Sigma         float64
	MatchesPlayed int
	LastPlayedAt  *time.Time
}

func recomputeCompetitionRatingsTx(ctx context.Context, queries *store.Queries, sportKey string, updatedAt time.Time) error {
	rows, err := queries.ListCompetitionRatingParticipantsBySport(ctx, sportKey)
	if err != nil {
		return err
	}

	matches := make([]ratingMatchRecord, 0, len(rows))
	matchIndex := make(map[uuid.UUID]int, len(rows))
	sideIndexByMatch := make(map[uuid.UUID]map[uuid.UUID]int, len(rows))
	for _, row := range rows {
		index, exists := matchIndex[row.CompetitionMatchID]
		if !exists {
			index = len(matches)
			matchIndex[row.CompetitionMatchID] = index
			matches = append(matches, ratingMatchRecord{
				CompetitionMatchID: row.CompetitionMatchID,
				ModeKey:            buildModeKey(row.CompetitionMode, int(row.SidesPerMatch), int(row.ParticipantsPerSide)),
				RecordedAt:         row.RecordedAt.Time.UTC(),
				Sides:              nil,
			})
			sideIndexByMatch[row.CompetitionMatchID] = make(map[uuid.UUID]int)
		}

		sideMap := sideIndexByMatch[row.CompetitionMatchID]
		sidePosition, sideExists := sideMap[row.CompetitionSessionTeamID]
		if !sideExists {
			sidePosition = len(matches[index].Sides)
			sideMap[row.CompetitionSessionTeamID] = sidePosition
			matches[index].Sides = append(matches[index].Sides, &ratingSideRecord{
				CompetitionSessionTeamID: row.CompetitionSessionTeamID,
				SideIndex:                int(row.SideIndex),
				Outcome:                  row.Outcome,
				UserIDs:                  make([]uuid.UUID, 0, 2),
			})
		}

		matches[index].Sides[sidePosition].UserIDs = append(matches[index].Sides[sidePosition].UserIDs, row.UserID)
	}

	if _, err := queries.DeleteCompetitionMemberRatingsBySportKey(ctx, sportKey); err != nil {
		return err
	}
	if len(matches) == 0 {
		return nil
	}

	ratingsByMode := make(map[string]map[uuid.UUID]*ratingState, len(matches))
	for _, match := range matches {
		states, exists := ratingsByMode[match.ModeKey]
		if !exists {
			states = make(map[uuid.UUID]*ratingState)
			ratingsByMode[match.ModeKey] = states
		}
		applyRatingMatch(states, match)
	}

	modeKeys := make([]string, 0, len(ratingsByMode))
	for modeKey := range ratingsByMode {
		modeKeys = append(modeKeys, modeKey)
	}
	slices.Sort(modeKeys)

	for _, modeKey := range modeKeys {
		userIDs := make([]uuid.UUID, 0, len(ratingsByMode[modeKey]))
		for userID := range ratingsByMode[modeKey] {
			userIDs = append(userIDs, userID)
		}
		slices.SortFunc(userIDs, func(left, right uuid.UUID) int {
			return slices.Compare(left[:], right[:])
		})

		for _, userID := range userIDs {
			state := ratingsByMode[modeKey][userID]
			mu, err := numericFromFloat64(state.Mu)
			if err != nil {
				return err
			}
			sigma, err := numericFromFloat64(state.Sigma)
			if err != nil {
				return err
			}

			var lastPlayed pgtype.Timestamptz
			if state.LastPlayedAt != nil {
				lastPlayed = timestamptz(*state.LastPlayedAt)
			}

			if _, err := queries.UpsertCompetitionMemberRating(ctx, store.UpsertCompetitionMemberRatingParams{
				UserID:        userID,
				SportKey:      sportKey,
				ModeKey:       modeKey,
				Mu:            mu,
				Sigma:         sigma,
				MatchesPlayed: int32(state.MatchesPlayed),
				LastPlayed:    lastPlayed,
				UpdatedAt:     timestamptz(updatedAt),
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func applyRatingMatch(states map[uuid.UUID]*ratingState, match ratingMatchRecord) {
	teamMus := make(map[uuid.UUID]float64, len(match.Sides))
	for _, side := range match.Sides {
		totalMu := 0.0
		for _, userID := range side.UserIDs {
			state := ensureRatingState(states, userID)
			totalMu += state.Mu
		}
		teamMus[side.CompetitionSessionTeamID] = totalMu / float64(len(side.UserIDs))
	}

	deltasByTeam := make(map[uuid.UUID]float64, len(match.Sides))
	for _, side := range match.Sides {
		expectedTotal := 0.0
		opponentCount := 0
		for _, opponent := range match.Sides {
			if opponent.CompetitionSessionTeamID == side.CompetitionSessionTeamID {
				continue
			}
			expectedTotal += logisticExpectation(teamMus[side.CompetitionSessionTeamID], teamMus[opponent.CompetitionSessionTeamID])
			opponentCount++
		}
		if opponentCount == 0 {
			continue
		}

		expected := expectedTotal / float64(opponentCount)
		actual := 0.0
		switch side.Outcome {
		case matchOutcomeWin:
			actual = 1.0
		case matchOutcomeDraw:
			actual = 0.5
		case matchOutcomeLoss:
			actual = 0.0
		}
		deltasByTeam[side.CompetitionSessionTeamID] = ratingKFactor * (actual - expected)
	}

	for _, side := range match.Sides {
		delta := deltasByTeam[side.CompetitionSessionTeamID]
		for _, userID := range side.UserIDs {
			state := ensureRatingState(states, userID)
			state.Mu += delta
			state.Sigma = math.Max(minRatingSigma, state.Sigma*ratingSigmaDecay)
			state.MatchesPlayed++
			recordedAt := match.RecordedAt.UTC()
			state.LastPlayedAt = &recordedAt
		}
	}
}

func ensureRatingState(states map[uuid.UUID]*ratingState, userID uuid.UUID) *ratingState {
	state, exists := states[userID]
	if !exists {
		state = &ratingState{
			Mu:    initialRatingMu,
			Sigma: initialRatingSigma,
		}
		states[userID] = state
	}
	return state
}

func logisticExpectation(leftMu float64, rightMu float64) float64 {
	return 1.0 / (1.0 + math.Exp((rightMu-leftMu)/ratingScale))
}

func float64FromNumeric(value pgtype.Numeric) (float64, error) {
	floatValue, err := value.Float64Value()
	if err != nil {
		return 0, err
	}
	if !floatValue.Valid {
		return 0, nil
	}
	return floatValue.Float64, nil
}

func numericFromFloat64(value float64) (pgtype.Numeric, error) {
	var numeric pgtype.Numeric
	if err := numeric.Scan(strconv.FormatFloat(value, 'f', 4, 64)); err != nil {
		return pgtype.Numeric{}, err
	}
	return numeric, nil
}
