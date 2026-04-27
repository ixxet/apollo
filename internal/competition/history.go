package competition

import (
	"context"
	"errors"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	matchOutcomeWin  = "win"
	matchOutcomeLoss = "loss"
	matchOutcomeDraw = "draw"
)

func (s *Service) RecordMatchResult(ctx context.Context, actor StaffActor, sessionID uuid.UUID, matchID uuid.UUID, input RecordMatchResultInput) (Session, error) {
	if input.ExpectedResultVersion < 0 {
		return Session{}, ErrMatchResultVersion
	}

	session, err := s.repository.GetSessionByID(ctx, sessionID)
	if err != nil {
		return Session{}, err
	}
	if session == nil {
		return Session{}, ErrSessionNotFound
	}
	if session.Status == SessionStatusArchived {
		return Session{}, ErrSessionArchived
	}
	if session.Status != SessionStatusInProgress {
		return Session{}, ErrInvalidSessionTransition
	}

	match, err := s.repository.GetMatchByID(ctx, matchID)
	if err != nil {
		return Session{}, err
	}
	if match == nil || match.SessionID != session.ID {
		return Session{}, ErrMatchNotFound
	}
	if match.Status == MatchStatusArchived {
		return Session{}, ErrMatchArchived
	}
	if match.Status != MatchStatusInProgress {
		return Session{}, ErrMatchNotInProgress
	}
	if match.ResultVersion != input.ExpectedResultVersion {
		return Session{}, ErrMatchResultStateStale
	}

	recorded, err := s.repository.GetMatchResultByMatchID(ctx, match.ID)
	if err != nil {
		return Session{}, err
	}
	if recorded != nil {
		return Session{}, ErrMatchResultRecorded
	}

	sport, err := s.repository.GetSportConfig(ctx, session.SportKey)
	if err != nil {
		return Session{}, err
	}
	if sport == nil {
		return Session{}, ErrSportNotFound
	}

	sideSlots, err := s.repository.ListMatchSideSlotsBySessionID(ctx, session.ID)
	if err != nil {
		return Session{}, err
	}
	expectedSlots := make([]matchSideSlotRecord, 0, len(input.Sides))
	for _, sideSlot := range sideSlots {
		if sideSlot.MatchID == match.ID {
			expectedSlots = append(expectedSlots, sideSlot)
		}
	}
	if err := validateMatchResultInput(input.Sides, expectedSlots); err != nil {
		return Session{}, err
	}

	if _, err := s.repository.RecordMatchResult(ctx, actor, *session, *sport, *match, input, input.ExpectedResultVersion, s.now().UTC()); err != nil {
		switch {
		case isUniqueViolation(err):
			return Session{}, ErrMatchResultRecorded
		case errors.Is(err, pgx.ErrNoRows):
			return Session{}, ErrMatchResultStateStale
		default:
			return Session{}, err
		}
	}

	refreshed, err := s.repository.GetSessionByID(ctx, session.ID)
	if err != nil {
		return Session{}, err
	}
	if refreshed == nil {
		return Session{}, ErrSessionNotFound
	}

	return s.loadSessionDetail(ctx, *refreshed)
}

func (s *Service) FinalizeMatchResult(ctx context.Context, actor StaffActor, sessionID uuid.UUID, matchID uuid.UUID, expectedResultVersion int) (Session, error) {
	session, sport, match, result, err := s.loadResultMutationContext(ctx, sessionID, matchID, expectedResultVersion)
	if err != nil {
		return Session{}, err
	}
	if result == nil {
		return Session{}, ErrMatchResultNotFound
	}
	if result.ResultStatus == ResultStatusVoided {
		return Session{}, ErrMatchResultVoided
	}
	if result.ResultStatus != ResultStatusRecorded && result.ResultStatus != ResultStatusDisputed {
		return Session{}, ErrMatchResultNotRecorded
	}

	if _, err := s.repository.FinalizeMatchResult(ctx, actor, session, sport, match, *result, expectedResultVersion, s.now().UTC()); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrMatchResultStateStale
		}
		return Session{}, err
	}

	refreshed, err := s.repository.GetSessionByID(ctx, session.ID)
	if err != nil {
		return Session{}, err
	}
	if refreshed == nil {
		return Session{}, ErrSessionNotFound
	}
	return s.loadSessionDetail(ctx, *refreshed)
}

