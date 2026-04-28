package competition

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ixxet/apollo/internal/rating"
	"github.com/ixxet/apollo/internal/store"
)

func (r *Repository) GetMatchResultByMatchID(ctx context.Context, matchID uuid.UUID) (*matchResultRecord, error) {
	row, err := store.New(r.db).GetCompetitionCanonicalMatchResultByMatchID(ctx, matchID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	result := buildMatchResultRecordValues(
		row.ID,
		row.CompetitionMatchID,
		row.RecordedByUserID,
		row.RecordedAt,
		row.ResultStatus,
		row.DisputeStatus,
		row.CorrectionID,
		row.SupersedesResultID,
		row.FinalizedAt,
		row.CorrectedAt,
	)
	return &result, nil
}

func (r *Repository) ListMatchResultsBySessionID(ctx context.Context, sessionID uuid.UUID) ([]matchResultSideRecord, error) {
	rows, err := store.New(r.db).ListCompetitionMatchResultRowsBySessionID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	results := make([]matchResultSideRecord, 0, len(rows))
	for _, row := range rows {
		results = append(results, matchResultSideRecord{
			CompetitionMatchResultID: row.CompetitionMatchResultID,
			CompetitionMatchID:       row.CompetitionMatchID,
			RecordedByUserID:         row.RecordedByUserID,
			RecordedAt:               row.RecordedAt.Time.UTC(),
			ResultStatus:             row.ResultStatus,
			DisputeStatus:            row.DisputeStatus,
			CorrectionID:             uuidFromPgtype(row.CorrectionID),
			SupersedesResultID:       uuidFromPgtype(row.SupersedesResultID),
			FinalizedAt:              timePtr(row.FinalizedAt),
			CorrectedAt:              timePtr(row.CorrectedAt),
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

func (r *Repository) ListMemberHistoryByUserID(ctx context.Context, userID uuid.UUID) ([]memberHistoryRowRecord, error) {
	rows, err := store.New(r.db).ListCompetitionMemberHistoryByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	history := make([]memberHistoryRowRecord, 0, len(rows))
	for _, row := range rows {
		history = append(history, memberHistoryRowRecord{
			CompetitionMatchID:  row.CompetitionMatchID,
			CompetitionResultID: row.CompetitionMatchResultID,
			DisplayName:         row.DisplayName,
			SportKey:            row.SportKey,
			FacilityKey:         row.FacilityKey,
			CompetitionMode:     row.CompetitionMode,
			SidesPerMatch:       int(row.SidesPerMatch),
			ParticipantsPerSide: int(row.ParticipantsPerSide),
			RecordedAt:          row.RecordedAt.Time.UTC(),
			Outcome:             row.Outcome,
		})
	}

	return history, nil
}

func (r *Repository) RecordMatchResult(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, match matchRecord, input RecordMatchResultInput, expectedResultVersion int, recordedAt time.Time) (matchResultRecord, error) {
	_ = sport

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return matchResultRecord{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	queries := store.New(tx)
	resultRow, err := queries.CreateCompetitionMatchResult(ctx, store.CreateCompetitionMatchResultParams{
		CompetitionMatchID: match.ID,
		RecordedByUserID:   actor.UserID,
		RecordedAt:         timestamptz(recordedAt),
		ResultStatus:       ResultStatusRecorded,
		DisputeStatus:      DisputeStatusNone,
	})
	if err != nil {
		return matchResultRecord{}, err
	}
	result := buildMatchResultRecordValues(
		resultRow.ID,
		resultRow.CompetitionMatchID,
		resultRow.RecordedByUserID,
		resultRow.RecordedAt,
		resultRow.ResultStatus,
		resultRow.DisputeStatus,
		resultRow.CorrectionID,
		resultRow.SupersedesResultID,
		resultRow.FinalizedAt,
		resultRow.CorrectedAt,
	)

	for _, side := range input.Sides {
		if _, err := queries.CreateCompetitionMatchResultSide(ctx, store.CreateCompetitionMatchResultSideParams{
			CompetitionMatchResultID: result.ID,
			CompetitionMatchID:       match.ID,
			SideIndex:                int32(side.SideIndex),
			CompetitionSessionTeamID: side.CompetitionSessionTeamID,
			Outcome:                  side.Outcome,
		}); err != nil {
			return matchResultRecord{}, err
		}
	}

	if _, err := queries.CompleteCompetitionMatchWithResult(ctx, store.CompleteCompetitionMatchWithResultParams{
		ID:                match.ID,
		CanonicalResultID: optionalUUID(&result.ID),
		ResultVersion:     int32(expectedResultVersion),
		UpdatedAt:         timestamptz(recordedAt),
	}); err != nil {
		return matchResultRecord{}, err
	}

	incompleteCount, err := queries.CountCompetitionIncompleteActiveMatchesBySessionID(ctx, session.ID)
	if err != nil {
		return matchResultRecord{}, err
	}
	if incompleteCount == 0 {
		if _, err := queries.CompleteCompetitionSession(ctx, store.CompleteCompetitionSessionParams{
			ID:          session.ID,
			OwnerUserID: session.OwnerUserID,
			UpdatedAt:   timestamptz(recordedAt),
		}); err != nil {
			return matchResultRecord{}, err
		}
	}

	if err := recordLifecycleEventTx(ctx, queries, newLifecycleEvent(actor, session.ID, &match.ID, &result.ID, "competition.result.recorded", "", result, recordedAt)); err != nil {
		return matchResultRecord{}, err
	}
	attribution := newStaffActionAttribution(actor, "competition_match.result_record", recordedAt)
	attribution.CompetitionSessionID = uuidPtr(session.ID)
	attribution.CompetitionMatchID = uuidPtr(match.ID)
	if err := recordStaffActionAttributionTx(ctx, queries, attribution); err != nil {
		return matchResultRecord{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return matchResultRecord{}, err
	}
	return result, nil
}

func (r *Repository) FinalizeMatchResult(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, match matchRecord, result matchResultRecord, expectedResultVersion int, finalizedAt time.Time) (matchResultRecord, error) {
	return r.transitionMatchResult(ctx, actor, session, sport, match, result, expectedResultVersion, ResultStatusFinalized, DisputeStatusNone, "competition.result.finalized", "competition_match.result_finalize", finalizedAt)
}

func (r *Repository) DisputeMatchResult(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, match matchRecord, result matchResultRecord, expectedResultVersion int, disputedAt time.Time) (matchResultRecord, error) {
	return r.transitionMatchResult(ctx, actor, session, sport, match, result, expectedResultVersion, ResultStatusDisputed, DisputeStatusDisputed, "competition.result.disputed", "competition_match.result_dispute", disputedAt)
}

func (r *Repository) VoidMatchResult(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, match matchRecord, result matchResultRecord, expectedResultVersion int, voidedAt time.Time) (matchResultRecord, error) {
	return r.transitionMatchResult(ctx, actor, session, sport, match, result, expectedResultVersion, ResultStatusVoided, DisputeStatusNone, "competition.result.voided", "competition_match.result_void", voidedAt)
}

func (r *Repository) transitionMatchResult(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, match matchRecord, result matchResultRecord, expectedResultVersion int, nextStatus string, nextDisputeStatus string, eventType string, attributionAction string, occurredAt time.Time) (matchResultRecord, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return matchResultRecord{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	queries := store.New(tx)
	updatedRow, err := queries.UpdateCompetitionMatchResultStatus(ctx, store.UpdateCompetitionMatchResultStatusParams{
		ID:             result.ID,
		ResultStatus:   nextStatus,
		DisputeStatus:  nextDisputeStatus,
		ResultStatus_2: result.ResultStatus,
		FinalizedAt:    timestamptz(occurredAt),
	})
	if err != nil {
		return matchResultRecord{}, err
	}
	updated := buildMatchResultRecordValues(
		updatedRow.ID,
		updatedRow.CompetitionMatchID,
		updatedRow.RecordedByUserID,
		updatedRow.RecordedAt,
		updatedRow.ResultStatus,
		updatedRow.DisputeStatus,
		updatedRow.CorrectionID,
		updatedRow.SupersedesResultID,
		updatedRow.FinalizedAt,
		updatedRow.CorrectedAt,
	)

	if _, err := queries.UpdateCompetitionMatchCanonicalResult(ctx, store.UpdateCompetitionMatchCanonicalResultParams{
		ID:                match.ID,
		CanonicalResultID: optionalUUID(&updated.ID),
		ResultVersion:     int32(expectedResultVersion),
		UpdatedAt:         timestamptz(occurredAt),
	}); err != nil {
		return matchResultRecord{}, err
	}

	if err := recomputeCompetitionRatingsTx(ctx, queries, sport.SportKey, occurredAt); err != nil {
		return matchResultRecord{}, err
	}
	if err := recordLifecycleEventTx(ctx, queries, newLifecycleEvent(actor, session.ID, &match.ID, &updated.ID, eventType, result.ResultStatus, updated, occurredAt)); err != nil {
		return matchResultRecord{}, err
	}
	attribution := newStaffActionAttribution(actor, attributionAction, occurredAt)
	attribution.CompetitionSessionID = uuidPtr(session.ID)
	attribution.CompetitionMatchID = uuidPtr(match.ID)
	if err := recordStaffActionAttributionTx(ctx, queries, attribution); err != nil {
		return matchResultRecord{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return matchResultRecord{}, err
	}
	return updated, nil
}

func (r *Repository) CorrectMatchResult(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, match matchRecord, result matchResultRecord, input RecordMatchResultInput, expectedResultVersion int, correctedAt time.Time) (matchResultRecord, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return matchResultRecord{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	queries := store.New(tx)
	correctionID := uuid.New()
	resultRow, err := queries.CreateCompetitionMatchResult(ctx, store.CreateCompetitionMatchResultParams{
		CompetitionMatchID: match.ID,
		RecordedByUserID:   actor.UserID,
		RecordedAt:         timestamptz(correctedAt),
		ResultStatus:       ResultStatusCorrected,
		DisputeStatus:      DisputeStatusNone,
		CorrectionID:       optionalUUID(&correctionID),
		SupersedesResultID: optionalUUID(&result.ID),
		FinalizedAt:        timestamptz(correctedAt),
		CorrectedAt:        timestamptz(correctedAt),
	})
	if err != nil {
		return matchResultRecord{}, err
	}
	corrected := buildMatchResultRecordValues(
		resultRow.ID,
		resultRow.CompetitionMatchID,
		resultRow.RecordedByUserID,
		resultRow.RecordedAt,
		resultRow.ResultStatus,
		resultRow.DisputeStatus,
		resultRow.CorrectionID,
		resultRow.SupersedesResultID,
		resultRow.FinalizedAt,
		resultRow.CorrectedAt,
	)

	for _, side := range input.Sides {
		if _, err := queries.CreateCompetitionMatchResultSide(ctx, store.CreateCompetitionMatchResultSideParams{
			CompetitionMatchResultID: corrected.ID,
			CompetitionMatchID:       match.ID,
			SideIndex:                int32(side.SideIndex),
			CompetitionSessionTeamID: side.CompetitionSessionTeamID,
			Outcome:                  side.Outcome,
		}); err != nil {
			return matchResultRecord{}, err
		}
	}

	if _, err := queries.UpdateCompetitionMatchCanonicalResult(ctx, store.UpdateCompetitionMatchCanonicalResultParams{
		ID:                match.ID,
		CanonicalResultID: optionalUUID(&corrected.ID),
		ResultVersion:     int32(expectedResultVersion),
		UpdatedAt:         timestamptz(correctedAt),
	}); err != nil {
		return matchResultRecord{}, err
	}

	if err := recomputeCompetitionRatingsTx(ctx, queries, sport.SportKey, correctedAt); err != nil {
		return matchResultRecord{}, err
	}
	if err := recordLifecycleEventTx(ctx, queries, newLifecycleEvent(actor, session.ID, &match.ID, &corrected.ID, "competition.result.corrected", result.ResultStatus, corrected, correctedAt)); err != nil {
		return matchResultRecord{}, err
	}
	attribution := newStaffActionAttribution(actor, "competition_match.result_correct", correctedAt)
	attribution.CompetitionSessionID = uuidPtr(session.ID)
	attribution.CompetitionMatchID = uuidPtr(match.ID)
	if err := recordStaffActionAttributionTx(ctx, queries, attribution); err != nil {
		return matchResultRecord{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return matchResultRecord{}, err
	}
	return corrected, nil
}

type lifecycleEventRecord struct {
	Actor                StaffActor
	CompetitionSessionID uuid.UUID
	CompetitionMatchID   *uuid.UUID
	ResultID             *uuid.UUID
	EventType            string
	PreviousResultStatus string
	ResultStatus         string
	DisputeStatus        string
	CorrectionID         *uuid.UUID
	SupersedesResultID   *uuid.UUID
	OccurredAt           time.Time
}

func newLifecycleEvent(actor StaffActor, sessionID uuid.UUID, matchID *uuid.UUID, resultID *uuid.UUID, eventType string, previousStatus string, result matchResultRecord, occurredAt time.Time) lifecycleEventRecord {
	return lifecycleEventRecord{
		Actor:                actor,
		CompetitionSessionID: sessionID,
		CompetitionMatchID:   matchID,
		ResultID:             resultID,
		EventType:            eventType,
		PreviousResultStatus: previousStatus,
		ResultStatus:         result.ResultStatus,
		DisputeStatus:        result.DisputeStatus,
		CorrectionID:         result.CorrectionID,
		SupersedesResultID:   result.SupersedesResultID,
		OccurredAt:           occurredAt.UTC(),
	}
}

func recordLifecycleEventTx(ctx context.Context, queries *store.Queries, event lifecycleEventRecord) error {
	actorRole := string(event.Actor.Role)
	capability := string(event.Actor.Capability)
	_, err := queries.CreateCompetitionLifecycleEvent(ctx, store.CreateCompetitionLifecycleEventParams{
		CompetitionSessionID:     event.CompetitionSessionID,
		CompetitionMatchID:       optionalUUID(event.CompetitionMatchID),
		CompetitionMatchResultID: optionalUUID(event.ResultID),
		EventType:                event.EventType,
		ActorUserID:              optionalUUID(&event.Actor.UserID),
		ActorRole:                optionalText(actorRole),
		ActorSessionID:           optionalUUID(&event.Actor.SessionID),
		Capability:               optionalText(capability),
		TrustedSurfaceKey:        optionalText(event.Actor.TrustedSurfaceKey),
		TrustedSurfaceLabel:      optionalText(event.Actor.TrustedSurfaceLabel),
		PreviousResultStatus:     optionalText(event.PreviousResultStatus),
		ResultStatus:             optionalText(event.ResultStatus),
		DisputeStatus:            optionalText(event.DisputeStatus),
		CorrectionID:             optionalUUID(event.CorrectionID),
		SupersedesResultID:       optionalUUID(event.SupersedesResultID),
		OccurredAt:               timestamptz(event.OccurredAt),
	})
	return err
}

func buildMatchResultRecordValues(id uuid.UUID, matchID uuid.UUID, recordedBy uuid.UUID, recordedAt pgtype.Timestamptz, status string, disputeStatus string, correctionID pgtype.UUID, supersedesResultID pgtype.UUID, finalizedAt pgtype.Timestamptz, correctedAt pgtype.Timestamptz) matchResultRecord {
	return matchResultRecord{
		ID:                 id,
		CompetitionMatchID: matchID,
		RecordedByUserID:   recordedBy,
		RecordedAt:         recordedAt.Time.UTC(),
		ResultStatus:       status,
		DisputeStatus:      disputeStatus,
		CorrectionID:       uuidFromPgtype(correctionID),
		SupersedesResultID: uuidFromPgtype(supersedesResultID),
		FinalizedAt:        timePtr(finalizedAt),
		CorrectedAt:        timePtr(correctedAt),
	}
}

func recomputeCompetitionRatingsTx(ctx context.Context, queries *store.Queries, sportKey string, updatedAt time.Time) error {
	rows, err := queries.ListCompetitionRatingParticipantsBySport(ctx, sportKey)
	if err != nil {
		return err
	}

	matches := make([]rating.Match, 0, len(rows))
	matchIndex := make(map[uuid.UUID]int, len(rows))
	sideIndexByMatch := make(map[uuid.UUID]map[uuid.UUID]int, len(rows))
	for _, row := range rows {
		index, exists := matchIndex[row.CompetitionMatchID]
		if !exists {
			index = len(matches)
			matchIndex[row.CompetitionMatchID] = index
			matches = append(matches, rating.Match{
				CompetitionMatchID: row.CompetitionMatchID,
				SourceResultID:     row.CompetitionMatchResultID,
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
			matches[index].Sides = append(matches[index].Sides, rating.Side{
				CompetitionSessionTeamID: row.CompetitionSessionTeamID,
				SideIndex:                int(row.SideIndex),
				Outcome:                  row.Outcome,
				UserIDs:                  make([]uuid.UUID, 0, 2),
			})
		}

		matches[index].Sides[sidePosition].UserIDs = append(matches[index].Sides[sidePosition].UserIDs, row.UserID)
	}

	projection := rating.RebuildLegacy(matches)
	comparison := rating.RebuildOpenSkillComparison(matches, projection)
	if err := recordRatingPolicySelectedEventTx(ctx, queries, sportKey, projection.Watermark, updatedAt); err != nil {
		return err
	}

	if _, err := queries.DeleteCompetitionMemberRatingsBySportKey(ctx, sportKey); err != nil {
		return err
	}
	if _, err := queries.DeleteCompetitionRatingComparisonsBySportKey(ctx, sportKey); err != nil {
		return err
	}

	eventIDsByState := make(map[ratingEventStateKey]uuid.UUID, len(projection.Events))
	for _, event := range projection.Events {
		eventID, err := recordLegacyRatingComputedEventTx(ctx, queries, sportKey, event)
		if err != nil {
			return err
		}
		eventIDsByState[ratingEventStateKey{
			modeKey:        event.ModeKey,
			userID:         event.UserID,
			sourceResultID: event.SourceResultID,
		}] = eventID
	}
	for _, fact := range comparison.Facts {
		if err := recordOpenSkillComparisonTx(ctx, queries, sportKey, fact, updatedAt); err != nil {
			return err
		}
	}

	for _, state := range projection.States {
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

		eventID, exists := eventIDsByState[ratingEventStateKey{
			modeKey:        state.ModeKey,
			userID:         state.UserID,
			sourceResultID: state.SourceResultID,
		}]
		if !exists {
			return errors.New("legacy rating event missing for projection state")
		}

		if _, err := queries.UpsertCompetitionMemberRating(ctx, store.UpsertCompetitionMemberRatingParams{
			UserID:              state.UserID,
			SportKey:            sportKey,
			ModeKey:             state.ModeKey,
			Mu:                  mu,
			Sigma:               sigma,
			MatchesPlayed:       int32(state.MatchesPlayed),
			LastPlayed:          lastPlayed,
			UpdatedAt:           timestamptz(updatedAt),
			RatingEngine:        rating.EngineLegacyEloLike,
			EngineVersion:       rating.EngineVersionLegacy,
			PolicyVersion:       rating.PolicyVersionLegacy,
			SourceResultID:      optionalUUID(&state.SourceResultID),
			RatingEventID:       optionalUUID(&eventID),
			ProjectionWatermark: projection.Watermark,
		}); err != nil {
			return err
		}
	}

	return recordRatingProjectionRebuiltEventTx(ctx, queries, sportKey, projection.Watermark, projectionSourceResultID(matches), updatedAt)
}

type ratingEventStateKey struct {
	modeKey        string
	userID         uuid.UUID
	sourceResultID uuid.UUID
}

func recordRatingPolicySelectedEventTx(ctx context.Context, queries *store.Queries, sportKey string, projectionWatermark string, occurredAt time.Time) error {
	_, err := queries.CreateCompetitionRatingEvent(ctx, store.CreateCompetitionRatingEventParams{
		EventType:           rating.EventPolicySelected,
		RatingEngine:        rating.EngineLegacyEloLike,
		EngineVersion:       rating.EngineVersionLegacy,
		PolicyVersion:       rating.PolicyVersionLegacy,
		SportKey:            sportKey,
		ProjectionWatermark: projectionWatermark,
		OccurredAt:          timestamptz(occurredAt),
	})
	return err
}

func recordRatingProjectionRebuiltEventTx(ctx context.Context, queries *store.Queries, sportKey string, projectionWatermark string, sourceResultID *uuid.UUID, occurredAt time.Time) error {
	_, err := queries.CreateCompetitionRatingEvent(ctx, store.CreateCompetitionRatingEventParams{
		EventType:           rating.EventProjectionRebuilt,
		RatingEngine:        rating.EngineLegacyEloLike,
		EngineVersion:       rating.EngineVersionLegacy,
		PolicyVersion:       rating.PolicyVersionLegacy,
		SportKey:            sportKey,
		SourceResultID:      optionalUUID(sourceResultID),
		ProjectionWatermark: projectionWatermark,
		OccurredAt:          timestamptz(occurredAt),
	})
	return err
}

func recordLegacyRatingComputedEventTx(ctx context.Context, queries *store.Queries, sportKey string, event rating.ComputedEvent) (uuid.UUID, error) {
	mu, err := numericFromFloat64(event.Mu)
	if err != nil {
		return uuid.Nil, err
	}
	sigma, err := numericFromFloat64(event.Sigma)
	if err != nil {
		return uuid.Nil, err
	}
	deltaMu, err := numericFromFloat64(event.DeltaMu)
	if err != nil {
		return uuid.Nil, err
	}
	deltaSigma, err := numericFromFloat64(event.DeltaSigma)
	if err != nil {
		return uuid.Nil, err
	}

	modeKey := event.ModeKey
	row, err := queries.UpsertCompetitionLegacyRatingEvent(ctx, store.UpsertCompetitionLegacyRatingEventParams{
		RatingEngine:        rating.EngineLegacyEloLike,
		EngineVersion:       rating.EngineVersionLegacy,
		PolicyVersion:       rating.PolicyVersionLegacy,
		SportKey:            sportKey,
		ModeKey:             &modeKey,
		UserID:              optionalUUID(&event.UserID),
		SourceResultID:      optionalUUID(&event.SourceResultID),
		Mu:                  mu,
		Sigma:               sigma,
		DeltaMu:             deltaMu,
		DeltaSigma:          deltaSigma,
		ProjectionWatermark: event.Watermark,
		OccurredAt:          timestamptz(event.OccurredAt),
	})
	if err != nil {
		return uuid.Nil, err
	}
	return row.ID, nil
}

func recordOpenSkillComparisonTx(ctx context.Context, queries *store.Queries, sportKey string, fact rating.ComparisonFact, updatedAt time.Time) error {
	legacyMu, err := numericFromFloat64(fact.LegacyMu)
	if err != nil {
		return err
	}
	legacySigma, err := numericFromFloat64(fact.LegacySigma)
	if err != nil {
		return err
	}
	openSkillMu, err := numericFromFloat64(fact.OpenSkillMu)
	if err != nil {
		return err
	}
	openSkillSigma, err := numericFromFloat64(fact.OpenSkillSigma)
	if err != nil {
		return err
	}
	deltaFromLegacy, err := numericFromFloat64(fact.DeltaFromLegacy)
	if err != nil {
		return err
	}
	acceptedDeltaBudget, err := numericFromFloat64(fact.AcceptedDeltaBudget)
	if err != nil {
		return err
	}

	modeKey := fact.ModeKey
	scenario := fact.ComparisonScenario
	eventParams := store.UpsertCompetitionOpenSkillComputedEventParams{
		RatingEngine:        rating.EngineOpenSkill,
		EngineVersion:       rating.EngineVersionOpenSkill,
		PolicyVersion:       rating.PolicyVersionOpenSkill,
		SportKey:            sportKey,
		ModeKey:             &modeKey,
		UserID:              optionalUUID(&fact.UserID),
		SourceResultID:      optionalUUID(&fact.SourceResultID),
		LegacyMu:            legacyMu,
		LegacySigma:         legacySigma,
		OpenskillMu:         openSkillMu,
		OpenskillSigma:      openSkillSigma,
		DeltaFromLegacy:     deltaFromLegacy,
		AcceptedDeltaBudget: acceptedDeltaBudget,
		ComparisonScenario:  &scenario,
		ProjectionWatermark: fact.Watermark,
		OccurredAt:          timestamptz(fact.OccurredAt),
	}
	if _, err := queries.UpsertCompetitionOpenSkillComputedEvent(ctx, eventParams); err != nil {
		return err
	}
	if fact.DeltaFlagged {
		if _, err := queries.UpsertCompetitionRatingDeltaFlaggedEvent(ctx, store.UpsertCompetitionRatingDeltaFlaggedEventParams(eventParams)); err != nil {
			return err
		}
	}

	_, err = queries.UpsertCompetitionRatingComparison(ctx, store.UpsertCompetitionRatingComparisonParams{
		SportKey:               sportKey,
		ModeKey:                fact.ModeKey,
		UserID:                 fact.UserID,
		SourceResultID:         fact.SourceResultID,
		LegacyRatingEngine:     rating.EngineLegacyEloLike,
		LegacyEngineVersion:    rating.EngineVersionLegacy,
		LegacyPolicyVersion:    rating.PolicyVersionLegacy,
		OpenskillRatingEngine:  rating.EngineOpenSkill,
		OpenskillEngineVersion: rating.EngineVersionOpenSkill,
		OpenskillPolicyVersion: rating.PolicyVersionOpenSkill,
		LegacyMu:               legacyMu,
		LegacySigma:            legacySigma,
		OpenskillMu:            openSkillMu,
		OpenskillSigma:         openSkillSigma,
		DeltaFromLegacy:        deltaFromLegacy,
		AcceptedDeltaBudget:    acceptedDeltaBudget,
		ComparisonScenario:     fact.ComparisonScenario,
		DeltaFlagged:           fact.DeltaFlagged,
		ProjectionWatermark:    fact.Watermark,
		OccurredAt:             timestamptz(fact.OccurredAt),
		UpdatedAt:              timestamptz(updatedAt),
	})
	return err
}

func projectionSourceResultID(matches []rating.Match) *uuid.UUID {
	if len(matches) == 0 {
		return nil
	}
	return &matches[len(matches)-1].SourceResultID
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
