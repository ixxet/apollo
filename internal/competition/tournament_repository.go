package competition

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ixxet/apollo/internal/store"
)

func (r *Repository) ListTournaments(ctx context.Context) ([]tournamentRecord, error) {
	rows, err := store.New(r.db).ListCompetitionTournaments(ctx)
	if err != nil {
		return nil, err
	}

	tournaments := make([]tournamentRecord, 0, len(rows))
	for _, row := range rows {
		tournaments = append(tournaments, buildTournamentRecord(row))
	}
	return tournaments, nil
}

func (r *Repository) GetTournamentByID(ctx context.Context, tournamentID uuid.UUID) (*tournamentRecord, error) {
	row, err := store.New(r.db).GetCompetitionTournamentByID(ctx, tournamentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	record := buildTournamentRecord(row)
	return &record, nil
}

func (r *Repository) CreateTournament(ctx context.Context, actor StaffActor, input CreateTournamentInput, createdAt time.Time) (tournamentRecord, error) {
	return withQueriesTx(ctx, r.db, func(queries *store.Queries) (tournamentRecord, error) {
		row, err := queries.CreateCompetitionTournament(ctx, store.CreateCompetitionTournamentParams{
			OwnerUserID:         actor.UserID,
			DisplayName:         input.DisplayName,
			Format:              input.Format,
			SportKey:            input.SportKey,
			FacilityKey:         input.FacilityKey,
			ZoneKey:             input.ZoneKey,
			ParticipantsPerSide: int32(input.ParticipantsPerSide),
			UpdatedAt:           timestamptz(createdAt),
		})
		if err != nil {
			return tournamentRecord{}, err
		}
		tournament := buildTournamentRecord(row)

		bracketRow, err := queries.CreateCompetitionTournamentBracket(ctx, store.CreateCompetitionTournamentBracketParams{
			TournamentID: tournament.ID,
			BracketIndex: 1,
			Format:       input.Format,
			Status:       TournamentStatusDraft,
			UpdatedAt:    timestamptz(createdAt),
		})
		if err != nil {
			return tournamentRecord{}, err
		}
		bracket := buildTournamentBracketRecord(bracketRow)
		if err := recordTournamentEventTx(ctx, queries, tournamentEventRecord{
			Actor:        actor,
			TournamentID: tournament.ID,
			BracketID:    &bracket.ID,
			EventType:    tournamentEventCreated,
			OccurredAt:   createdAt.UTC(),
		}); err != nil {
			return tournamentRecord{}, err
		}

		return tournament, nil
	})
}

func (r *Repository) SeedTournament(ctx context.Context, actor StaffActor, tournament tournamentRecord, bracket tournamentBracketRecord, input SeedTournamentInput, seededAt time.Time) (tournamentRecord, error) {
	return withQueriesTx(ctx, r.db, func(queries *store.Queries) (tournamentRecord, error) {
		for _, seedInput := range input.Seeds {
			seedRow, err := queries.CreateCompetitionTournamentSeed(ctx, store.CreateCompetitionTournamentSeedParams{
				TournamentID:             tournament.ID,
				BracketID:                bracket.ID,
				Seed:                     int32(seedInput.Seed),
				CompetitionSessionTeamID: seedInput.CompetitionSessionTeamID,
				SeededAt:                 timestamptz(seededAt),
			})
			if err != nil {
				return tournamentRecord{}, err
			}
			seed := buildTournamentSeedRecord(seedRow)
			if err := recordTournamentEventTx(ctx, queries, tournamentEventRecord{
				Actor:                    actor,
				TournamentID:             tournament.ID,
				BracketID:                &bracket.ID,
				EventType:                tournamentEventSeeded,
				TournamentSeedID:         &seed.ID,
				Seed:                     &seed.Seed,
				CompetitionSessionTeamID: &seed.CompetitionSessionTeamID,
				OccurredAt:               seededAt.UTC(),
			}); err != nil {
				return tournamentRecord{}, err
			}
		}

		if _, err := queries.UpdateCompetitionTournamentBracketStatus(ctx, store.UpdateCompetitionTournamentBracketStatusParams{
			ID:           bracket.ID,
			TournamentID: tournament.ID,
			Status:       TournamentStatusSeeded,
			UpdatedAt:    timestamptz(seededAt),
		}); err != nil {
			return tournamentRecord{}, err
		}
		updated, err := queries.UpdateCompetitionTournamentStatusWithVersion(ctx, store.UpdateCompetitionTournamentStatusWithVersionParams{
			ID:                tournament.ID,
			TournamentVersion: int32(input.ExpectedTournamentVersion),
			Status:            TournamentStatusSeeded,
			UpdatedAt:         timestamptz(seededAt),
		})
		if err != nil {
			return tournamentRecord{}, err
		}
		return buildTournamentRecord(updated), nil
	})
}

func (r *Repository) LockTournamentTeam(ctx context.Context, actor StaffActor, tournament tournamentRecord, bracket tournamentBracketRecord, seed tournamentSeedRecord, team teamRecord, roster []rosterRecord, rosterHash string, lockedAt time.Time) (tournamentRecord, error) {
	return withQueriesTx(ctx, r.db, func(queries *store.Queries) (tournamentRecord, error) {
		snapshotRow, err := queries.CreateCompetitionTournamentTeamSnapshot(ctx, store.CreateCompetitionTournamentTeamSnapshotParams{
			TournamentID:             tournament.ID,
			BracketID:                bracket.ID,
			TournamentSeedID:         seed.ID,
			Seed:                     int32(seed.Seed),
			CompetitionSessionID:     team.SessionID,
			CompetitionSessionTeamID: team.ID,
			RosterHash:               rosterHash,
			LockedAt:                 timestamptz(lockedAt),
		})
		if err != nil {
			return tournamentRecord{}, err
		}
		snapshot := buildTournamentTeamSnapshotRecord(snapshotRow)
		for _, member := range roster {
			if _, err := queries.CreateCompetitionTournamentTeamSnapshotMember(ctx, store.CreateCompetitionTournamentTeamSnapshotMemberParams{
				TeamSnapshotID: snapshot.ID,
				UserID:         member.UserID,
				DisplayName:    member.DisplayName,
				SlotIndex:      int32(member.SlotIndex),
			}); err != nil {
				return tournamentRecord{}, err
			}
		}

		if err := recordTournamentEventTx(ctx, queries, tournamentEventRecord{
			Actor:                    actor,
			TournamentID:             tournament.ID,
			BracketID:                &bracket.ID,
			EventType:                tournamentEventTeamLocked,
			TournamentSeedID:         &seed.ID,
			TeamSnapshotID:           &snapshot.ID,
			Seed:                     &seed.Seed,
			CompetitionSessionTeamID: &team.ID,
			OccurredAt:               lockedAt.UTC(),
		}); err != nil {
			return tournamentRecord{}, err
		}

		seedCount, err := queries.CountCompetitionTournamentSeedsByBracketID(ctx, bracket.ID)
		if err != nil {
			return tournamentRecord{}, err
		}
		snapshotCount, err := queries.CountCompetitionTournamentTeamSnapshotsByBracketID(ctx, bracket.ID)
		if err != nil {
			return tournamentRecord{}, err
		}
		nextStatus := TournamentStatusSeeded
		if snapshotCount == seedCount {
			nextStatus = TournamentStatusLocked
		}
		if _, err := queries.UpdateCompetitionTournamentBracketStatus(ctx, store.UpdateCompetitionTournamentBracketStatusParams{
			ID:           bracket.ID,
			TournamentID: tournament.ID,
			Status:       nextStatus,
			UpdatedAt:    timestamptz(lockedAt),
		}); err != nil {
			return tournamentRecord{}, err
		}
		updated, err := queries.UpdateCompetitionTournamentStatusWithVersion(ctx, store.UpdateCompetitionTournamentStatusWithVersionParams{
			ID:                tournament.ID,
			TournamentVersion: int32(tournament.TournamentVersion),
			Status:            nextStatus,
			UpdatedAt:         timestamptz(lockedAt),
		})
		if err != nil {
			return tournamentRecord{}, err
		}
		return buildTournamentRecord(updated), nil
	})
}

func (r *Repository) BindTournamentMatch(ctx context.Context, actor StaffActor, tournament tournamentRecord, bracket tournamentBracketRecord, input BindTournamentMatchInput, sideOne tournamentTeamSnapshotRecord, sideTwo tournamentTeamSnapshotRecord, boundAt time.Time) (tournamentRecord, error) {
	return withQueriesTx(ctx, r.db, func(queries *store.Queries) (tournamentRecord, error) {
		bindingRow, err := queries.CreateCompetitionTournamentMatchBinding(ctx, store.CreateCompetitionTournamentMatchBindingParams{
			TournamentID:          tournament.ID,
			BracketID:             bracket.ID,
			Round:                 int32(input.Round),
			MatchNumber:           int32(input.MatchNumber),
			CompetitionMatchID:    input.CompetitionMatchID,
			SideOneTeamSnapshotID: sideOne.ID,
			SideTwoTeamSnapshotID: sideTwo.ID,
			BoundAt:               timestamptz(boundAt),
		})
		if err != nil {
			return tournamentRecord{}, err
		}
		binding := buildTournamentMatchBindingRecord(bindingRow)

		if err := recordTournamentEventTx(ctx, queries, tournamentEventRecord{
			Actor:              actor,
			TournamentID:       tournament.ID,
			BracketID:          &bracket.ID,
			EventType:          tournamentEventMatchBound,
			MatchBindingID:     &binding.ID,
			Round:              &binding.Round,
			CompetitionMatchID: &binding.CompetitionMatchID,
			OccurredAt:         boundAt.UTC(),
		}); err != nil {
			return tournamentRecord{}, err
		}

		if _, err := queries.UpdateCompetitionTournamentBracketStatus(ctx, store.UpdateCompetitionTournamentBracketStatusParams{
			ID:           bracket.ID,
			TournamentID: tournament.ID,
			Status:       TournamentStatusInProgress,
			UpdatedAt:    timestamptz(boundAt),
		}); err != nil {
			return tournamentRecord{}, err
		}
		updated, err := queries.UpdateCompetitionTournamentStatusWithVersion(ctx, store.UpdateCompetitionTournamentStatusWithVersionParams{
			ID:                tournament.ID,
			TournamentVersion: int32(input.ExpectedTournamentVersion),
			Status:            TournamentStatusInProgress,
			UpdatedAt:         timestamptz(boundAt),
		})
		if err != nil {
			return tournamentRecord{}, err
		}
		return buildTournamentRecord(updated), nil
	})
}

func (r *Repository) AdvanceTournamentRound(ctx context.Context, actor StaffActor, tournament tournamentRecord, bracket tournamentBracketRecord, binding tournamentMatchBindingRecord, input AdvanceTournamentRoundInput, winner tournamentTeamSnapshotRecord, loser tournamentTeamSnapshotRecord, canonicalResultID uuid.UUID, finalRound bool, advancedAt time.Time) (tournamentRecord, error) {
	return withQueriesTx(ctx, r.db, func(queries *store.Queries) (tournamentRecord, error) {
		advancementRow, err := queries.CreateCompetitionTournamentAdvancement(ctx, store.CreateCompetitionTournamentAdvancementParams{
			TournamentID:          tournament.ID,
			BracketID:             bracket.ID,
			MatchBindingID:        binding.ID,
			Round:                 int32(binding.Round),
			WinningTeamSnapshotID: winner.ID,
			LosingTeamSnapshotID:  loser.ID,
			CompetitionMatchID:    binding.CompetitionMatchID,
			CanonicalResultID:     canonicalResultID,
			AdvanceReason:         input.AdvanceReason,
			AdvancedAt:            timestamptz(advancedAt),
		})
		if err != nil {
			return tournamentRecord{}, err
		}
		advancement := buildTournamentAdvancementRecord(advancementRow)

		if err := recordTournamentEventTx(ctx, queries, tournamentEventRecord{
			Actor:              actor,
			TournamentID:       tournament.ID,
			BracketID:          &bracket.ID,
			EventType:          tournamentEventRoundAdvanced,
			MatchBindingID:     &binding.ID,
			Round:              &advancement.Round,
			AdvanceReason:      &advancement.AdvanceReason,
			CompetitionMatchID: &binding.CompetitionMatchID,
			CanonicalResultID:  &canonicalResultID,
			OccurredAt:         advancedAt.UTC(),
		}); err != nil {
			return tournamentRecord{}, err
		}

		nextStatus := TournamentStatusInProgress
		if finalRound {
			nextStatus = TournamentStatusCompleted
		}
		if _, err := queries.UpdateCompetitionTournamentBracketStatus(ctx, store.UpdateCompetitionTournamentBracketStatusParams{
			ID:           bracket.ID,
			TournamentID: tournament.ID,
			Status:       nextStatus,
			UpdatedAt:    timestamptz(advancedAt),
		}); err != nil {
			return tournamentRecord{}, err
		}
		updated, err := queries.UpdateCompetitionTournamentStatusWithVersion(ctx, store.UpdateCompetitionTournamentStatusWithVersionParams{
			ID:                tournament.ID,
			TournamentVersion: int32(input.ExpectedTournamentVersion),
			Status:            nextStatus,
			UpdatedAt:         timestamptz(advancedAt),
		})
		if err != nil {
			return tournamentRecord{}, err
		}
		return buildTournamentRecord(updated), nil
	})
}

func (r *Repository) ListTournamentBracketsByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]tournamentBracketRecord, error) {
	rows, err := store.New(r.db).ListCompetitionTournamentBracketsByTournamentID(ctx, tournamentID)
	if err != nil {
		return nil, err
	}
	records := make([]tournamentBracketRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, buildTournamentBracketRecord(row))
	}
	return records, nil
}

