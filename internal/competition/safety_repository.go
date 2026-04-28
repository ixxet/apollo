package competition

import (
	"context"
	"time"

	"github.com/ixxet/apollo/internal/store"
)

func (r *Repository) CreateSafetyReport(ctx context.Context, actor StaffActor, input normalizedSafetyReportInput, occurredAt time.Time) (safetyReportRecord, safetyEventRecord, error) {
	result, err := withQueriesTx(ctx, r.db, func(queries *store.Queries) (struct {
		report safetyReportRecord
		event  safetyEventRecord
	}, error) {
		reportRow, err := queries.CreateCompetitionSafetyReport(ctx, store.CreateCompetitionSafetyReportParams{
			CompetitionSessionID:     optionalUUID(input.CompetitionSessionID),
			CompetitionMatchID:       optionalUUID(input.CompetitionMatchID),
			CompetitionSessionTeamID: optionalUUID(input.CompetitionSessionTeamID),
			CompetitionTournamentID:  optionalUUID(input.CompetitionTournamentID),
			ReporterUserID:           input.ReporterUserID,
			SubjectUserID:            optionalUUID(input.SubjectUserID),
			TargetType:               input.TargetType,
			TargetID:                 input.TargetID,
			ReasonCode:               input.ReasonCode,
			Note:                     optionalText(input.Note),
			ActorUserID:              actor.UserID,
			ActorRole:                string(actor.Role),
			ActorSessionID:           actor.SessionID,
			Capability:               string(actor.Capability),
			TrustedSurfaceKey:        actor.TrustedSurfaceKey,
			TrustedSurfaceLabel:      optionalText(actor.TrustedSurfaceLabel),
			OccurredAt:               timestamptz(occurredAt),
		})
		if err != nil {
			return struct {
				report safetyReportRecord
				event  safetyEventRecord
			}{}, err
		}
		report := safetyReportRecordFromStore(reportRow)
		eventRow, err := queries.CreateCompetitionSafetyEvent(ctx, store.CreateCompetitionSafetyEventParams{
			EventType:                safetyReportEventRecorded,
			CompetitionSessionID:     optionalUUID(input.CompetitionSessionID),
			CompetitionMatchID:       optionalUUID(input.CompetitionMatchID),
			CompetitionSessionTeamID: optionalUUID(input.CompetitionSessionTeamID),
			CompetitionTournamentID:  optionalUUID(input.CompetitionTournamentID),
			SafetyReportID:           optionalUUID(&report.ID),
			ReporterUserID:           optionalUUID(&input.ReporterUserID),
			SubjectUserID:            optionalUUID(input.SubjectUserID),
			TargetType:               optionalText(input.TargetType),
			TargetID:                 optionalUUID(&input.TargetID),
			ReasonCode:               optionalText(input.ReasonCode),
			ActorUserID:              actor.UserID,
			ActorRole:                string(actor.Role),
			ActorSessionID:           actor.SessionID,
			Capability:               string(actor.Capability),
			TrustedSurfaceKey:        actor.TrustedSurfaceKey,
			TrustedSurfaceLabel:      optionalText(actor.TrustedSurfaceLabel),
			OccurredAt:               timestamptz(occurredAt),
		})
		if err != nil {
			return struct {
				report safetyReportRecord
				event  safetyEventRecord
			}{}, err
		}
		return struct {
			report safetyReportRecord
			event  safetyEventRecord
		}{
			report: report,
			event:  safetyEventRecordFromStore(eventRow),
		}, nil
	})
	if err != nil {
		return safetyReportRecord{}, safetyEventRecord{}, err
	}
	return result.report, result.event, nil
}