func (s *Service) DisputeMatchResult(ctx context.Context, actor StaffActor, sessionID uuid.UUID, matchID uuid.UUID, expectedResultVersion int) (Session, error) {
	session, sport, match, result, err := s.loadResultMutationContext(ctx, sessionID, matchID, expectedResultVersion)
	if err != nil {
		return Session{}, err
	}
	if result == nil {
		return Session{}, ErrMatchResultNotFound
	}
	if result.ResultStatus == ResultStatusVoided {
		return Session{}, ErrMatchResultVoided
	}
	if result.ResultStatus == ResultStatusDisputed {
		return Session{}, ErrMatchResultNotFinal
	}

	if _, err := s.repository.DisputeMatchResult(ctx, actor, session, sport, match, *result, expectedResultVersion, s.now().UTC()); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrMatchResultStateStale
		}
		return Session{}, err
	}

	refreshed, err := s.repository.GetSessionByID(ctx, session.ID)
	if err != nil {
		return Session{}, err
	}
	if refreshed == nil {
		return Session{}, ErrSessionNotFound
	}
	return s.loadSessionDetail(ctx, *refreshed)
}

func (s *Service) CorrectMatchResult(ctx context.Context, actor StaffActor, sessionID uuid.UUID, matchID uuid.UUID, input RecordMatchResultInput) (Session, error) {
	if input.ExpectedResultVersion < 0 {
		return Session{}, ErrMatchResultVersion
	}

	session, sport, match, result, err := s.loadResultMutationContext(ctx, sessionID, matchID, input.ExpectedResultVersion)
	if err != nil {
		return Session{}, err
	}
	if result == nil {
		return Session{}, ErrMatchResultNotFound
	}
	if result.ResultStatus == ResultStatusVoided {
		return Session{}, ErrMatchResultVoided
	}

	sideSlots, err := s.repository.ListMatchSideSlotsBySessionID(ctx, session.ID)
	if err != nil {
		return Session{}, err
	}
	expectedSlots := make([]matchSideSlotRecord, 0, len(input.Sides))
	for _, sideSlot := range sideSlots {
		if sideSlot.MatchID == match.ID {
			expectedSlots = append(expectedSlots, sideSlot)
		}
	}
	if err := validateMatchResultInput(input.Sides, expectedSlots); err != nil {
		return Session{}, err
	}

	if _, err := s.repository.CorrectMatchResult(ctx, actor, session, sport, match, *result, input, input.ExpectedResultVersion, s.now().UTC()); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrMatchResultStateStale
		}
		return Session{}, err
	}

	refreshed, err := s.repository.GetSessionByID(ctx, session.ID)
	if err != nil {
		return Session{}, err
	}
	if refreshed == nil {
		return Session{}, ErrSessionNotFound
	}
	return s.loadSessionDetail(ctx, *refreshed)
}

func (s *Service) VoidMatchResult(ctx context.Context, actor StaffActor, sessionID uuid.UUID, matchID uuid.UUID, expectedResultVersion int) (Session, error) {
	session, sport, match, result, err := s.loadResultMutationContext(ctx, sessionID, matchID, expectedResultVersion)
	if err != nil {
		return Session{}, err
	}
	if result == nil {
		return Session{}, ErrMatchResultNotFound
	}
	if result.ResultStatus == ResultStatusVoided {
		return Session{}, ErrMatchResultVoided
	}

	if _, err := s.repository.VoidMatchResult(ctx, actor, session, sport, match, *result, expectedResultVersion, s.now().UTC()); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrMatchResultStateStale
		}
		return Session{}, err
	}

	refreshed, err := s.repository.GetSessionByID(ctx, session.ID)
	if err != nil {
		return Session{}, err
	}
	if refreshed == nil {
		return Session{}, ErrSessionNotFound
	}
	return s.loadSessionDetail(ctx, *refreshed)
}

