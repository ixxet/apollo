package competition

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ixxet/apollo/internal/store"
)

type stubStore struct {
	getUserByID              func(ctx context.Context, userID uuid.UUID) (*store.ApolloUser, error)
	getLobbyMembershipByUser func(ctx context.Context, userID uuid.UUID) (*store.ApolloLobbyMembership, error)
	getSportConfig           func(ctx context.Context, sportKey string) (*SportConfig, error)
	listFacilityCapabilities func(ctx context.Context) ([]FacilityCapability, error)
	listSessionsByOwner      func(ctx context.Context, ownerUserID uuid.UUID) ([]sessionRecord, error)
	getSessionByIDForOwner   func(ctx context.Context, sessionID uuid.UUID, ownerUserID uuid.UUID) (*sessionRecord, error)
	createSession            func(ctx context.Context, ownerUserID uuid.UUID, input CreateSessionInput) (sessionRecord, error)
	openQueue                func(ctx context.Context, sessionID uuid.UUID, ownerUserID uuid.UUID, updatedAt time.Time) (sessionRecord, error)
	updateSessionStatus      func(ctx context.Context, sessionID uuid.UUID, ownerUserID uuid.UUID, fromStatus string, toStatus string, updatedAt time.Time) (sessionRecord, error)
	addQueueMember           func(ctx context.Context, sessionID uuid.UUID, ownerUserID uuid.UUID, userID uuid.UUID, joinedAt time.Time) error
	removeQueueMember        func(ctx context.Context, sessionID uuid.UUID, ownerUserID uuid.UUID, userID uuid.UUID, updatedAt time.Time) error
	assignQueue              func(ctx context.Context, ownerUserID uuid.UUID, session sessionRecord, input AssignSessionInput, sport SportConfig, queueMembers []queueRecord, assignedAt time.Time) (sessionRecord, error)
	countDraftMatchesByID    func(ctx context.Context, sessionID uuid.UUID) (int64, error)
	countQueueMembersByID    func(ctx context.Context, sessionID uuid.UUID) (int64, error)
	listQueueMembersByID     func(ctx context.Context, sessionID uuid.UUID) ([]queueRecord, error)
	listTeamsBySessionID     func(ctx context.Context, sessionID uuid.UUID) ([]teamRecord, error)
	getTeamByID              func(ctx context.Context, teamID uuid.UUID) (*teamRecord, error)
	createTeam               func(ctx context.Context, sessionID uuid.UUID, sideIndex int) (teamRecord, error)
	deleteTeam               func(ctx context.Context, teamID uuid.UUID) (int64, error)
	countRosterMembersByTeam func(ctx context.Context, teamID uuid.UUID) (int64, error)
	sessionHasRosterMember   func(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID) (bool, error)
	listRosterMembersByID    func(ctx context.Context, sessionID uuid.UUID) ([]rosterRecord, error)
	createRosterMember       func(ctx context.Context, sessionID uuid.UUID, teamID uuid.UUID, userID uuid.UUID, slotIndex int) (rosterRecord, error)
	deleteRosterMember       func(ctx context.Context, teamID uuid.UUID, userID uuid.UUID) (int64, error)
	teamHasMatchReference    func(ctx context.Context, teamID uuid.UUID) (bool, error)
	listMatchesBySessionID   func(ctx context.Context, sessionID uuid.UUID) ([]matchRecord, error)
	getMatchByID             func(ctx context.Context, matchID uuid.UUID) (*matchRecord, error)
	createMatchWithSideSlots func(ctx context.Context, sessionID uuid.UUID, matchIndex int, sideSlots []MatchSideInput) (matchRecord, error)
	archiveMatch             func(ctx context.Context, matchID uuid.UUID, archivedAt time.Time) (matchRecord, error)
	updateMatchStatusesByID  func(ctx context.Context, sessionID uuid.UUID, fromStatus string, toStatus string, updatedAt time.Time) (int64, error)
	listMatchSideSlotsByID   func(ctx context.Context, sessionID uuid.UUID) ([]matchSideSlotRecord, error)
	getMatchResultByID       func(ctx context.Context, matchID uuid.UUID) (*matchResultRecord, error)
	listMatchResultsByID     func(ctx context.Context, sessionID uuid.UUID) ([]matchResultSideRecord, error)
	recordMatchResult        func(ctx context.Context, ownerUserID uuid.UUID, session sessionRecord, sport SportConfig, match matchRecord, input RecordMatchResultInput, recordedAt time.Time) error
	listMemberRatingsByUser  func(ctx context.Context, userID uuid.UUID) ([]memberRatingRecord, error)
	listMemberStatRowsByUser func(ctx context.Context, userID uuid.UUID) ([]memberStatRowRecord, error)
}