func (r *Repository) CreateSafetyBlock(ctx context.Context, actor StaffActor, input normalizedSafetyBlockInput, occurredAt time.Time) (safetyBlockRecord, safetyEventRecord, error) {
	result, err := withQueriesTx(ctx, r.db, func(queries *store.Queries) (struct {
		block safetyBlockRecord
		event safetyEventRecord
	}, error) {
		blockRow, err := queries.CreateCompetitionSafetyBlock(ctx, store.CreateCompetitionSafetyBlockParams{
			CompetitionSessionID: input.CompetitionSessionID,
			CompetitionMatchID:   optionalUUID(input.CompetitionMatchID),
			BlockerUserID:        input.BlockerUserID,
			BlockedUserID:        input.BlockedUserID,
			ReasonCode:           input.ReasonCode,
			ActorUserID:          actor.UserID,
			ActorRole:            string(actor.Role),
			ActorSessionID:       actor.SessionID,
			Capability:           string(actor.Capability),
			TrustedSurfaceKey:    actor.TrustedSurfaceKey,
			TrustedSurfaceLabel:  optionalText(actor.TrustedSurfaceLabel),
			OccurredAt:           timestamptz(occurredAt),
		})
		if err != nil {
			return struct {
				block safetyBlockRecord
				event safetyEventRecord
			}{}, err
		}
		block := safetyBlockRecordFromStore(blockRow)
		eventRow, err := queries.CreateCompetitionSafetyEvent(ctx, store.CreateCompetitionSafetyEventParams{
			EventType:            safetyBlockEventRecorded,
			CompetitionSessionID: optionalUUID(&input.CompetitionSessionID),
			CompetitionMatchID:   optionalUUID(input.CompetitionMatchID),
			SafetyBlockID:        optionalUUID(&block.ID),
			BlockerUserID:        optionalUUID(&input.BlockerUserID),
			BlockedUserID:        optionalUUID(&input.BlockedUserID),
			ReasonCode:           optionalText(input.ReasonCode),
			ActorUserID:          actor.UserID,
			ActorRole:            string(actor.Role),
			ActorSessionID:       actor.SessionID,
			Capability:           string(actor.Capability),
			TrustedSurfaceKey:    actor.TrustedSurfaceKey,
			TrustedSurfaceLabel:  optionalText(actor.TrustedSurfaceLabel),
			OccurredAt:           timestamptz(occurredAt),
		})
		if err != nil {
			return struct {
				block safetyBlockRecord
				event safetyEventRecord
			}{}, err
		}
		return struct {
			block safetyBlockRecord
			event safetyEventRecord
		}{
			block: block,
			event: safetyEventRecordFromStore(eventRow),
		}, nil
	})
	if err != nil {
		return safetyBlockRecord{}, safetyEventRecord{}, err
	}
	return result.block, result.event, nil
}