func (s *Service) loadResultMutationContext(ctx context.Context, sessionID uuid.UUID, matchID uuid.UUID, expectedResultVersion int) (sessionRecord, SportConfig, matchRecord, *matchResultRecord, error) {
	if expectedResultVersion < 0 {
		return sessionRecord{}, SportConfig{}, matchRecord{}, nil, ErrMatchResultVersion
	}

	session, err := s.repository.GetSessionByID(ctx, sessionID)
	if err != nil {
		return sessionRecord{}, SportConfig{}, matchRecord{}, nil, err
	}
	if session == nil {
		return sessionRecord{}, SportConfig{}, matchRecord{}, nil, ErrSessionNotFound
	}
	if session.Status == SessionStatusArchived {
		return sessionRecord{}, SportConfig{}, matchRecord{}, nil, ErrSessionArchived
	}

	match, err := s.repository.GetMatchByID(ctx, matchID)
	if err != nil {
		return sessionRecord{}, SportConfig{}, matchRecord{}, nil, err
	}
	if match == nil || match.SessionID != session.ID {
		return sessionRecord{}, SportConfig{}, matchRecord{}, nil, ErrMatchNotFound
	}
	if match.Status == MatchStatusArchived {
		return sessionRecord{}, SportConfig{}, matchRecord{}, nil, ErrMatchArchived
	}
	if match.Status != MatchStatusCompleted {
		return sessionRecord{}, SportConfig{}, matchRecord{}, nil, ErrMatchResultNotFound
	}
	if match.ResultVersion != expectedResultVersion {
		return sessionRecord{}, SportConfig{}, matchRecord{}, nil, ErrMatchResultStateStale
	}

	sport, err := s.repository.GetSportConfig(ctx, session.SportKey)
	if err != nil {
		return sessionRecord{}, SportConfig{}, matchRecord{}, nil, err
	}
	if sport == nil {
		return sessionRecord{}, SportConfig{}, matchRecord{}, nil, ErrSportNotFound
	}

	result, err := s.repository.GetMatchResultByMatchID(ctx, match.ID)
	if err != nil {
		return sessionRecord{}, SportConfig{}, matchRecord{}, nil, err
	}
	if result != nil && match.CanonicalResultID != nil && result.ID != *match.CanonicalResultID {
		return sessionRecord{}, SportConfig{}, matchRecord{}, nil, ErrMatchResultNotCanonical
	}

	return *session, *sport, *match, result, nil
}