func (s stubStore) GetUserByID(ctx context.Context, userID uuid.UUID) (*store.ApolloUser, error) {
	return s.getUserByID(ctx, userID)
}

func (s stubStore) GetLobbyMembershipByUserID(ctx context.Context, userID uuid.UUID) (*store.ApolloLobbyMembership, error) {
	return s.getLobbyMembershipByUser(ctx, userID)
}

func (s stubStore) GetSportConfig(ctx context.Context, sportKey string) (*SportConfig, error) {
	return s.getSportConfig(ctx, sportKey)
}

func (s stubStore) ListFacilityCapabilities(ctx context.Context) ([]FacilityCapability, error) {
	return s.listFacilityCapabilities(ctx)
}

func (s stubStore) ListSessionsByOwner(ctx context.Context, ownerUserID uuid.UUID) ([]sessionRecord, error) {
	return s.listSessionsByOwner(ctx, ownerUserID)
}

func (s stubStore) GetSessionByIDForOwner(ctx context.Context, sessionID uuid.UUID, ownerUserID uuid.UUID) (*sessionRecord, error) {
	return s.getSessionByIDForOwner(ctx, sessionID, ownerUserID)
}

func (s stubStore) CreateSession(ctx context.Context, ownerUserID uuid.UUID, input CreateSessionInput) (sessionRecord, error) {
	return s.createSession(ctx, ownerUserID, input)
}

func (s stubStore) OpenQueue(ctx context.Context, sessionID uuid.UUID, ownerUserID uuid.UUID, updatedAt time.Time) (sessionRecord, error) {
	return s.openQueue(ctx, sessionID, ownerUserID, updatedAt)
}

func (s stubStore) UpdateSessionStatus(ctx context.Context, sessionID uuid.UUID, ownerUserID uuid.UUID, fromStatus string, toStatus string, updatedAt time.Time) (sessionRecord, error) {
	return s.updateSessionStatus(ctx, sessionID, ownerUserID, fromStatus, toStatus, updatedAt)
}

func (s stubStore) AddQueueMember(ctx context.Context, sessionID uuid.UUID, ownerUserID uuid.UUID, userID uuid.UUID, joinedAt time.Time) error {
	return s.addQueueMember(ctx, sessionID, ownerUserID, userID, joinedAt)
}

func (s stubStore) RemoveQueueMember(ctx context.Context, sessionID uuid.UUID, ownerUserID uuid.UUID, userID uuid.UUID, updatedAt time.Time) error {
	return s.removeQueueMember(ctx, sessionID, ownerUserID, userID, updatedAt)
}

func (s stubStore) AssignQueue(ctx context.Context, ownerUserID uuid.UUID, session sessionRecord, input AssignSessionInput, sport SportConfig, queueMembers []queueRecord, assignedAt time.Time) (sessionRecord, error) {
	return s.assignQueue(ctx, ownerUserID, session, input, sport, queueMembers, assignedAt)
}

func (s stubStore) CountDraftMatchesBySessionID(ctx context.Context, sessionID uuid.UUID) (int64, error) {
	return s.countDraftMatchesByID(ctx, sessionID)
}

func (s stubStore) CountQueueMembersBySessionID(ctx context.Context, sessionID uuid.UUID) (int64, error) {
	return s.countQueueMembersByID(ctx, sessionID)
}

func (s stubStore) ListQueueMembersBySessionID(ctx context.Context, sessionID uuid.UUID) ([]queueRecord, error) {
	return s.listQueueMembersByID(ctx, sessionID)
}

func (s stubStore) ListTeamsBySessionID(ctx context.Context, sessionID uuid.UUID) ([]teamRecord, error) {
	return s.listTeamsBySessionID(ctx, sessionID)
}

func (s stubStore) GetTeamByID(ctx context.Context, teamID uuid.UUID) (*teamRecord, error) {
	return s.getTeamByID(ctx, teamID)
}

func (s stubStore) CreateTeam(ctx context.Context, sessionID uuid.UUID, sideIndex int) (teamRecord, error) {
	return s.createTeam(ctx, sessionID, sideIndex)
}

func (s stubStore) DeleteTeam(ctx context.Context, teamID uuid.UUID) (int64, error) {
	return s.deleteTeam(ctx, teamID)
}

func (s stubStore) CountRosterMembersByTeamID(ctx context.Context, teamID uuid.UUID) (int64, error) {
	return s.countRosterMembersByTeam(ctx, teamID)
}

func (s stubStore) SessionHasRosterMemberUser(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID) (bool, error) {
	return s.sessionHasRosterMember(ctx, sessionID, userID)
}

func (s stubStore) ListRosterMembersBySessionID(ctx context.Context, sessionID uuid.UUID) ([]rosterRecord, error) {
	return s.listRosterMembersByID(ctx, sessionID)
}