func (r *Repository) ListTournamentSeedsByBracketID(ctx context.Context, bracketID uuid.UUID) ([]tournamentSeedRecord, error) {
	rows, err := store.New(r.db).ListCompetitionTournamentSeedsByBracketID(ctx, bracketID)
	if err != nil {
		return nil, err
	}
	records := make([]tournamentSeedRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, buildTournamentSeedRecord(row))
	}
	return records, nil
}

func (r *Repository) GetTournamentSeedByBracketSeed(ctx context.Context, bracketID uuid.UUID, seed int) (*tournamentSeedRecord, error) {
	row, err := store.New(r.db).GetCompetitionTournamentSeedByBracketSeed(ctx, store.GetCompetitionTournamentSeedByBracketSeedParams{
		BracketID: bracketID,
		Seed:      int32(seed),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	record := buildTournamentSeedRecord(row)
	return &record, nil
}

func (r *Repository) ListTournamentTeamSnapshotsByBracketID(ctx context.Context, bracketID uuid.UUID) ([]tournamentTeamSnapshotRecord, error) {
	rows, err := store.New(r.db).ListCompetitionTournamentTeamSnapshotsByBracketID(ctx, bracketID)
	if err != nil {
		return nil, err
	}
	records := make([]tournamentTeamSnapshotRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, buildTournamentTeamSnapshotRecord(row))
	}
	return records, nil
}

func (r *Repository) GetTournamentTeamSnapshotByID(ctx context.Context, teamSnapshotID uuid.UUID) (*tournamentTeamSnapshotRecord, error) {
	row, err := store.New(r.db).GetCompetitionTournamentTeamSnapshotByID(ctx, teamSnapshotID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	record := buildTournamentTeamSnapshotRecord(row)
	return &record, nil
}

func (r *Repository) ListTournamentTeamSnapshotMembersByBracketID(ctx context.Context, bracketID uuid.UUID) ([]tournamentSnapshotMemberRecord, error) {
	rows, err := store.New(r.db).ListCompetitionTournamentTeamSnapshotMembersByBracketID(ctx, bracketID)
	if err != nil {
		return nil, err
	}
	records := make([]tournamentSnapshotMemberRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, tournamentSnapshotMemberRecord{
			TeamSnapshotID: row.TeamSnapshotID,
			UserID:         row.UserID,
			DisplayName:    row.DisplayName,
			SlotIndex:      int(row.SlotIndex),
			CreatedAt:      row.CreatedAt.Time.UTC(),
		})
	}
	return records, nil
}

func (r *Repository) CountTournamentTeamSnapshotsByBracketID(ctx context.Context, bracketID uuid.UUID) (int64, error) {
	return store.New(r.db).CountCompetitionTournamentTeamSnapshotsByBracketID(ctx, bracketID)
}

func (r *Repository) ListTournamentMatchBindingsByBracketID(ctx context.Context, bracketID uuid.UUID) ([]tournamentMatchBindingRecord, error) {
	rows, err := store.New(r.db).ListCompetitionTournamentMatchBindingsByBracketID(ctx, bracketID)
	if err != nil {
		return nil, err
	}
	records := make([]tournamentMatchBindingRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, buildTournamentMatchBindingRecord(row))
	}
	return records, nil
}