func (s *Service) ListMemberStats(ctx context.Context, userID uuid.UUID) ([]MemberStat, error) {
	rows, err := s.repository.ListMemberStatRowsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	ratings, err := s.repository.ListMemberRatingsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	type statKey struct {
		sportKey string
		modeKey  string
	}

	statsByKey := make(map[statKey]*MemberStat, len(rows))
	for _, row := range rows {
		modeKey := buildModeKey(row.CompetitionMode, row.SidesPerMatch, row.ParticipantsPerSide)
		key := statKey{sportKey: row.SportKey, modeKey: modeKey}
		stat, exists := statsByKey[key]
		if !exists {
			stat = &MemberStat{
				UserID:   userID,
				SportKey: row.SportKey,
				ModeKey:  modeKey,
			}
			statsByKey[key] = stat
		}

		stat.MatchesPlayed++
		switch row.Outcome {
		case matchOutcomeWin:
			stat.Wins++
		case matchOutcomeLoss:
			stat.Losses++
		case matchOutcomeDraw:
			stat.Draws++
		}
		recordedAt := row.RecordedAt.UTC()
		if stat.LastPlayedAt == nil || recordedAt.After(*stat.LastPlayedAt) {
			stat.LastPlayedAt = &recordedAt
		}
	}

	for _, rating := range ratings {
		key := statKey{sportKey: rating.SportKey, modeKey: rating.ModeKey}
		stat, exists := statsByKey[key]
		if !exists {
			stat = &MemberStat{
				UserID:   userID,
				SportKey: rating.SportKey,
				ModeKey:  rating.ModeKey,
			}
			statsByKey[key] = stat
		}
		stat.CurrentRatingMu = rating.Mu
		stat.CurrentRatingSigma = rating.Sigma
		if rating.LastPlayedAt != nil && (stat.LastPlayedAt == nil || rating.LastPlayedAt.After(*stat.LastPlayedAt)) {
			lastPlayedAt := rating.LastPlayedAt.UTC()
			stat.LastPlayedAt = &lastPlayedAt
		}
	}

	stats := make([]MemberStat, 0, len(statsByKey))
	for _, stat := range statsByKey {
		stats = append(stats, *stat)
	}

	slices.SortFunc(stats, func(left, right MemberStat) int {
		if left.SportKey != right.SportKey {
			if left.SportKey < right.SportKey {
				return -1
			}
			return 1
		}
		if left.ModeKey != right.ModeKey {
			if left.ModeKey < right.ModeKey {
				return -1
			}
			return 1
		}
		return 0
	})

	return stats, nil
}

func (s *Service) ListMemberHistory(ctx context.Context, userID uuid.UUID) ([]MemberHistoryEntry, error) {
	rows, err := s.repository.ListMemberHistoryByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	history := make([]MemberHistoryEntry, 0, len(rows))
	for _, row := range rows {
		history = append(history, MemberHistoryEntry{
			CompetitionMatchID: row.CompetitionMatchID,
			SourceResultID:     row.CompetitionResultID,
			CanonicalResultID:  row.CompetitionResultID,
			DisplayName:        row.DisplayName,
			SportKey:           row.SportKey,
			ModeKey:            buildModeKey(row.CompetitionMode, row.SidesPerMatch, row.ParticipantsPerSide),
			FacilityKey:        row.FacilityKey,
			RecordedAt:         row.RecordedAt.UTC(),
			Outcome:            row.Outcome,
		})
	}

	return history, nil
}

func validateMatchResultInput(input []MatchResultSideInput, expectedSlots []matchSideSlotRecord) error {
	if len(input) != len(expectedSlots) {
		return ErrMatchResultSideCount
	}

	expectedByIndex := make(map[int]uuid.UUID, len(expectedSlots))
	orderedExpected := make([]int, 0, len(expectedSlots))
	for _, sideSlot := range expectedSlots {
		expectedByIndex[sideSlot.SideIndex] = sideSlot.TeamID
		orderedExpected = append(orderedExpected, sideSlot.SideIndex)
	}
	slices.Sort(orderedExpected)

	inputByIndex := make(map[int]MatchResultSideInput, len(input))
	orderedInput := make([]int, 0, len(input))
	for _, side := range input {
		if side.SideIndex <= 0 {
			return ErrMatchResultSideIndex
		}
		if _, exists := inputByIndex[side.SideIndex]; exists {
			return ErrMatchResultSideIndex
		}
		if !isValidMatchOutcome(side.Outcome) {
			return ErrMatchResultOutcome
		}
		inputByIndex[side.SideIndex] = side
		orderedInput = append(orderedInput, side.SideIndex)
	}
	slices.Sort(orderedInput)
	if !slices.Equal(orderedInput, orderedExpected) {
		return ErrMatchResultSideIndex
	}

	winCount := 0
	drawCount := 0
	for _, sideIndex := range orderedExpected {
		expectedTeamID, exists := expectedByIndex[sideIndex]
		if !exists {
			return ErrMatchResultTeamMismatch
		}
		inputSide := inputByIndex[sideIndex]
		if inputSide.CompetitionSessionTeamID != expectedTeamID {
			return ErrMatchResultTeamMismatch
		}
		switch inputSide.Outcome {
		case matchOutcomeWin:
			winCount++
		case matchOutcomeDraw:
			drawCount++
		}
	}

	switch {
	case drawCount == len(input):
		return nil
	case drawCount > 0:
		return ErrMatchResultShape
	case winCount != 1:
		return ErrMatchResultShape
	default:
		return nil
	}
}