func (r *Repository) CreateReliabilityEvent(ctx context.Context, actor StaffActor, input normalizedReliabilityEventInput, occurredAt time.Time) (reliabilityEventRecord, safetyEventRecord, error) {
	result, err := withQueriesTx(ctx, r.db, func(queries *store.Queries) (struct {
		reliability reliabilityEventRecord
		event       safetyEventRecord
	}, error) {
		reliabilityRow, err := queries.CreateCompetitionReliabilityEvent(ctx, store.CreateCompetitionReliabilityEventParams{
			CompetitionSessionID: input.CompetitionSessionID,
			CompetitionMatchID:   optionalUUID(input.CompetitionMatchID),
			SubjectUserID:        optionalUUID(input.SubjectUserID),
			ReliabilityType:      input.ReliabilityType,
			Severity:             input.Severity,
			Note:                 optionalText(input.Note),
			ActorUserID:          actor.UserID,
			ActorRole:            string(actor.Role),
			ActorSessionID:       actor.SessionID,
			Capability:           string(actor.Capability),
			TrustedSurfaceKey:    actor.TrustedSurfaceKey,
			TrustedSurfaceLabel:  optionalText(actor.TrustedSurfaceLabel),
			OccurredAt:           timestamptz(occurredAt),
		})
		if err != nil {
			return struct {
				reliability reliabilityEventRecord
				event       safetyEventRecord
			}{}, err
		}
		reliability := reliabilityEventRecordFromStore(reliabilityRow)
		eventRow, err := queries.CreateCompetitionSafetyEvent(ctx, store.CreateCompetitionSafetyEventParams{
			EventType:            reliabilityEventRecordRecorded,
			CompetitionSessionID: optionalUUID(&input.CompetitionSessionID),
			CompetitionMatchID:   optionalUUID(input.CompetitionMatchID),
			ReliabilityEventID:   optionalUUID(&reliability.ID),
			SubjectUserID:        optionalUUID(input.SubjectUserID),
			ReliabilityType:      optionalText(input.ReliabilityType),
			Severity:             optionalText(input.Severity),
			ActorUserID:          actor.UserID,
			ActorRole:            string(actor.Role),
			ActorSessionID:       actor.SessionID,
			Capability:           string(actor.Capability),
			TrustedSurfaceKey:    actor.TrustedSurfaceKey,
			TrustedSurfaceLabel:  optionalText(actor.TrustedSurfaceLabel),
			OccurredAt:           timestamptz(occurredAt),
		})
		if err != nil {
			return struct {
				reliability reliabilityEventRecord
				event       safetyEventRecord
			}{}, err
		}
		return struct {
			reliability reliabilityEventRecord
			event       safetyEventRecord
		}{
			reliability: reliability,
			event:       safetyEventRecordFromStore(eventRow),
		}, nil
	})
	if err != nil {
		return reliabilityEventRecord{}, safetyEventRecord{}, err
	}
	return result.reliability, result.event, nil
}

func (r *Repository) GetSafetyReviewSummary(ctx context.Context) (safetyReviewSummaryRecord, error) {
	row, err := store.New(r.db).GetCompetitionSafetyReviewSummary(ctx)
	if err != nil {
		return safetyReviewSummaryRecord{}, err
	}
	return safetyReviewSummaryRecord{
		ReportCount:           row.ReportCount,
		BlockCount:            row.BlockCount,
		ReliabilityEventCount: row.ReliabilityEventCount,
		AuditEventCount:       row.AuditEventCount,
	}, nil
}

func (r *Repository) ListSafetyReportsForReview(ctx context.Context, limit int) ([]safetyReportRecord, error) {
	rows, err := store.New(r.db).ListCompetitionSafetyReportsForReview(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	reports := make([]safetyReportRecord, 0, len(rows))
	for _, row := range rows {
		reports = append(reports, safetyReportRecordFromStore(row))
	}
	return reports, nil
}

func (r *Repository) ListSafetyBlocksForReview(ctx context.Context, limit int) ([]safetyBlockRecord, error) {
	rows, err := store.New(r.db).ListCompetitionSafetyBlocksForReview(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	blocks := make([]safetyBlockRecord, 0, len(rows))
	for _, row := range rows {
		blocks = append(blocks, safetyBlockRecordFromStore(row))
	}
	return blocks, nil
}

func (r *Repository) ListReliabilityEventsForReview(ctx context.Context, limit int) ([]reliabilityEventRecord, error) {
	rows, err := store.New(r.db).ListCompetitionReliabilityEventsForReview(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	events := make([]reliabilityEventRecord, 0, len(rows))
	for _, row := range rows {
		events = append(events, reliabilityEventRecordFromStore(row))
	}
	return events, nil
}

func (r *Repository) ListSafetyAuditEventsForReview(ctx context.Context, limit int) ([]safetyEventRecord, error) {
	rows, err := store.New(r.db).ListCompetitionSafetyAuditEventsForReview(ctx, int32(limit))
	if err != nil {
		return nil, err
	}
	events := make([]safetyEventRecord, 0, len(rows))
	for _, row := range rows {
		events = append(events, safetyEventRecordFromStore(row))
	}
	return events, nil
}