func (r *Repository) GetTournamentMatchBindingByID(ctx context.Context, matchBindingID uuid.UUID) (*tournamentMatchBindingRecord, error) {
	row, err := store.New(r.db).GetCompetitionTournamentMatchBindingByID(ctx, matchBindingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	record := buildTournamentMatchBindingRecord(row)
	return &record, nil
}

func (r *Repository) ListTournamentAdvancementsByBracketID(ctx context.Context, bracketID uuid.UUID) ([]tournamentAdvancementRecord, error) {
	rows, err := store.New(r.db).ListCompetitionTournamentAdvancementsByBracketID(ctx, bracketID)
	if err != nil {
		return nil, err
	}
	records := make([]tournamentAdvancementRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, buildTournamentAdvancementRecord(row))
	}
	return records, nil
}

func (r *Repository) ListMatchResultSidesByResultID(ctx context.Context, resultID uuid.UUID) ([]matchResultSideRecord, error) {
	rows, err := store.New(r.db).ListCompetitionMatchResultSidesByResultID(ctx, resultID)
	if err != nil {
		return nil, err
	}
	records := make([]matchResultSideRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, matchResultSideRecord{
			CompetitionMatchResultID: row.CompetitionMatchResultID,
			CompetitionMatchID:       row.CompetitionMatchID,
			SideIndex:                int(row.SideIndex),
			CompetitionSessionTeamID: row.CompetitionSessionTeamID,
			Outcome:                  row.Outcome,
		})
	}
	return records, nil
}