func (s stubStore) CreateRosterMember(ctx context.Context, sessionID uuid.UUID, teamID uuid.UUID, userID uuid.UUID, slotIndex int) (rosterRecord, error) {
	return s.createRosterMember(ctx, sessionID, teamID, userID, slotIndex)
}

func (s stubStore) DeleteRosterMember(ctx context.Context, teamID uuid.UUID, userID uuid.UUID) (int64, error) {
	return s.deleteRosterMember(ctx, teamID, userID)
}

func (s stubStore) TeamHasMatchReference(ctx context.Context, teamID uuid.UUID) (bool, error) {
	return s.teamHasMatchReference(ctx, teamID)
}

func (s stubStore) ListMatchesBySessionID(ctx context.Context, sessionID uuid.UUID) ([]matchRecord, error) {
	return s.listMatchesBySessionID(ctx, sessionID)
}

func (s stubStore) GetMatchByID(ctx context.Context, matchID uuid.UUID) (*matchRecord, error) {
	return s.getMatchByID(ctx, matchID)
}

func (s stubStore) CreateMatchWithSideSlots(ctx context.Context, sessionID uuid.UUID, matchIndex int, sideSlots []MatchSideInput) (matchRecord, error) {
	return s.createMatchWithSideSlots(ctx, sessionID, matchIndex, sideSlots)
}

func (s stubStore) ArchiveMatch(ctx context.Context, matchID uuid.UUID, archivedAt time.Time) (matchRecord, error) {
	return s.archiveMatch(ctx, matchID, archivedAt)
}

func (s stubStore) UpdateMatchStatusesBySessionID(ctx context.Context, sessionID uuid.UUID, fromStatus string, toStatus string, updatedAt time.Time) (int64, error) {
	return s.updateMatchStatusesByID(ctx, sessionID, fromStatus, toStatus, updatedAt)
}

func (s stubStore) ListMatchSideSlotsBySessionID(ctx context.Context, sessionID uuid.UUID) ([]matchSideSlotRecord, error) {
	return s.listMatchSideSlotsByID(ctx, sessionID)
}

func (s stubStore) GetMatchResultByMatchID(ctx context.Context, matchID uuid.UUID) (*matchResultRecord, error) {
	if s.getMatchResultByID == nil {
		return nil, nil
	}
	return s.getMatchResultByID(ctx, matchID)
}

func (s stubStore) ListMatchResultsBySessionID(ctx context.Context, sessionID uuid.UUID) ([]matchResultSideRecord, error) {
	if s.listMatchResultsByID == nil {
		return nil, nil
	}
	return s.listMatchResultsByID(ctx, sessionID)
}

func (s stubStore) RecordMatchResult(ctx context.Context, ownerUserID uuid.UUID, session sessionRecord, sport SportConfig, match matchRecord, input RecordMatchResultInput, recordedAt time.Time) error {
	if s.recordMatchResult == nil {
		return errors.New("unexpected RecordMatchResult call")
	}
	return s.recordMatchResult(ctx, ownerUserID, session, sport, match, input, recordedAt)
}

func (s stubStore) ListMemberRatingsByUserID(ctx context.Context, userID uuid.UUID) ([]memberRatingRecord, error) {
	if s.listMemberRatingsByUser == nil {
		return nil, nil
	}
	return s.listMemberRatingsByUser(ctx, userID)
}

func (s stubStore) ListMemberStatRowsByUserID(ctx context.Context, userID uuid.UUID) ([]memberStatRowRecord, error) {
	if s.listMemberStatRowsByUser == nil {
		return nil, nil
	}
	return s.listMemberStatRowsByUser(ctx, userID)
}

