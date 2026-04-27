package competition

import (
	"context"
	"errors"
	"strings"
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
		CompetitionMode:        row.CompetitionMode,
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

func (r *Repository) ListSessions(ctx context.Context) ([]sessionRecord, error) {
	rows, err := store.New(r.db).ListCompetitionSessions(ctx)
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

func (r *Repository) GetSessionByID(ctx context.Context, sessionID uuid.UUID) (*sessionRecord, error) {
	row, err := store.New(r.db).GetCompetitionSessionByID(ctx, sessionID)
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

func (r *Repository) CreateSession(ctx context.Context, actor StaffActor, input CreateSessionInput, createdAt time.Time) (sessionRecord, error) {
	return withQueriesTx(ctx, r.db, func(queries *store.Queries) (sessionRecord, error) {
		row, err := queries.CreateCompetitionSession(ctx, store.CreateCompetitionSessionParams{
			OwnerUserID:         actor.UserID,
			DisplayName:         input.DisplayName,
			SportKey:            input.SportKey,
			FacilityKey:         input.FacilityKey,
			ZoneKey:             input.ZoneKey,
			ParticipantsPerSide: int32(input.ParticipantsPerSide),
		})
		if err != nil {
			return sessionRecord{}, err
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
		attribution := newStaffActionAttribution(actor, "competition_session.create", createdAt)
		attribution.CompetitionSessionID = uuidPtr(record.ID)
		if err := recordStaffActionAttributionTx(ctx, queries, attribution); err != nil {
			return sessionRecord{}, err
		}

		return record, nil
	})
}

func (r *Repository) OpenQueue(ctx context.Context, actor StaffActor, session sessionRecord, updatedAt time.Time) (sessionRecord, error) {
	return withQueriesTx(ctx, r.db, func(queries *store.Queries) (sessionRecord, error) {
		row, err := queries.OpenCompetitionSessionQueue(ctx, store.OpenCompetitionSessionQueueParams{
			ID:          session.ID,
			OwnerUserID: session.OwnerUserID,
			UpdatedAt:   timestamptz(updatedAt),
		})
		if err != nil {
			return sessionRecord{}, err
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
		attribution := newStaffActionAttribution(actor, "competition_session.queue_open", updatedAt)
		attribution.CompetitionSessionID = uuidPtr(session.ID)
		if err := recordStaffActionAttributionTx(ctx, queries, attribution); err != nil {
			return sessionRecord{}, err
		}

		return record, nil
	})
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

func (r *Repository) AddQueueMember(ctx context.Context, actor StaffActor, session sessionRecord, userID uuid.UUID, joinedAt time.Time) error {
	_, err := withQueriesTx(ctx, r.db, func(queries *store.Queries) (struct{}, error) {
		if _, err := queries.CreateCompetitionSessionQueueMember(ctx, store.CreateCompetitionSessionQueueMemberParams{
			CompetitionSessionID: session.ID,
			UserID:               userID,
			JoinedAt:             timestamptz(joinedAt),
		}); err != nil {
			return struct{}{}, err
		}

		if _, err := queries.BumpCompetitionSessionQueueVersion(ctx, store.BumpCompetitionSessionQueueVersionParams{
			ID:          session.ID,
			OwnerUserID: session.OwnerUserID,
			UpdatedAt:   timestamptz(joinedAt),
		}); err != nil {
			return struct{}{}, err
		}

		attribution := newStaffActionAttribution(actor, "competition_session.queue_member_add", joinedAt)
		attribution.CompetitionSessionID = uuidPtr(session.ID)
		attribution.SubjectUserID = uuidPtr(userID)
		return struct{}{}, recordStaffActionAttributionTx(ctx, queries, attribution)
	})
	return err
}

func (r *Repository) RemoveQueueMember(ctx context.Context, actor StaffActor, session sessionRecord, userID uuid.UUID, updatedAt time.Time) error {
	_, err := withQueriesTx(ctx, r.db, func(queries *store.Queries) (struct{}, error) {
		deleted, err := queries.DeleteCompetitionSessionQueueMember(ctx, store.DeleteCompetitionSessionQueueMemberParams{
			CompetitionSessionID: session.ID,
			UserID:               userID,
		})
		if err != nil {
			return struct{}{}, err
		}
		if deleted == 0 {
			return struct{}{}, pgx.ErrNoRows
		}

		if _, err := queries.BumpCompetitionSessionQueueVersion(ctx, store.BumpCompetitionSessionQueueVersionParams{
			ID:          session.ID,
			OwnerUserID: session.OwnerUserID,
			UpdatedAt:   timestamptz(updatedAt),
		}); err != nil {
			return struct{}{}, err
		}

		attribution := newStaffActionAttribution(actor, "competition_session.queue_member_remove", updatedAt)
		attribution.CompetitionSessionID = uuidPtr(session.ID)
		attribution.SubjectUserID = uuidPtr(userID)
		return struct{}{}, recordStaffActionAttributionTx(ctx, queries, attribution)
	})
	return err
}

func (r *Repository) AssignQueue(ctx context.Context, actor StaffActor, session sessionRecord, input AssignSessionInput, sport SportConfig, queueMembers []queueRecord, assignedAt time.Time) (sessionRecord, error) {
	return withQueriesTx(ctx, r.db, func(queries *store.Queries) (sessionRecord, error) {
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
			OwnerUserID:  session.OwnerUserID,
			QueueVersion: int32(input.ExpectedQueueVersion),
			UpdatedAt:    timestamptz(assignedAt),
		})
		if err != nil {
			return sessionRecord{}, err
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
		attribution := newStaffActionAttribution(actor, "competition_session.assign", assignedAt)
		attribution.CompetitionSessionID = uuidPtr(session.ID)
		if err := recordStaffActionAttributionTx(ctx, queries, attribution); err != nil {
			return sessionRecord{}, err
		}

		return record, nil
	})
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

func (r *Repository) CreateTeam(ctx context.Context, actor StaffActor, sessionID uuid.UUID, sideIndex int, createdAt time.Time) (teamRecord, error) {
	return withQueriesTx(ctx, r.db, func(queries *store.Queries) (teamRecord, error) {
		row, err := queries.CreateCompetitionSessionTeam(ctx, store.CreateCompetitionSessionTeamParams{
			CompetitionSessionID: sessionID,
			SideIndex:            int32(sideIndex),
		})
		if err != nil {
			return teamRecord{}, err
		}

		record := buildTeamRecord(row)
		attribution := newStaffActionAttribution(actor, "competition_team.create", createdAt)
		attribution.CompetitionSessionID = uuidPtr(sessionID)
		attribution.CompetitionTeamID = uuidPtr(record.ID)
		if err := recordStaffActionAttributionTx(ctx, queries, attribution); err != nil {
			return teamRecord{}, err
		}

		return record, nil
	})
}

func (r *Repository) DeleteTeam(ctx context.Context, actor StaffActor, sessionID uuid.UUID, teamID uuid.UUID, deletedAt time.Time) (int64, error) {
	return withQueriesTx(ctx, r.db, func(queries *store.Queries) (int64, error) {
		deleted, err := queries.DeleteCompetitionSessionTeam(ctx, teamID)
		if err != nil {
			return 0, err
		}
		if deleted == 0 {
			return 0, nil
		}

		attribution := newStaffActionAttribution(actor, "competition_team.remove", deletedAt)
		attribution.CompetitionSessionID = uuidPtr(sessionID)
		attribution.CompetitionTeamID = uuidPtr(teamID)
		if err := recordStaffActionAttributionTx(ctx, queries, attribution); err != nil {
			return 0, err
		}

		return deleted, nil
	})
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

func (r *Repository) CreateRosterMember(ctx context.Context, actor StaffActor, sessionID uuid.UUID, teamID uuid.UUID, userID uuid.UUID, slotIndex int, createdAt time.Time) (rosterRecord, error) {
	return withQueriesTx(ctx, r.db, func(queries *store.Queries) (rosterRecord, error) {
		row, err := queries.CreateCompetitionTeamRosterMember(ctx, store.CreateCompetitionTeamRosterMemberParams{
			CompetitionSessionID:     sessionID,
			CompetitionSessionTeamID: teamID,
			UserID:                   userID,
			SlotIndex:                int32(slotIndex),
		})
		if err != nil {
			return rosterRecord{}, err
		}

		record := rosterRecord{
			TeamID:    row.CompetitionSessionTeamID,
			UserID:    row.UserID,
			SlotIndex: int(row.SlotIndex),
			CreatedAt: row.CreatedAt.Time.UTC(),
		}
		attribution := newStaffActionAttribution(actor, "competition_roster_member.add", createdAt)
		attribution.CompetitionSessionID = uuidPtr(sessionID)
		attribution.CompetitionTeamID = uuidPtr(teamID)
		attribution.SubjectUserID = uuidPtr(userID)
		if err := recordStaffActionAttributionTx(ctx, queries, attribution); err != nil {
			return rosterRecord{}, err
		}

		return record, nil
	})
}

func (r *Repository) DeleteRosterMember(ctx context.Context, actor StaffActor, sessionID uuid.UUID, teamID uuid.UUID, userID uuid.UUID, deletedAt time.Time) (int64, error) {
	return withQueriesTx(ctx, r.db, func(queries *store.Queries) (int64, error) {
		deleted, err := queries.DeleteCompetitionTeamRosterMember(ctx, store.DeleteCompetitionTeamRosterMemberParams{
			CompetitionSessionTeamID: teamID,
			UserID:                   userID,
		})
		if err != nil {
			return 0, err
		}
		if deleted == 0 {
			return 0, nil
		}

		attribution := newStaffActionAttribution(actor, "competition_roster_member.remove", deletedAt)
		attribution.CompetitionSessionID = uuidPtr(sessionID)
		attribution.CompetitionTeamID = uuidPtr(teamID)
		attribution.SubjectUserID = uuidPtr(userID)
		if err := recordStaffActionAttributionTx(ctx, queries, attribution); err != nil {
			return 0, err
		}

		return deleted, nil
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
		matches = append(matches, buildMatchRecordValues(
			row.ID,
			row.CompetitionSessionID,
			row.MatchIndex,
			row.Status,
			row.ResultVersion,
			row.CanonicalResultID,
			row.CreatedAt,
			row.UpdatedAt,
			row.ArchivedAt,
		))
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

	record := buildMatchRecordValues(
		row.ID,
		row.CompetitionSessionID,
		row.MatchIndex,
		row.Status,
		row.ResultVersion,
		row.CanonicalResultID,
		row.CreatedAt,
		row.UpdatedAt,
		row.ArchivedAt,
	)
	return &record, nil
}

func (r *Repository) CreateMatchWithSideSlots(ctx context.Context, actor StaffActor, sessionID uuid.UUID, matchIndex int, sideSlots []MatchSideInput, createdAt time.Time) (matchRecord, error) {
	return withQueriesTx(ctx, r.db, func(queries *store.Queries) (matchRecord, error) {
		match, err := createCompetitionMatchWithSideSlotsTx(ctx, queries, sessionID, matchIndex, MatchStatusDraft, sideSlots)
		if err != nil {
			return matchRecord{}, err
		}

		attribution := newStaffActionAttribution(actor, "competition_match.create", createdAt)
		attribution.CompetitionSessionID = uuidPtr(sessionID)
		attribution.CompetitionMatchID = uuidPtr(match.ID)
		if err := recordStaffActionAttributionTx(ctx, queries, attribution); err != nil {
			return matchRecord{}, err
		}

		return match, nil
	})
}

func (r *Repository) ArchiveMatch(ctx context.Context, actor StaffActor, sessionID uuid.UUID, matchID uuid.UUID, archivedAt time.Time) (matchRecord, error) {
	return withQueriesTx(ctx, r.db, func(queries *store.Queries) (matchRecord, error) {
		row, err := queries.ArchiveCompetitionMatch(ctx, store.ArchiveCompetitionMatchParams{
			ID:         matchID,
			ArchivedAt: timestamptz(archivedAt),
		})
		if err != nil {
			return matchRecord{}, err
		}

		record := buildMatchRecordValues(
			row.ID,
			row.CompetitionSessionID,
			row.MatchIndex,
			row.Status,
			row.ResultVersion,
			row.CanonicalResultID,
			row.CreatedAt,
			row.UpdatedAt,
			row.ArchivedAt,
		)
		attribution := newStaffActionAttribution(actor, "competition_match.archive", archivedAt)
		attribution.CompetitionSessionID = uuidPtr(sessionID)
		attribution.CompetitionMatchID = uuidPtr(matchID)
		if err := recordStaffActionAttributionTx(ctx, queries, attribution); err != nil {
			return matchRecord{}, err
		}

		return record, nil
	})
}

func (r *Repository) StartSession(ctx context.Context, actor StaffActor, session sessionRecord, updatedAt time.Time) (sessionRecord, error) {
	return withQueriesTx(ctx, r.db, func(queries *store.Queries) (sessionRecord, error) {
		matches, err := queries.ListCompetitionMatchesBySessionID(ctx, session.ID)
		if err != nil {
			return sessionRecord{}, err
		}

		updatedMatches, err := queries.UpdateCompetitionMatchStatusBySessionID(ctx, store.UpdateCompetitionMatchStatusBySessionIDParams{
			CompetitionSessionID: session.ID,
			Status:               MatchStatusInProgress,
			Status_2:             MatchStatusAssigned,
			UpdatedAt:            timestamptz(updatedAt),
		})
		if err != nil {
			return sessionRecord{}, err
		}
		if updatedMatches == 0 {
			return sessionRecord{}, pgx.ErrNoRows
		}

		row, err := queries.UpdateCompetitionSessionStatus(ctx, store.UpdateCompetitionSessionStatusParams{
			ID:          session.ID,
			OwnerUserID: session.OwnerUserID,
			Status:      SessionStatusInProgress,
			UpdatedAt:   timestamptz(updatedAt),
			Status_2:    SessionStatusAssigned,
		})
		if err != nil {
			return sessionRecord{}, err
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
		attribution := newStaffActionAttribution(actor, "competition_session.start", updatedAt)
		attribution.CompetitionSessionID = uuidPtr(session.ID)
		if err := recordStaffActionAttributionTx(ctx, queries, attribution); err != nil {
			return sessionRecord{}, err
		}
		for _, match := range matches {
			if match.Status != MatchStatusAssigned {
				continue
			}
			matchID := match.ID
			if err := recordLifecycleEventTx(ctx, queries, lifecycleEventRecord{
				Actor:                actor,
				CompetitionSessionID: session.ID,
				CompetitionMatchID:   &matchID,
				EventType:            "competition.match.started",
				OccurredAt:           updatedAt.UTC(),
			}); err != nil {
				return sessionRecord{}, err
			}
		}

		return record, nil
	})
}

func (r *Repository) ArchiveSession(ctx context.Context, actor StaffActor, session sessionRecord, updatedAt time.Time) (sessionRecord, error) {
	return withQueriesTx(ctx, r.db, func(queries *store.Queries) (sessionRecord, error) {
		switch session.Status {
		case SessionStatusAssigned:
			updatedMatches, err := queries.UpdateCompetitionMatchStatusBySessionID(ctx, store.UpdateCompetitionMatchStatusBySessionIDParams{
				CompetitionSessionID: session.ID,
				Status:               MatchStatusArchived,
				Status_2:             MatchStatusAssigned,
				UpdatedAt:            timestamptz(updatedAt),
			})
			if err != nil {
				return sessionRecord{}, err
			}
			if updatedMatches == 0 {
				return sessionRecord{}, pgx.ErrNoRows
			}
		case SessionStatusInProgress:
			updatedMatches, err := queries.UpdateCompetitionMatchStatusBySessionID(ctx, store.UpdateCompetitionMatchStatusBySessionIDParams{
				CompetitionSessionID: session.ID,
				Status:               MatchStatusArchived,
				Status_2:             MatchStatusInProgress,
				UpdatedAt:            timestamptz(updatedAt),
			})
			if err != nil {
				return sessionRecord{}, err
			}
			if updatedMatches == 0 {
				return sessionRecord{}, pgx.ErrNoRows
			}
		}

		row, err := queries.UpdateCompetitionSessionStatus(ctx, store.UpdateCompetitionSessionStatusParams{
			ID:          session.ID,
			OwnerUserID: session.OwnerUserID,
			Status:      SessionStatusArchived,
			UpdatedAt:   timestamptz(updatedAt),
			Status_2:    session.Status,
		})
		if err != nil {
			return sessionRecord{}, err
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
		attribution := newStaffActionAttribution(actor, "competition_session.archive", updatedAt)
		attribution.CompetitionSessionID = uuidPtr(session.ID)
		if err := recordStaffActionAttributionTx(ctx, queries, attribution); err != nil {
			return sessionRecord{}, err
		}

		return record, nil
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

	return buildMatchRecordValues(
		match.ID,
		match.CompetitionSessionID,
		match.MatchIndex,
		match.Status,
		match.ResultVersion,
		match.CanonicalResultID,
		match.CreatedAt,
		match.UpdatedAt,
		match.ArchivedAt,
	), nil
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

func buildMatchRecordValues(id uuid.UUID, sessionID uuid.UUID, matchIndex int32, status string, resultVersion int32, canonicalResultID pgtype.UUID, createdAt pgtype.Timestamptz, updatedAt pgtype.Timestamptz, archivedAt pgtype.Timestamptz) matchRecord {
	return matchRecord{
		ID:                id,
		SessionID:         sessionID,
		MatchIndex:        int(matchIndex),
		Status:            status,
		ResultVersion:     int(resultVersion),
		CanonicalResultID: uuidFromPgtype(canonicalResultID),
		CreatedAt:         createdAt.Time.UTC(),
		UpdatedAt:         updatedAt.Time.UTC(),
		ArchivedAt:        timePtr(archivedAt),
	}
}

func optionalUUID(value *uuid.UUID) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}

	return pgtype.UUID{Bytes: *value, Valid: true}
}

func uuidFromPgtype(value pgtype.UUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}

	parsed := uuid.UUID(value.Bytes)
	return &parsed
}

func optionalText(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func recordStaffActionAttributionTx(ctx context.Context, queries *store.Queries, attribution staffActionAttribution) error {
	_, err := queries.CreateCompetitionStaffActionAttribution(ctx, store.CreateCompetitionStaffActionAttributionParams{
		ActorUserID:              attribution.Actor.UserID,
		ActorRole:                string(attribution.Actor.Role),
		SessionID:                attribution.Actor.SessionID,
		Capability:               string(attribution.Actor.Capability),
		TrustedSurfaceKey:        attribution.Actor.TrustedSurfaceKey,
		TrustedSurfaceLabel:      optionalText(attribution.Actor.TrustedSurfaceLabel),
		Action:                   attribution.Action,
		CompetitionSessionID:     optionalUUID(attribution.CompetitionSessionID),
		CompetitionSessionTeamID: optionalUUID(attribution.CompetitionTeamID),
		CompetitionMatchID:       optionalUUID(attribution.CompetitionMatchID),
		SubjectUserID:            optionalUUID(attribution.SubjectUserID),
		OccurredAt:               timestamptz(attribution.OccurredAt),
	})
	return err
}

func withQueriesTx[T any](ctx context.Context, db *pgxpool.Pool, fn func(*store.Queries) (T, error)) (T, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		var zero T
		return zero, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	value, err := fn(store.New(tx))
	if err != nil {
		var zero T
		return zero, err
	}
	if err := tx.Commit(ctx); err != nil {
		var zero T
		return zero, err
	}

	return value, nil
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