type tournamentEventRecord struct {
	Actor                    StaffActor
	TournamentID             uuid.UUID
	BracketID                *uuid.UUID
	EventType                string
	TournamentSeedID         *uuid.UUID
	TeamSnapshotID           *uuid.UUID
	MatchBindingID           *uuid.UUID
	Round                    *int
	Seed                     *int
	AdvanceReason            *string
	CompetitionSessionTeamID *uuid.UUID
	CompetitionMatchID       *uuid.UUID
	CanonicalResultID        *uuid.UUID
	OccurredAt               time.Time
}

func recordTournamentEventTx(ctx context.Context, queries *store.Queries, event tournamentEventRecord) error {
	actorRole := string(event.Actor.Role)
	capability := string(event.Actor.Capability)
	_, err := queries.CreateCompetitionTournamentEvent(ctx, store.CreateCompetitionTournamentEventParams{
		TournamentID:             event.TournamentID,
		BracketID:                optionalUUID(event.BracketID),
		EventType:                event.EventType,
		TournamentSeedID:         optionalUUID(event.TournamentSeedID),
		TeamSnapshotID:           optionalUUID(event.TeamSnapshotID),
		MatchBindingID:           optionalUUID(event.MatchBindingID),
		Round:                    optionalInt32(event.Round),
		Seed:                     optionalInt32(event.Seed),
		AdvanceReason:            optionalTextPtr(event.AdvanceReason),
		CompetitionSessionTeamID: optionalUUID(event.CompetitionSessionTeamID),
		CompetitionMatchID:       optionalUUID(event.CompetitionMatchID),
		CanonicalResultID:        optionalUUID(event.CanonicalResultID),
		ActorUserID:              optionalUUID(&event.Actor.UserID),
		ActorRole:                optionalText(actorRole),
		ActorSessionID:           optionalUUID(&event.Actor.SessionID),
		Capability:               optionalText(capability),
		TrustedSurfaceKey:        optionalText(event.Actor.TrustedSurfaceKey),
		TrustedSurfaceLabel:      optionalText(event.Actor.TrustedSurfaceLabel),
		OccurredAt:               timestamptz(event.OccurredAt),
	})
	return err
}