func TestAddRosterMemberMapsSchemaUniqueConflictToErrRosterConflict(t *testing.T) {
	sessionID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	teamID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	ownerUserID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	memberUserID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	svc := NewService(stubStore{
		getUserByID: func(context.Context, uuid.UUID) (*store.ApolloUser, error) {
			return &store.ApolloUser{ID: memberUserID}, nil
		},
		getLobbyMembershipByUser: func(context.Context, uuid.UUID) (*store.ApolloLobbyMembership, error) {
			return nil, errors.New("unexpected GetLobbyMembershipByUserID call")
		},
		getSportConfig: func(context.Context, string) (*SportConfig, error) {
			return nil, errors.New("unexpected GetSportConfig call")
		},
		listFacilityCapabilities: func(context.Context) ([]FacilityCapability, error) {
			return nil, errors.New("unexpected ListFacilityCapabilities call")
		},
		listSessionsByOwner: func(context.Context, uuid.UUID) ([]sessionRecord, error) {
			return nil, errors.New("unexpected ListSessionsByOwner call")
		},
		getSessionByIDForOwner: func(context.Context, uuid.UUID, uuid.UUID) (*sessionRecord, error) {
			return &sessionRecord{
				ID:                  sessionID,
				OwnerUserID:         ownerUserID,
				ParticipantsPerSide: 2,
				Status:              SessionStatusDraft,
			}, nil
		},
		createSession: func(context.Context, uuid.UUID, CreateSessionInput) (sessionRecord, error) {
			return sessionRecord{}, errors.New("unexpected CreateSession call")
		},
		openQueue: func(context.Context, uuid.UUID, uuid.UUID, time.Time) (sessionRecord, error) {
			return sessionRecord{}, errors.New("unexpected OpenQueue call")
		},
		updateSessionStatus: func(context.Context, uuid.UUID, uuid.UUID, string, string, time.Time) (sessionRecord, error) {
			return sessionRecord{}, errors.New("unexpected UpdateSessionStatus call")
		},
		addQueueMember: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time) error {
			return errors.New("unexpected AddQueueMember call")
		},
		removeQueueMember: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, time.Time) error {
			return errors.New("unexpected RemoveQueueMember call")
		},
		assignQueue: func(context.Context, uuid.UUID, sessionRecord, AssignSessionInput, SportConfig, []queueRecord, time.Time) (sessionRecord, error) {
			return sessionRecord{}, errors.New("unexpected AssignQueue call")
		},
		countDraftMatchesByID: func(context.Context, uuid.UUID) (int64, error) {
			return 0, errors.New("unexpected CountDraftMatchesBySessionID call")
		},
		countQueueMembersByID: func(context.Context, uuid.UUID) (int64, error) {
			return 0, errors.New("unexpected CountQueueMembersBySessionID call")
		},
		listQueueMembersByID: func(context.Context, uuid.UUID) ([]queueRecord, error) {
			return nil, errors.New("unexpected ListQueueMembersBySessionID call")
		},
		listTeamsBySessionID: func(context.Context, uuid.UUID) ([]teamRecord, error) {
			return nil, errors.New("unexpected ListTeamsBySessionID call")
		},
		getTeamByID: func(context.Context, uuid.UUID) (*teamRecord, error) {
			return &teamRecord{ID: teamID, SessionID: sessionID}, nil
		},
		createTeam: func(context.Context, uuid.UUID, int) (teamRecord, error) {
			return teamRecord{}, errors.New("unexpected CreateTeam call")
		},
		deleteTeam: func(context.Context, uuid.UUID) (int64, error) {
			return 0, errors.New("unexpected DeleteTeam call")
		},
		countRosterMembersByTeam: func(context.Context, uuid.UUID) (int64, error) {
			return 0, nil
		},
		sessionHasRosterMember: func(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
			return false, nil
		},
		listRosterMembersByID: func(context.Context, uuid.UUID) ([]rosterRecord, error) {
			return nil, errors.New("unexpected ListRosterMembersBySessionID call")
		},
		createRosterMember: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, int) (rosterRecord, error) {
			return rosterRecord{}, &pgconn.PgError{
				Code:           "23505",
				ConstraintName: competitionTeamRosterMembersSessionUserUnique,
			}
		},
		deleteRosterMember: func(context.Context, uuid.UUID, uuid.UUID) (int64, error) {
			return 0, errors.New("unexpected DeleteRosterMember call")
		},
		teamHasMatchReference: func(context.Context, uuid.UUID) (bool, error) {
			return false, nil
		},
		listMatchesBySessionID: func(context.Context, uuid.UUID) ([]matchRecord, error) {
			return nil, errors.New("unexpected ListMatchesBySessionID call")
		},
		getMatchByID: func(context.Context, uuid.UUID) (*matchRecord, error) {
			return nil, errors.New("unexpected GetMatchByID call")
		},
		createMatchWithSideSlots: func(context.Context, uuid.UUID, int, []MatchSideInput) (matchRecord, error) {
			return matchRecord{}, errors.New("unexpected CreateMatchWithSideSlots call")
		},
		archiveMatch: func(context.Context, uuid.UUID, time.Time) (matchRecord, error) {
			return matchRecord{}, errors.New("unexpected ArchiveMatch call")
		},
		updateMatchStatusesByID: func(context.Context, uuid.UUID, string, string, time.Time) (int64, error) {
			return 0, errors.New("unexpected UpdateMatchStatusesBySessionID call")
		},
		listMatchSideSlotsByID: func(context.Context, uuid.UUID) ([]matchSideSlotRecord, error) {
			return nil, errors.New("unexpected ListMatchSideSlotsBySessionID call")
		},
	})

	_, err := svc.AddRosterMember(context.Background(), ownerUserID, sessionID, teamID, AddRosterMemberInput{
		UserID:    memberUserID,
		SlotIndex: 1,
	})
	if !errors.Is(err, ErrRosterConflict) {
		t.Fatalf("AddRosterMember() error = %v, want %v", err, ErrRosterConflict)
	}
}