func isValidMatchOutcome(value string) bool {
	switch value {
	case matchOutcomeWin, matchOutcomeLoss, matchOutcomeDraw:
		return true
	default:
		return false
	}
}

func buildMatchResults(rows []matchResultSideRecord) map[uuid.UUID]*MatchResult {
	results := make(map[uuid.UUID]*MatchResult, len(rows))
	for _, row := range rows {
		result, exists := results[row.CompetitionMatchID]
		if !exists {
			recordedAt := row.RecordedAt.UTC()
			canonicalResultID := row.CompetitionMatchResultID
			result = &MatchResult{
				ID:                 row.CompetitionMatchResultID,
				CompetitionMatchID: row.CompetitionMatchID,
				CanonicalResultID:  canonicalResultID,
				ResultStatus:       row.ResultStatus,
				DisputeStatus:      row.DisputeStatus,
				CorrectionID:       row.CorrectionID,
				SupersedesResultID: row.SupersedesResultID,
				RecordedByUserID:   row.RecordedByUserID,
				RecordedAt:         recordedAt,
				FinalizedAt:        row.FinalizedAt,
				CorrectedAt:        row.CorrectedAt,
				Sides:              make([]MatchResultSide, 0, 2),
			}
			results[row.CompetitionMatchID] = result
		}
		result.Sides = append(result.Sides, MatchResultSide{
			SideIndex:                row.SideIndex,
			CompetitionSessionTeamID: row.CompetitionSessionTeamID,
			Outcome:                  row.Outcome,
		})
	}

	for _, result := range results {
		slices.SortFunc(result.Sides, func(left, right MatchResultSide) int {
			if left.SideIndex < right.SideIndex {
				return -1
			}
			if left.SideIndex > right.SideIndex {
				return 1
			}
			return 0
		})
	}

	return results
}

func buildStandings(sessionID uuid.UUID, teams []Team, results map[uuid.UUID]*MatchResult) []Standing {
	standings := make([]Standing, 0, len(teams))
	indexByTeamID := make(map[uuid.UUID]int, len(teams))
	for _, team := range teams {
		indexByTeamID[team.ID] = len(standings)
		standings = append(standings, Standing{
			CompetitionSessionID:     sessionID,
			CompetitionSessionTeamID: team.ID,
			SideIndex:                team.SideIndex,
		})
	}

	for _, result := range results {
		for _, side := range result.Sides {
			index, exists := indexByTeamID[side.CompetitionSessionTeamID]
			if !exists {
				continue
			}
			standings[index].MatchesPlayed++
			switch side.Outcome {
			case matchOutcomeWin:
				standings[index].Wins++
			case matchOutcomeLoss:
				standings[index].Losses++
			case matchOutcomeDraw:
				standings[index].Draws++
			}
		}
	}

	slices.SortFunc(standings, func(left, right Standing) int {
		switch {
		case left.Wins != right.Wins:
			if left.Wins > right.Wins {
				return -1
			}
			return 1
		case left.Losses != right.Losses:
			if left.Losses < right.Losses {
				return -1
			}
			return 1
		case left.Draws != right.Draws:
			if left.Draws > right.Draws {
				return -1
			}
			return 1
		case left.SideIndex < right.SideIndex:
			return -1
		case left.SideIndex > right.SideIndex:
			return 1
		default:
			return 0
		}
	})

	for index := range standings {
		standings[index].Rank = index + 1
	}

	return standings
}