func buildTournamentRecord(row store.ApolloCompetitionTournament) tournamentRecord {
	return tournamentRecord{
		ID:                  row.ID,
		OwnerUserID:         row.OwnerUserID,
		DisplayName:         row.DisplayName,
		Format:              row.Format,
		Visibility:          row.Visibility,
		SportKey:            row.SportKey,
		FacilityKey:         row.FacilityKey,
		ZoneKey:             row.ZoneKey,
		ParticipantsPerSide: int(row.ParticipantsPerSide),
		Status:              row.Status,
		TournamentVersion:   int(row.TournamentVersion),
		CreatedAt:           row.CreatedAt.Time.UTC(),
		UpdatedAt:           row.UpdatedAt.Time.UTC(),
		ArchivedAt:          timePtr(row.ArchivedAt),
	}
}

func buildTournamentBracketRecord(row store.ApolloCompetitionTournamentBracket) tournamentBracketRecord {
	return tournamentBracketRecord{
		ID:           row.ID,
		TournamentID: row.TournamentID,
		BracketIndex: int(row.BracketIndex),
		Format:       row.Format,
		Status:       row.Status,
		CreatedAt:    row.CreatedAt.Time.UTC(),
		UpdatedAt:    row.UpdatedAt.Time.UTC(),
	}
}

