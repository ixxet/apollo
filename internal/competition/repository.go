package competition

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ixxet/apollo/internal/store"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetUserByID(ctx context.Context, userID uuid.UUID) (*store.ApolloUser, error) {
	row, err := store.New(r.db).GetUserByID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	user := store.ApolloUserFromGetUserByIDRow(row)
	return &user, nil
}

func (r *Repository) GetLobbyMembershipByUserID(ctx context.Context, userID uuid.UUID) (*store.ApolloLobbyMembership, error) {
	row, err := store.New(r.db).GetLobbyMembershipByUserID(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &row, nil
}

func (r *Repository) GetSportConfig(ctx context.Context, sportKey string) (*SportConfig, error) {
	row, err := store.New(r.db).GetSportByKey(ctx, sportKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &SportConfig{
		SportKey:               row.SportKey,
		SidesPerMatch:          int(row.SidesPerMatch),
		ParticipantsPerSideMin: int(row.ParticipantsPerSideMin),
		ParticipantsPerSideMax: int(row.ParticipantsPerSideMax),
	}, nil
}

func (r *Repository) ListFacilityCapabilities(ctx context.Context) ([]FacilityCapability, error) {
	rows, err := store.New(r.db).ListSportFacilityCapabilities(ctx)
	if err != nil {
		return nil, err
	}

	capabilities := make([]FacilityCapability, 0, len(rows))
	indexByKey := make(map[string]int, len(rows))
	for _, row := range rows {
		key := row.SportKey + "\x00" + row.FacilityKey
		index, exists := indexByKey[key]
		if !exists {
			index = len(capabilities)
			indexByKey[key] = index
			capabilities = append(capabilities, FacilityCapability{
				SportKey:    row.SportKey,
				FacilityKey: row.FacilityKey,
			})
		}

		if row.ZoneKey != nil {
			capabilities[index].ZoneKeys = append(capabilities[index].ZoneKeys, *row.ZoneKey)
		}
	}

	return capabilities, nil
}

func (r *Repository) ListSessionsByOwner(ctx context.Context, ownerUserID uuid.UUID) ([]sessionRecord, error) {
	rows, err := store.New(r.db).ListCompetitionSessionsByOwner(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}

	sessions := make([]sessionRecord, 0, len(rows))
	for _, row := range rows {
		sessions = append(sessions, buildSessionRecordValues(
			row.ID,
			row.OwnerUserID,
			row.DisplayName,
			row.SportKey,
			row.FacilityKey,
			row.ZoneKey,
			row.ParticipantsPerSide,
			row.QueueVersion,
			row.Status,
			row.CreatedAt,
			row.UpdatedAt,
			row.ArchivedAt,
		))
	}

	return sessions, nil
}

func (r *Repository) GetSessionByIDForOwner(ctx context.Context, sessionID uuid.UUID, ownerUserID uuid.UUID) (*sessionRecord, error) {
	row, err := store.New(r.db).GetCompetitionSessionByIDForOwner(ctx, store.GetCompetitionSessionByIDForOwnerParams{
		ID:          sessionID,
		OwnerUserID: ownerUserID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	record := buildSessionRecordValues(
		row.ID,
		row.OwnerUserID,
		row.DisplayName,
		row.SportKey,
		row.FacilityKey,
		row.ZoneKey,
		row.ParticipantsPerSide,
		row.QueueVersion,
		row.Status,
		row.CreatedAt,
		row.UpdatedAt,
		row.ArchivedAt,
	)
	return &record, nil
}

func (r *Repository) CreateSession(ctx context.Context, ownerUserID uuid.UUID, input CreateSessionInput) (sessionRecord, error) {
	row, err := store.New(r.db).CreateCompetitionSession(ctx, store.CreateCompetitionSessionParams{
		OwnerUserID:         ownerUserID,
		DisplayName:         input.DisplayName,
		SportKey:            input.SportKey,
		FacilityKey:         input.FacilityKey,
		ZoneKey:             input.ZoneKey,
		ParticipantsPerSide: int32(input.ParticipantsPerSide),
	})
	if err != nil {
		return sessionRecord{}, err
	}

	return buildSessionRecordValues(
		row.ID,
		row.OwnerUserID,
		row.DisplayName,
		row.SportKey,
		row.FacilityKey,
		row.ZoneKey,
		row.ParticipantsPerSide,
		row.QueueVersion,
		row.Status,
		row.CreatedAt,
		row.UpdatedAt,
		row.ArchivedAt,
	), nil
}

func (r *Repository) OpenQueue(ctx context.Context, sessionID uuid.UUID, ownerUserID uuid.UUID, updatedAt time.Time) (sessionRecord, error) {
	row, err := store.New(r.db).OpenCompetitionSessionQueue(ctx, store.OpenCompetitionSessionQueueParams{
		ID:          sessionID,
		OwnerUserID: ownerUserID,
		UpdatedAt:   timestamptz(updatedAt),
	})
	if err != nil {
		return sessionRecord{}, err
	}

	return buildSessionRecordValues(
		row.ID,
		row.OwnerUserID,
		row.DisplayName,
		row.SportKey,
		row.FacilityKey,
		row.ZoneKey,
		row.ParticipantsPerSide,
		row.QueueVersion,
		row.Status,
		row.CreatedAt,
		row.UpdatedAt,
		row.ArchivedAt,
	), nil
}

func (r *Repository) UpdateSessionStatus(ctx context.Context, sessionID uuid.UUID, ownerUserID uuid.UUID, fromStatus string, toStatus string, updatedAt time.Time) (sessionRecord, error) {
	row, err := store.New(r.db).UpdateCompetitionSessionStatus(ctx, store.UpdateCompetitionSessionStatusParams{
		ID:          sessionID,
		OwnerUserID: ownerUserID,
		Status:      toStatus,
		UpdatedAt:   timestamptz(updatedAt),
		Status_2:    fromStatus,
	})
	if err != nil {
		return sessionRecord{}, err
	}

	return buildSessionRecordValues(
		row.ID,
		row.OwnerUserID,
		row.DisplayName,
		row.SportKey,
		row.FacilityKey,
		row.ZoneKey,
		row.ParticipantsPerSide,
		row.QueueVersion,
		row.Status,
		row.CreatedAt,
		row.UpdatedAt,
		row.ArchivedAt,
	), nil
}

func (r *Repository) AddQueueMember(ctx context.Context, sessionID uuid.UUID, ownerUserID uuid.UUID, userID uuid.UUID, joinedAt time.Time) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	queries := store.New(tx)
	if _, err := queries.CreateCompetitionSessionQueueMember(ctx, store.CreateCompetitionSessionQueueMemberParams{
		CompetitionSessionID: sessionID,
		UserID:               userID,
		JoinedAt:             timestamptz(joinedAt),
	}); err != nil {
		return err
	}

	if _, err := queries.BumpCompetitionSessionQueueVersion(ctx, store.BumpCompetitionSessionQueueVersionParams{
		ID:          sessionID,
		OwnerUserID: ownerUserID,
		UpdatedAt:   timestamptz(joinedAt),
	}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *Repository) RemoveQueueMember(ctx context.Context, sessionID uuid.UUID, ownerUserID uuid.UUID, userID uuid.UUID, updatedAt time.Time) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	queries := store.New(tx)
	deleted, err := queries.DeleteCompetitionSessionQueueMember(ctx, store.DeleteCompetitionSessionQueueMemberParams{
		CompetitionSessionID: sessionID,
		UserID:               userID,
	})
	if err != nil {
		return err
	}
	if deleted == 0 {
		return pgx.ErrNoRows
	}

	if _, err := queries.BumpCompetitionSessionQueueVersion(ctx, store.BumpCompetitionSessionQueueVersionParams{
		ID:          sessionID,
		OwnerUserID: ownerUserID,
		UpdatedAt:   timestamptz(updatedAt),
	}); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *Repository) AssignQueue(ctx context.Context, ownerUserID uuid.UUID, session sessionRecord, input AssignSessionInput, sport SportConfig, queueMembers []queueRecord, assignedAt time.Time) (sessionRecord, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return sessionRecord{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	queries := store.New(tx)
	sideSlots := make([]MatchSideInput, 0, sport.SidesPerMatch)
	queueIndex := 0

	for sideIndex := 1; sideIndex <= sport.SidesPerMatch; sideIndex++ {
		team, err := queries.CreateCompetitionSessionTeam(ctx, store.CreateCompetitionSessionTeamParams{
			CompetitionSessionID: session.ID,
			SideIndex:            int32(sideIndex),
		})
		if err != nil {
			return sessionRecord{}, err
		}

		sideSlots = append(sideSlots, MatchSideInput{
			TeamID:    team.ID,
			SideIndex: sideIndex,
		})

		for slotIndex := 1; slotIndex <= session.ParticipantsPerSide; slotIndex++ {
			member := queueMembers[queueIndex]
			queueIndex++

			if _, err := queries.CreateCompetitionTeamRosterMember(ctx, store.CreateCompetitionTeamRosterMemberParams{
				CompetitionSessionID:     session.ID,
				CompetitionSessionTeamID: team.ID,
				UserID:                   member.UserID,
				SlotIndex:                int32(slotIndex),
			}); err != nil {
				return sessionRecord{}, err
			}
		}
	}

	if _, err := createCompetitionMatchWithSideSlotsTx(ctx, queries, session.ID, 1, MatchStatusAssigned, sideSlots); err != nil {
		return sessionRecord{}, err
	}

	if _, err := queries.DeleteCompetitionSessionQueueMembersBySessionID(ctx, session.ID); err != nil {
		return sessionRecord{}, err
	}

	row, err := queries.AssignCompetitionSessionFromQueue(ctx, store.AssignCompetitionSessionFromQueueParams{
		ID:           session.ID,
		OwnerUserID:  ownerUserID,
		QueueVersion: int32(input.ExpectedQueueVersion),
		UpdatedAt:    timestamptz(assignedAt),
	})
	if err != nil {
		return sessionRecord{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return sessionRecord{}, err
	}

	return buildSessionRecordValues(
		row.ID,
		row.OwnerUserID,
		row.DisplayName,
		row.SportKey,
		row.FacilityKey,
		row.ZoneKey,
		row.ParticipantsPerSide,
		row.QueueVersion,
		row.Status,
		row.CreatedAt,
		row.UpdatedAt,
		row.ArchivedAt,
	), nil
}

func (r *Repository) CountDraftMatchesBySessionID(ctx context.Context, sessionID uuid.UUID) (int64, error) {
	return store.New(r.db).CountDraftCompetitionMatchesBySessionID(ctx, sessionID)
}

func (r *Repository) CountQueueMembersBySessionID(ctx context.Context, sessionID uuid.UUID) (int64, error) {
	return store.New(r.db).CountCompetitionSessionQueueMembersBySessionID(ctx, sessionID)
}

func (r *Repository) ListQueueMembersBySessionID(ctx context.Context, sessionID uuid.UUID) ([]queueRecord, error) {
	rows, err := store.New(r.db).ListCompetitionSessionQueueMembersBySessionID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	members := make([]queueRecord, 0, len(rows))
	for _, row := range rows {
		members = append(members, queueRecord{
			UserID:                row.UserID,
			DisplayName:           row.DisplayName,
			Preferences:           row.Preferences,
			LobbyMembershipStatus: row.LobbyMembershipStatus,
			JoinedAt:              row.JoinedAt.Time.UTC(),
		})
	}

	return members, nil
}

func (r *Repository) ListTeamsBySessionID(ctx context.Context, sessionID uuid.UUID) ([]teamRecord, error) {
	rows, err := store.New(r.db).ListCompetitionSessionTeamsBySessionID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	teams := make([]teamRecord, 0, len(rows))
	for _, row := range rows {
		teams = append(teams, buildTeamRecord(row))
	}

	return teams, nil
}

func (r *Repository) GetTeamByID(ctx context.Context, teamID uuid.UUID) (*teamRecord, error) {
	row, err := store.New(r.db).GetCompetitionSessionTeamByID(ctx, teamID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	record := buildTeamRecord(row)
	return &record, nil
}

func (r *Repository) CreateTeam(ctx context.Context, sessionID uuid.UUID, sideIndex int) (teamRecord, error) {
	row, err := store.New(r.db).CreateCompetitionSessionTeam(ctx, store.CreateCompetitionSessionTeamParams{
		CompetitionSessionID: sessionID,
		SideIndex:            int32(sideIndex),
	})
	if err != nil {
		return teamRecord{}, err
	}

	return buildTeamRecord(row), nil
}

func (r *Repository) DeleteTeam(ctx context.Context, teamID uuid.UUID) (int64, error) {
	return store.New(r.db).DeleteCompetitionSessionTeam(ctx, teamID)
}

func (r *Repository) CountRosterMembersByTeamID(ctx context.Context, teamID uuid.UUID) (int64, error) {
	return store.New(r.db).CountCompetitionRosterMembersByTeamID(ctx, teamID)
}

func (r *Repository) SessionHasRosterMemberUser(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID) (bool, error) {
	return store.New(r.db).CompetitionSessionHasRosterMemberUser(ctx, store.CompetitionSessionHasRosterMemberUserParams{
		CompetitionSessionID: sessionID,
		UserID:               userID,
	})
}

func (r *Repository) ListRosterMembersBySessionID(ctx context.Context, sessionID uuid.UUID) ([]rosterRecord, error) {
	rows, err := store.New(r.db).ListCompetitionTeamRosterMembersBySessionID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	members := make([]rosterRecord, 0, len(rows))
	for _, row := range rows {
		members = append(members, rosterRecord{
			TeamID:      row.CompetitionSessionTeamID,
			UserID:      row.UserID,
			DisplayName: row.DisplayName,
			SlotIndex:   int(row.SlotIndex),
			CreatedAt:   row.CreatedAt.Time.UTC(),
		})
	}

	return members, nil
}

func (r *Repository) CreateRosterMember(ctx context.Context, sessionID uuid.UUID, teamID uuid.UUID, userID uuid.UUID, slotIndex int) (rosterRecord, error) {
	row, err := store.New(r.db).CreateCompetitionTeamRosterMember(ctx, store.CreateCompetitionTeamRosterMemberParams{
		CompetitionSessionID:     sessionID,
		CompetitionSessionTeamID: teamID,
		UserID:                   userID,
		SlotIndex:                int32(slotIndex),
	})
	if err != nil {
		return rosterRecord{}, err
	}

	return rosterRecord{
		TeamID:    row.CompetitionSessionTeamID,
		UserID:    row.UserID,
		SlotIndex: int(row.SlotIndex),
		CreatedAt: row.CreatedAt.Time.UTC(),
	}, nil
}

func (r *Repository) DeleteRosterMember(ctx context.Context, teamID uuid.UUID, userID uuid.UUID) (int64, error) {
	return store.New(r.db).DeleteCompetitionTeamRosterMember(ctx, store.DeleteCompetitionTeamRosterMemberParams{
		CompetitionSessionTeamID: teamID,
		UserID:                   userID,
	})
}

func (r *Repository) TeamHasMatchReference(ctx context.Context, teamID uuid.UUID) (bool, error) {
	return store.New(r.db).CompetitionTeamHasMatchReference(ctx, teamID)
}

func (r *Repository) ListMatchesBySessionID(ctx context.Context, sessionID uuid.UUID) ([]matchRecord, error) {
	rows, err := store.New(r.db).ListCompetitionMatchesBySessionID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	matches := make([]matchRecord, 0, len(rows))
	for _, row := range rows {
		matches = append(matches, buildMatchRecord(row))
	}

	return matches, nil
}

func (r *Repository) GetMatchByID(ctx context.Context, matchID uuid.UUID) (*matchRecord, error) {
	row, err := store.New(r.db).GetCompetitionMatchByID(ctx, matchID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	record := buildMatchRecord(row)
	return &record, nil
}

func (r *Repository) CreateMatchWithSideSlots(ctx context.Context, sessionID uuid.UUID, matchIndex int, sideSlots []MatchSideInput) (matchRecord, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return matchRecord{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	match, err := createCompetitionMatchWithSideSlotsTx(ctx, store.New(tx), sessionID, matchIndex, MatchStatusDraft, sideSlots)
	if err != nil {
		return matchRecord{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return matchRecord{}, err
	}

	return match, nil
}

func (r *Repository) ArchiveMatch(ctx context.Context, matchID uuid.UUID, archivedAt time.Time) (matchRecord, error) {
	row, err := store.New(r.db).ArchiveCompetitionMatch(ctx, store.ArchiveCompetitionMatchParams{
		ID:         matchID,
		ArchivedAt: timestamptz(archivedAt),
	})
	if err != nil {
		return matchRecord{}, err
	}

	return buildMatchRecord(row), nil
}

func (r *Repository) UpdateMatchStatusesBySessionID(ctx context.Context, sessionID uuid.UUID, fromStatus string, toStatus string, updatedAt time.Time) (int64, error) {
	return store.New(r.db).UpdateCompetitionMatchStatusBySessionID(ctx, store.UpdateCompetitionMatchStatusBySessionIDParams{
		CompetitionSessionID: sessionID,
		Status:               toStatus,
		Status_2:             fromStatus,
		UpdatedAt:            timestamptz(updatedAt),
	})
}

func (r *Repository) ListMatchSideSlotsBySessionID(ctx context.Context, sessionID uuid.UUID) ([]matchSideSlotRecord, error) {
	rows, err := store.New(r.db).ListCompetitionMatchSideSlotsBySessionID(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	slots := make([]matchSideSlotRecord, 0, len(rows))
	for _, row := range rows {
		slots = append(slots, matchSideSlotRecord{
			MatchID:   row.CompetitionMatchID,
			TeamID:    row.CompetitionSessionTeamID,
			SideIndex: int(row.SideIndex),
			CreatedAt: row.CreatedAt.Time.UTC(),
		})
	}

	return slots, nil
}

func createCompetitionMatchWithSideSlotsTx(ctx context.Context, queries *store.Queries, sessionID uuid.UUID, matchIndex int, status string, sideSlots []MatchSideInput) (matchRecord, error) {
	match, err := queries.CreateCompetitionMatch(ctx, store.CreateCompetitionMatchParams{
		CompetitionSessionID: sessionID,
		MatchIndex:           int32(matchIndex),
		Status:               status,
	})
	if err != nil {
		return matchRecord{}, err
	}

	for _, sideSlot := range sideSlots {
		if _, err := queries.CreateCompetitionMatchSideSlot(ctx, store.CreateCompetitionMatchSideSlotParams{
			CompetitionMatchID:       match.ID,
			CompetitionSessionTeamID: sideSlot.TeamID,
			SideIndex:                int32(sideSlot.SideIndex),
		}); err != nil {
			return matchRecord{}, err
		}
	}

	return buildMatchRecord(match), nil
}

func buildSessionRecordValues(id uuid.UUID, ownerUserID uuid.UUID, displayName string, sportKey string, facilityKey string, zoneKey *string, participantsPerSide int32, queueVersion int32, status string, createdAt pgtype.Timestamptz, updatedAt pgtype.Timestamptz, archivedAt pgtype.Timestamptz) sessionRecord {
	return sessionRecord{
		ID:                  id,
		OwnerUserID:         ownerUserID,
		DisplayName:         displayName,
		SportKey:            sportKey,
		FacilityKey:         facilityKey,
		ZoneKey:             zoneKey,
		ParticipantsPerSide: int(participantsPerSide),
		QueueVersion:        int(queueVersion),
		Status:              status,
		CreatedAt:           createdAt.Time.UTC(),
		UpdatedAt:           updatedAt.Time.UTC(),
		ArchivedAt:          timePtr(archivedAt),
	}
}

func buildTeamRecord(row store.ApolloCompetitionSessionTeam) teamRecord {
	return teamRecord{
		ID:        row.ID,
		SessionID: row.CompetitionSessionID,
		SideIndex: int(row.SideIndex),
		CreatedAt: row.CreatedAt.Time.UTC(),
	}
}

func buildMatchRecord(row store.ApolloCompetitionMatch) matchRecord {
	return matchRecord{
		ID:         row.ID,
		SessionID:  row.CompetitionSessionID,
		MatchIndex: int(row.MatchIndex),
		Status:     row.Status,
		CreatedAt:  row.CreatedAt.Time.UTC(),
		UpdatedAt:  row.UpdatedAt.Time.UTC(),
		ArchivedAt: timePtr(row.ArchivedAt),
	}
}

func timestamptz(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

func timePtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}

	timestamp := value.Time.UTC()
	return &timestamp
}
