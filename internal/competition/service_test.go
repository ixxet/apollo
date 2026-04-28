package competition

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ixxet/apollo/internal/ares"
	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/store"
)

type stubStore struct {
	getUserByID              func(ctx context.Context, userID uuid.UUID) (*store.ApolloUser, error)
	getLobbyMembershipByUser func(ctx context.Context, userID uuid.UUID) (*store.ApolloLobbyMembership, error)
	getSportConfig           func(ctx context.Context, sportKey string) (*SportConfig, error)
	listFacilityCapabilities func(ctx context.Context) ([]FacilityCapability, error)
	listSessions             func(ctx context.Context) ([]sessionRecord, error)
	getSessionByID           func(ctx context.Context, sessionID uuid.UUID) (*sessionRecord, error)
	createSession            func(ctx context.Context, actor StaffActor, input CreateSessionInput, createdAt time.Time) (sessionRecord, error)
	openQueue                func(ctx context.Context, actor StaffActor, session sessionRecord, updatedAt time.Time) (sessionRecord, error)
	addQueueMember           func(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, input QueueMemberInput, joinedAt time.Time) error
	removeQueueMember        func(ctx context.Context, actor StaffActor, session sessionRecord, userID uuid.UUID, updatedAt time.Time) error
	updateQueueIntent        func(ctx context.Context, actor StaffActor, session sessionRecord, input UpdateQueueIntentInput, updatedAt time.Time) (sessionRecord, queueIntentRecord, error)
	listPreviewCandidates    func(ctx context.Context, sessionID uuid.UUID) ([]matchPreviewCandidateRecord, error)
	recordMatchPreview       func(ctx context.Context, actor StaffActor, session sessionRecord, preview ares.CompetitionMatchPreview, occurredAt time.Time) error
	assignQueue              func(ctx context.Context, actor StaffActor, session sessionRecord, input AssignSessionInput, sport SportConfig, queueMembers []queueRecord, assignedAt time.Time) (sessionRecord, error)
	startSession             func(ctx context.Context, actor StaffActor, session sessionRecord, updatedAt time.Time) (sessionRecord, error)
	archiveSession           func(ctx context.Context, actor StaffActor, session sessionRecord, updatedAt time.Time) (sessionRecord, error)
	countDraftMatchesByID    func(ctx context.Context, sessionID uuid.UUID) (int64, error)
	countQueueMembersByID    func(ctx context.Context, sessionID uuid.UUID) (int64, error)
	listQueueMembersByID     func(ctx context.Context, sessionID uuid.UUID) ([]queueRecord, error)
	listTeamsBySessionID     func(ctx context.Context, sessionID uuid.UUID) ([]teamRecord, error)
	getTeamByID              func(ctx context.Context, teamID uuid.UUID) (*teamRecord, error)
	createTeam               func(ctx context.Context, actor StaffActor, sessionID uuid.UUID, sideIndex int, createdAt time.Time) (teamRecord, error)
	deleteTeam               func(ctx context.Context, actor StaffActor, sessionID uuid.UUID, teamID uuid.UUID, deletedAt time.Time) (int64, error)
	countRosterMembersByTeam func(ctx context.Context, teamID uuid.UUID) (int64, error)
	sessionHasRosterMember   func(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID) (bool, error)
	listRosterMembersByID    func(ctx context.Context, sessionID uuid.UUID) ([]rosterRecord, error)
	createRosterMember       func(ctx context.Context, actor StaffActor, sessionID uuid.UUID, teamID uuid.UUID, userID uuid.UUID, slotIndex int, createdAt time.Time) (rosterRecord, error)
	deleteRosterMember       func(ctx context.Context, actor StaffActor, sessionID uuid.UUID, teamID uuid.UUID, userID uuid.UUID, deletedAt time.Time) (int64, error)
	teamHasMatchReference    func(ctx context.Context, teamID uuid.UUID) (bool, error)
	listMatchesBySessionID   func(ctx context.Context, sessionID uuid.UUID) ([]matchRecord, error)
	getMatchByID             func(ctx context.Context, matchID uuid.UUID) (*matchRecord, error)
	createMatchWithSideSlots func(ctx context.Context, actor StaffActor, sessionID uuid.UUID, matchIndex int, sideSlots []MatchSideInput, createdAt time.Time) (matchRecord, error)
	archiveMatch             func(ctx context.Context, actor StaffActor, sessionID uuid.UUID, matchID uuid.UUID, archivedAt time.Time) (matchRecord, error)
	listMatchSideSlotsByID   func(ctx context.Context, sessionID uuid.UUID) ([]matchSideSlotRecord, error)
	getMatchResultByID       func(ctx context.Context, matchID uuid.UUID) (*matchResultRecord, error)
	listMatchResultsByID     func(ctx context.Context, sessionID uuid.UUID) ([]matchResultSideRecord, error)
	recordMatchResult        func(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, match matchRecord, input RecordMatchResultInput, expectedResultVersion int, recordedAt time.Time) (matchResultRecord, error)
	finalizeMatchResult      func(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, match matchRecord, result matchResultRecord, expectedResultVersion int, finalizedAt time.Time) (matchResultRecord, error)
	disputeMatchResult       func(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, match matchRecord, result matchResultRecord, expectedResultVersion int, disputedAt time.Time) (matchResultRecord, error)
	correctMatchResult       func(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, match matchRecord, result matchResultRecord, input RecordMatchResultInput, expectedResultVersion int, correctedAt time.Time) (matchResultRecord, error)
	voidMatchResult          func(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, match matchRecord, result matchResultRecord, expectedResultVersion int, voidedAt time.Time) (matchResultRecord, error)
	listMemberRatingsByUser  func(ctx context.Context, userID uuid.UUID) ([]memberRatingRecord, error)
	listMemberStatRowsByUser func(ctx context.Context, userID uuid.UUID) ([]memberStatRowRecord, error)
	listMemberHistoryByUser  func(ctx context.Context, userID uuid.UUID) ([]memberHistoryRowRecord, error)
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

func (s stubStore) ListSessions(ctx context.Context) ([]sessionRecord, error) {
	return s.listSessions(ctx)
}

func (s stubStore) GetSessionByID(ctx context.Context, sessionID uuid.UUID) (*sessionRecord, error) {
	return s.getSessionByID(ctx, sessionID)
}

func (s stubStore) CreateSession(ctx context.Context, actor StaffActor, input CreateSessionInput, createdAt time.Time) (sessionRecord, error) {
	return s.createSession(ctx, actor, input, createdAt)
}

func (s stubStore) OpenQueue(ctx context.Context, actor StaffActor, session sessionRecord, updatedAt time.Time) (sessionRecord, error) {
	return s.openQueue(ctx, actor, session, updatedAt)
}

func (s stubStore) AddQueueMember(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, input QueueMemberInput, joinedAt time.Time) error {
	return s.addQueueMember(ctx, actor, session, sport, input, joinedAt)
}

func (s stubStore) RemoveQueueMember(ctx context.Context, actor StaffActor, session sessionRecord, userID uuid.UUID, updatedAt time.Time) error {
	return s.removeQueueMember(ctx, actor, session, userID, updatedAt)
}

func (s stubStore) UpdateQueueIntent(ctx context.Context, actor StaffActor, session sessionRecord, input UpdateQueueIntentInput, updatedAt time.Time) (sessionRecord, queueIntentRecord, error) {
	if s.updateQueueIntent == nil {
		return sessionRecord{}, queueIntentRecord{}, errors.New("unexpected UpdateQueueIntent call")
	}
	return s.updateQueueIntent(ctx, actor, session, input, updatedAt)
}

func (s stubStore) ListMatchPreviewCandidatesBySessionID(ctx context.Context, sessionID uuid.UUID) ([]matchPreviewCandidateRecord, error) {
	if s.listPreviewCandidates == nil {
		return nil, errors.New("unexpected ListMatchPreviewCandidatesBySessionID call")
	}
	return s.listPreviewCandidates(ctx, sessionID)
}

func (s stubStore) RecordMatchPreview(ctx context.Context, actor StaffActor, session sessionRecord, preview ares.CompetitionMatchPreview, occurredAt time.Time) error {
	if s.recordMatchPreview == nil {
		return errors.New("unexpected RecordMatchPreview call")
	}
	return s.recordMatchPreview(ctx, actor, session, preview, occurredAt)
}

func (s stubStore) AssignQueue(ctx context.Context, actor StaffActor, session sessionRecord, input AssignSessionInput, sport SportConfig, queueMembers []queueRecord, assignedAt time.Time) (sessionRecord, error) {
	return s.assignQueue(ctx, actor, session, input, sport, queueMembers, assignedAt)
}

func (s stubStore) StartSession(ctx context.Context, actor StaffActor, session sessionRecord, updatedAt time.Time) (sessionRecord, error) {
	return s.startSession(ctx, actor, session, updatedAt)
}

func (s stubStore) ArchiveSession(ctx context.Context, actor StaffActor, session sessionRecord, updatedAt time.Time) (sessionRecord, error) {
	return s.archiveSession(ctx, actor, session, updatedAt)
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

func (s stubStore) CreateTeam(ctx context.Context, actor StaffActor, sessionID uuid.UUID, sideIndex int, createdAt time.Time) (teamRecord, error) {
	return s.createTeam(ctx, actor, sessionID, sideIndex, createdAt)
}

func (s stubStore) DeleteTeam(ctx context.Context, actor StaffActor, sessionID uuid.UUID, teamID uuid.UUID, deletedAt time.Time) (int64, error) {
	return s.deleteTeam(ctx, actor, sessionID, teamID, deletedAt)
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

func (s stubStore) CreateRosterMember(ctx context.Context, actor StaffActor, sessionID uuid.UUID, teamID uuid.UUID, userID uuid.UUID, slotIndex int, createdAt time.Time) (rosterRecord, error) {
	return s.createRosterMember(ctx, actor, sessionID, teamID, userID, slotIndex, createdAt)
}

func (s stubStore) DeleteRosterMember(ctx context.Context, actor StaffActor, sessionID uuid.UUID, teamID uuid.UUID, userID uuid.UUID, deletedAt time.Time) (int64, error) {
	return s.deleteRosterMember(ctx, actor, sessionID, teamID, userID, deletedAt)
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

func (s stubStore) CreateMatchWithSideSlots(ctx context.Context, actor StaffActor, sessionID uuid.UUID, matchIndex int, sideSlots []MatchSideInput, createdAt time.Time) (matchRecord, error) {
	return s.createMatchWithSideSlots(ctx, actor, sessionID, matchIndex, sideSlots, createdAt)
}

func (s stubStore) ArchiveMatch(ctx context.Context, actor StaffActor, sessionID uuid.UUID, matchID uuid.UUID, archivedAt time.Time) (matchRecord, error) {
	return s.archiveMatch(ctx, actor, sessionID, matchID, archivedAt)
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

func (s stubStore) RecordMatchResult(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, match matchRecord, input RecordMatchResultInput, expectedResultVersion int, recordedAt time.Time) (matchResultRecord, error) {
	if s.recordMatchResult == nil {
		return matchResultRecord{}, errors.New("unexpected RecordMatchResult call")
	}
	return s.recordMatchResult(ctx, actor, session, sport, match, input, expectedResultVersion, recordedAt)
}

func (s stubStore) FinalizeMatchResult(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, match matchRecord, result matchResultRecord, expectedResultVersion int, finalizedAt time.Time) (matchResultRecord, error) {
	if s.finalizeMatchResult == nil {
		return matchResultRecord{}, errors.New("unexpected FinalizeMatchResult call")
	}
	return s.finalizeMatchResult(ctx, actor, session, sport, match, result, expectedResultVersion, finalizedAt)
}

func (s stubStore) DisputeMatchResult(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, match matchRecord, result matchResultRecord, expectedResultVersion int, disputedAt time.Time) (matchResultRecord, error) {
	if s.disputeMatchResult == nil {
		return matchResultRecord{}, errors.New("unexpected DisputeMatchResult call")
	}
	return s.disputeMatchResult(ctx, actor, session, sport, match, result, expectedResultVersion, disputedAt)
}

func (s stubStore) CorrectMatchResult(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, match matchRecord, result matchResultRecord, input RecordMatchResultInput, expectedResultVersion int, correctedAt time.Time) (matchResultRecord, error) {
	if s.correctMatchResult == nil {
		return matchResultRecord{}, errors.New("unexpected CorrectMatchResult call")
	}
	return s.correctMatchResult(ctx, actor, session, sport, match, result, input, expectedResultVersion, correctedAt)
}

func (s stubStore) VoidMatchResult(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, match matchRecord, result matchResultRecord, expectedResultVersion int, voidedAt time.Time) (matchResultRecord, error) {
	if s.voidMatchResult == nil {
		return matchResultRecord{}, errors.New("unexpected VoidMatchResult call")
	}
	return s.voidMatchResult(ctx, actor, session, sport, match, result, expectedResultVersion, voidedAt)
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

func (s stubStore) ListMemberHistoryByUserID(ctx context.Context, userID uuid.UUID) ([]memberHistoryRowRecord, error) {
	if s.listMemberHistoryByUser == nil {
		return nil, nil
	}
	return s.listMemberHistoryByUser(ctx, userID)
}

func TestAddRosterMemberMapsSchemaUniqueConflictToErrRosterConflict(t *testing.T) {
	sessionID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	teamID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	ownerUserID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	memberUserID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	actor := StaffActor{
		UserID:              ownerUserID,
		Role:                authz.RoleManager,
		SessionID:           uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		Capability:          authz.CapabilityCompetitionStructureManage,
		TrustedSurfaceKey:   "staff-console",
		TrustedSurfaceLabel: "staff-console",
	}

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
		listSessions: func(context.Context) ([]sessionRecord, error) {
			return nil, errors.New("unexpected ListSessions call")
		},
		getSessionByID: func(context.Context, uuid.UUID) (*sessionRecord, error) {
			return &sessionRecord{
				ID:                  sessionID,
				OwnerUserID:         ownerUserID,
				ParticipantsPerSide: 2,
				Status:              SessionStatusDraft,
			}, nil
		},
		createSession: func(context.Context, StaffActor, CreateSessionInput, time.Time) (sessionRecord, error) {
			return sessionRecord{}, errors.New("unexpected CreateSession call")
		},
		openQueue: func(context.Context, StaffActor, sessionRecord, time.Time) (sessionRecord, error) {
			return sessionRecord{}, errors.New("unexpected OpenQueue call")
		},
		addQueueMember: func(context.Context, StaffActor, sessionRecord, SportConfig, QueueMemberInput, time.Time) error {
			return errors.New("unexpected AddQueueMember call")
		},
		removeQueueMember: func(context.Context, StaffActor, sessionRecord, uuid.UUID, time.Time) error {
			return errors.New("unexpected RemoveQueueMember call")
		},
		assignQueue: func(context.Context, StaffActor, sessionRecord, AssignSessionInput, SportConfig, []queueRecord, time.Time) (sessionRecord, error) {
			return sessionRecord{}, errors.New("unexpected AssignQueue call")
		},
		startSession: func(context.Context, StaffActor, sessionRecord, time.Time) (sessionRecord, error) {
			return sessionRecord{}, errors.New("unexpected StartSession call")
		},
		archiveSession: func(context.Context, StaffActor, sessionRecord, time.Time) (sessionRecord, error) {
			return sessionRecord{}, errors.New("unexpected ArchiveSession call")
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
		createTeam: func(context.Context, StaffActor, uuid.UUID, int, time.Time) (teamRecord, error) {
			return teamRecord{}, errors.New("unexpected CreateTeam call")
		},
		deleteTeam: func(context.Context, StaffActor, uuid.UUID, uuid.UUID, time.Time) (int64, error) {
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
		createRosterMember: func(context.Context, StaffActor, uuid.UUID, uuid.UUID, uuid.UUID, int, time.Time) (rosterRecord, error) {
			return rosterRecord{}, &pgconn.PgError{
				Code:           "23505",
				ConstraintName: competitionTeamRosterMembersSessionUserUnique,
			}
		},
		deleteRosterMember: func(context.Context, StaffActor, uuid.UUID, uuid.UUID, uuid.UUID, time.Time) (int64, error) {
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
		createMatchWithSideSlots: func(context.Context, StaffActor, uuid.UUID, int, []MatchSideInput, time.Time) (matchRecord, error) {
			return matchRecord{}, errors.New("unexpected CreateMatchWithSideSlots call")
		},
		archiveMatch: func(context.Context, StaffActor, uuid.UUID, uuid.UUID, time.Time) (matchRecord, error) {
			return matchRecord{}, errors.New("unexpected ArchiveMatch call")
		},
		listMatchSideSlotsByID: func(context.Context, uuid.UUID) ([]matchSideSlotRecord, error) {
			return nil, errors.New("unexpected ListMatchSideSlotsBySessionID call")
		},
	})

	_, err := svc.AddRosterMember(context.Background(), actor, sessionID, teamID, AddRosterMemberInput{
		UserID:    memberUserID,
		SlotIndex: 1,
	})
	if !errors.Is(err, ErrRosterConflict) {
		t.Fatalf("AddRosterMember() error = %v, want %v", err, ErrRosterConflict)
	}
}