func buildTournamentSeedRecord(row store.ApolloCompetitionTournamentSeed) tournamentSeedRecord {
	return tournamentSeedRecord{
		ID:                       row.ID,
		TournamentID:             row.TournamentID,
		BracketID:                row.BracketID,
		Seed:                     int(row.Seed),
		CompetitionSessionTeamID: row.CompetitionSessionTeamID,
		SeededAt:                 row.SeededAt.Time.UTC(),
		CreatedAt:                row.CreatedAt.Time.UTC(),
	}
}

func buildTournamentTeamSnapshotRecord(row store.ApolloCompetitionTournamentTeamSnapshot) tournamentTeamSnapshotRecord {
	return tournamentTeamSnapshotRecord{
		ID:                       row.ID,
		TournamentID:             row.TournamentID,
		BracketID:                row.BracketID,
		TournamentSeedID:         row.TournamentSeedID,
		Seed:                     int(row.Seed),
		CompetitionSessionID:     row.CompetitionSessionID,
		CompetitionSessionTeamID: row.CompetitionSessionTeamID,
		RosterHash:               row.RosterHash,
		LockedAt:                 row.LockedAt.Time.UTC(),
		CreatedAt:                row.CreatedAt.Time.UTC(),
	}
}

func buildTournamentMatchBindingRecord(row store.ApolloCompetitionTournamentMatchBinding) tournamentMatchBindingRecord {
	return tournamentMatchBindingRecord{
		ID:                    row.ID,
		TournamentID:          row.TournamentID,
		BracketID:             row.BracketID,
		Round:                 int(row.Round),
		MatchNumber:           int(row.MatchNumber),
		CompetitionMatchID:    row.CompetitionMatchID,
		SideOneTeamSnapshotID: row.SideOneTeamSnapshotID,
		SideTwoTeamSnapshotID: row.SideTwoTeamSnapshotID,
		BoundAt:               row.BoundAt.Time.UTC(),
		CreatedAt:             row.CreatedAt.Time.UTC(),
	}
}

func buildTournamentAdvancementRecord(row store.ApolloCompetitionTournamentAdvancement) tournamentAdvancementRecord {
	return tournamentAdvancementRecord{
		ID:                    row.ID,
		TournamentID:          row.TournamentID,
		BracketID:             row.BracketID,
		MatchBindingID:        row.MatchBindingID,
		Round:                 int(row.Round),
		WinningTeamSnapshotID: row.WinningTeamSnapshotID,
		LosingTeamSnapshotID:  row.LosingTeamSnapshotID,
		CompetitionMatchID:    row.CompetitionMatchID,
		CanonicalResultID:     row.CanonicalResultID,
		AdvanceReason:         row.AdvanceReason,
		AdvancedAt:            row.AdvancedAt.Time.UTC(),
		CreatedAt:             row.CreatedAt.Time.UTC(),
	}
}

func optionalInt32(value *int) *int32 {
	if value == nil {
		return nil
	}
	converted := int32(*value)
	return &converted
}

func optionalTextPtr(value *string) *string {
	if value == nil {
		return nil
	}
	return optionalText(*value)
}
