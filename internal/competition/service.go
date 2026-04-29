package competition

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ixxet/apollo/internal/ares"
	"github.com/ixxet/apollo/internal/eligibility"
	"github.com/ixxet/apollo/internal/membership"
	"github.com/ixxet/apollo/internal/profile"
	"github.com/ixxet/apollo/internal/rating"
	"github.com/ixxet/apollo/internal/store"
)

const (
	SessionStatusDraft      = "draft"
	SessionStatusQueueOpen  = "queue_open"
	SessionStatusAssigned   = "assigned"
	SessionStatusInProgress = "in_progress"
	SessionStatusCompleted  = "completed"
	SessionStatusArchived   = "archived"
	MatchStatusDraft        = "draft"
	MatchStatusAssigned     = "assigned"
	MatchStatusInProgress   = "in_progress"
	MatchStatusCompleted    = "completed"
	MatchStatusArchived     = "archived"
	ResultStatusRecorded    = "recorded"
	ResultStatusFinalized   = "finalized"
	ResultStatusDisputed    = "disputed"
	ResultStatusCorrected   = "corrected"
	ResultStatusVoided      = "voided"
	DisputeStatusNone       = "none"
	DisputeStatusDisputed   = "disputed"
	DisputeStatusResolved   = "resolved"

	competitionTeamRosterMembersPrimaryKey        = "competition_team_roster_members_pkey"
	competitionTeamRosterMembersSessionUserUnique = "competition_team_roster_members_session_user_unique"
	competitionTeamRosterMembersTeamSlotUnique    = "competition_team_roster_members_team_slot_unique"
	competitionSessionQueueMembersPrimaryKey      = "competition_session_queue_members_pkey"
)

var (
	ErrSessionNotFound          = errors.New("competition session not found")
	ErrTeamNotFound             = errors.New("competition team not found")
	ErrMatchNotFound            = errors.New("competition match not found")
	ErrRosterMemberNotFound     = errors.New("competition roster member not found")
	ErrUserNotFound             = errors.New("competition user not found")
	ErrSportNotFound            = errors.New("sport not found")
	ErrSessionNameRequired      = errors.New("competition session display_name is required")
	ErrParticipantsPerSide      = errors.New("participants_per_side is outside the sport range")
	ErrFacilityUnsupported      = errors.New("facility is not supported for the selected sport")
	ErrZoneUnsupported          = errors.New("zone is not supported for the selected sport facility")
	ErrSessionArchived          = errors.New("competition session is archived")
	ErrDuplicateSession         = errors.New("competition session already exists for this owner and sport")
	ErrDuplicateTeam            = errors.New("competition team side_index already exists in this session")
	ErrTeamSideIndexInvalid     = errors.New("team side_index must be positive")
	ErrRosterSlotIndexInvalid   = errors.New("roster slot_index must be positive")
	ErrRosterSlotOutOfRange     = errors.New("roster slot_index exceeds participants_per_side")
	ErrRosterConflict           = errors.New("user already belongs to a team in this session")
	ErrDuplicateRosterSlot      = errors.New("roster slot_index already exists on this team")
	ErrTeamReferencedByMatch    = errors.New("competition team is already referenced by a match")
	ErrTeamSizeMismatch         = errors.New("competition team roster size does not match participants_per_side")
	ErrMatchIndexInvalid        = errors.New("match_index must be positive")
	ErrDuplicateMatch           = errors.New("competition match already exists at this match_index")
	ErrMatchSideCountMismatch   = errors.New("match side count does not match sport sides_per_match")
	ErrMatchSideIndexInvalid    = errors.New("match side_index values must be positive and contiguous")
	ErrDuplicateMatchSideIndex  = errors.New("match side_index values must be unique")
	ErrDuplicateMatchTeam       = errors.New("match cannot reference the same team twice")
	ErrQueueClosed              = errors.New("competition session queue is not open")
	ErrQueueMemberAlreadyJoined = errors.New("competition queue member is already joined")
	ErrQueueMemberNotFound      = errors.New("competition queue member not found")
	ErrQueueMemberNotJoined     = errors.New("competition queue member is not joined in lobby")
	ErrQueueMemberIneligible    = errors.New("competition queue member is not eligible")
	ErrQueueCapacityReached     = errors.New("competition session queue is full")
	ErrQueueIntentTier          = errors.New("competition queue intent tier is invalid")
	ErrQueueIntentNotFound      = errors.New("competition queue intent not found")
	ErrQueueVersionRequired     = errors.New("expected_queue_version must be positive")
	ErrQueueStateStale          = errors.New("competition session queue state is stale")
	ErrQueueNotReady            = errors.New("competition session queue is not ready for assignment")
	ErrQueueNotEmpty            = errors.New("competition session queue must be empty")
	ErrExecutionAlreadySeeded   = errors.New("competition session already has execution containers")
	ErrInvalidSessionTransition = errors.New("competition session transition is invalid")
	ErrSessionHasDraftMatches   = errors.New("competition session still has draft matches")
	ErrMatchArchived            = errors.New("competition match is archived")
	ErrMatchNotInProgress       = errors.New("competition match is not in progress")
	ErrMatchResultRecorded      = errors.New("competition match result is already recorded")
	ErrMatchResultNotFound      = errors.New("competition match result not found")
	ErrMatchResultVersion       = errors.New("expected_result_version is required")
	ErrMatchResultStateStale    = errors.New("competition match result state is stale")
	ErrMatchResultNotRecorded   = errors.New("competition match result is not recorded")
	ErrMatchResultNotCanonical  = errors.New("competition match result is not canonical")
	ErrMatchResultNotFinal      = errors.New("competition match result is not finalized or corrected")
	ErrMatchResultVoided        = errors.New("competition match result is voided")
	ErrMatchResultSideCount     = errors.New("competition match result side count does not match match side slots")
	ErrMatchResultSideIndex     = errors.New("competition match result side_index values must be positive and contiguous")
	ErrMatchResultTeamMismatch  = errors.New("competition match result does not match match side slots")
	ErrMatchResultOutcome       = errors.New("competition match result outcome is invalid")
	ErrMatchResultShape         = errors.New("competition match result outcomes are invalid")
)

type Clock func() time.Time

type Store interface {
	GetUserByID(ctx context.Context, userID uuid.UUID) (*store.ApolloUser, error)
	GetLobbyMembershipByUserID(ctx context.Context, userID uuid.UUID) (*store.ApolloLobbyMembership, error)
	GetSportConfig(ctx context.Context, sportKey string) (*SportConfig, error)
	ListFacilityCapabilities(ctx context.Context) ([]FacilityCapability, error)
	ListSessions(ctx context.Context) ([]sessionRecord, error)
	GetSessionByID(ctx context.Context, sessionID uuid.UUID) (*sessionRecord, error)
	CreateSession(ctx context.Context, actor StaffActor, input CreateSessionInput, createdAt time.Time) (sessionRecord, error)
	OpenQueue(ctx context.Context, actor StaffActor, session sessionRecord, updatedAt time.Time) (sessionRecord, error)
	AddQueueMember(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, input QueueMemberInput, joinedAt time.Time) error
	RemoveQueueMember(ctx context.Context, actor StaffActor, session sessionRecord, userID uuid.UUID, updatedAt time.Time) error
	UpdateQueueIntent(ctx context.Context, actor StaffActor, session sessionRecord, input UpdateQueueIntentInput, updatedAt time.Time) (sessionRecord, queueIntentRecord, error)
	ListMatchPreviewCandidatesBySessionID(ctx context.Context, sessionID uuid.UUID) ([]matchPreviewCandidateRecord, error)
	RecordMatchPreview(ctx context.Context, actor StaffActor, session sessionRecord, preview ares.CompetitionMatchPreview, occurredAt time.Time) error
	AssignQueue(ctx context.Context, actor StaffActor, session sessionRecord, input AssignSessionInput, sport SportConfig, queueMembers []queueRecord, assignedAt time.Time) (sessionRecord, error)
	StartSession(ctx context.Context, actor StaffActor, session sessionRecord, updatedAt time.Time) (sessionRecord, error)
	ArchiveSession(ctx context.Context, actor StaffActor, session sessionRecord, updatedAt time.Time) (sessionRecord, error)
	CountDraftMatchesBySessionID(ctx context.Context, sessionID uuid.UUID) (int64, error)
	CountQueueMembersBySessionID(ctx context.Context, sessionID uuid.UUID) (int64, error)
	ListQueueMembersBySessionID(ctx context.Context, sessionID uuid.UUID) ([]queueRecord, error)
	ListTeamsBySessionID(ctx context.Context, sessionID uuid.UUID) ([]teamRecord, error)
	GetTeamByID(ctx context.Context, teamID uuid.UUID) (*teamRecord, error)
	CreateTeam(ctx context.Context, actor StaffActor, sessionID uuid.UUID, sideIndex int, createdAt time.Time) (teamRecord, error)
	DeleteTeam(ctx context.Context, actor StaffActor, sessionID uuid.UUID, teamID uuid.UUID, deletedAt time.Time) (int64, error)
	CountRosterMembersByTeamID(ctx context.Context, teamID uuid.UUID) (int64, error)
	SessionHasRosterMemberUser(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID) (bool, error)
	ListRosterMembersBySessionID(ctx context.Context, sessionID uuid.UUID) ([]rosterRecord, error)
	CreateRosterMember(ctx context.Context, actor StaffActor, sessionID uuid.UUID, teamID uuid.UUID, userID uuid.UUID, slotIndex int, createdAt time.Time) (rosterRecord, error)
	DeleteRosterMember(ctx context.Context, actor StaffActor, sessionID uuid.UUID, teamID uuid.UUID, userID uuid.UUID, deletedAt time.Time) (int64, error)
	TeamHasMatchReference(ctx context.Context, teamID uuid.UUID) (bool, error)
	ListMatchesBySessionID(ctx context.Context, sessionID uuid.UUID) ([]matchRecord, error)
	GetMatchByID(ctx context.Context, matchID uuid.UUID) (*matchRecord, error)
	CreateMatchWithSideSlots(ctx context.Context, actor StaffActor, sessionID uuid.UUID, matchIndex int, sideSlots []MatchSideInput, createdAt time.Time) (matchRecord, error)
	ArchiveMatch(ctx context.Context, actor StaffActor, sessionID uuid.UUID, matchID uuid.UUID, archivedAt time.Time) (matchRecord, error)
	ListMatchSideSlotsBySessionID(ctx context.Context, sessionID uuid.UUID) ([]matchSideSlotRecord, error)
	GetMatchResultByMatchID(ctx context.Context, matchID uuid.UUID) (*matchResultRecord, error)
	ListMatchResultsBySessionID(ctx context.Context, sessionID uuid.UUID) ([]matchResultSideRecord, error)
	RecordMatchResult(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, match matchRecord, input RecordMatchResultInput, expectedResultVersion int, recordedAt time.Time) (matchResultRecord, error)
	FinalizeMatchResult(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, match matchRecord, result matchResultRecord, expectedResultVersion int, finalizedAt time.Time) (matchResultRecord, error)
	DisputeMatchResult(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, match matchRecord, result matchResultRecord, expectedResultVersion int, disputedAt time.Time) (matchResultRecord, error)
	CorrectMatchResult(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, match matchRecord, result matchResultRecord, input RecordMatchResultInput, expectedResultVersion int, correctedAt time.Time) (matchResultRecord, error)
	VoidMatchResult(ctx context.Context, actor StaffActor, session sessionRecord, sport SportConfig, match matchRecord, result matchResultRecord, expectedResultVersion int, voidedAt time.Time) (matchResultRecord, error)
	ListMemberRatingsByUserID(ctx context.Context, userID uuid.UUID) ([]memberRatingRecord, error)
	ListMemberStatRowsByUserID(ctx context.Context, userID uuid.UUID) ([]memberStatRowRecord, error)
	ListMemberHistoryByUserID(ctx context.Context, userID uuid.UUID) ([]memberHistoryRowRecord, error)
	GetPublicCompetitionReadiness(ctx context.Context) (publicCompetitionReadinessRecord, error)
	ListPublicCompetitionLeaderboard(ctx context.Context, input PublicCompetitionLeaderboardInput) ([]publicCompetitionLeaderboardRowRecord, error)
	ListGameIdentityProjectionRows(ctx context.Context, input GameIdentityProjectionInput) ([]gameIdentityProjectionRowRecord, error)
}

type Service struct {
	repository Store
	now        Clock
}

type SportConfig struct {
	CompetitionMode        string
	SportKey               string
	SidesPerMatch          int
	ParticipantsPerSideMin int
	ParticipantsPerSideMax int
}

type FacilityCapability struct {
	SportKey    string
	FacilityKey string
	ZoneKeys    []string
}

type SessionSummary struct {
	ID                  uuid.UUID  `json:"id"`
	DisplayName         string     `json:"display_name"`
	SportKey            string     `json:"sport_key"`
	FacilityKey         string     `json:"facility_key"`
	ZoneKey             *string    `json:"zone_key,omitempty"`
	ParticipantsPerSide int        `json:"participants_per_side"`
	QueueVersion        int        `json:"queue_version"`
	Status              string     `json:"status"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	ArchivedAt          *time.Time `json:"archived_at,omitempty"`
}

type Session struct {
	SessionSummary
	Queue     []QueueMember `json:"queue"`
	Teams     []Team        `json:"teams"`
	Matches   []Match       `json:"matches"`
	Standings []Standing    `json:"standings"`
}

type QueueMember struct {
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
	JoinedAt    time.Time `json:"joined_at"`
}

type Team struct {
	ID        uuid.UUID      `json:"id"`
	SideIndex int            `json:"side_index"`
	CreatedAt time.Time      `json:"created_at"`
	Roster    []RosterMember `json:"roster"`
}

type RosterMember struct {
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
	SlotIndex   int       `json:"slot_index"`
	CreatedAt   time.Time `json:"created_at"`
}

type Match struct {
	ID                uuid.UUID      `json:"id"`
	MatchIndex        int            `json:"match_index"`
	Status            string         `json:"status"`
	MatchStatus       string         `json:"match_status"`
	ResultVersion     int            `json:"result_version"`
	CanonicalResultID *uuid.UUID     `json:"canonical_result_id,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	ArchivedAt        *time.Time     `json:"archived_at,omitempty"`
	SideSlots         []MatchSideRef `json:"side_slots"`
	Result            *MatchResult   `json:"result,omitempty"`
}

type MatchSideRef struct {
	TeamID    uuid.UUID `json:"team_id"`
	SideIndex int       `json:"side_index"`
	CreatedAt time.Time `json:"created_at"`
}

type MatchResult struct {
	ID                 uuid.UUID         `json:"id"`
	CompetitionMatchID uuid.UUID         `json:"competition_match_id"`
	CanonicalResultID  uuid.UUID         `json:"canonical_result_id"`
	ResultStatus       string            `json:"result_status"`
	DisputeStatus      string            `json:"dispute_status"`
	CorrectionID       *uuid.UUID        `json:"correction_id,omitempty"`
	SupersedesResultID *uuid.UUID        `json:"supersedes_result_id,omitempty"`
	RecordedByUserID   uuid.UUID         `json:"recorded_by_user_id"`
	RecordedAt         time.Time         `json:"recorded_at"`
	FinalizedAt        *time.Time        `json:"finalized_at,omitempty"`
	CorrectedAt        *time.Time        `json:"corrected_at,omitempty"`
	Sides              []MatchResultSide `json:"sides"`
}

type MatchResultSide struct {
	SideIndex                int       `json:"side_index"`
	CompetitionSessionTeamID uuid.UUID `json:"competition_session_team_id"`
	Outcome                  string    `json:"outcome"`
}

type MatchResultSideInput struct {
	SideIndex                int       `json:"side_index"`
	CompetitionSessionTeamID uuid.UUID `json:"competition_session_team_id"`
	Outcome                  string    `json:"outcome"`
}

type Standing struct {
	CompetitionSessionID     uuid.UUID `json:"competition_session_id"`
	CompetitionSessionTeamID uuid.UUID `json:"competition_session_team_id"`
	SideIndex                int       `json:"side_index"`
	Rank                     int       `json:"rank"`
	MatchesPlayed            int       `json:"matches_played"`
	Wins                     int       `json:"wins"`
	Losses                   int       `json:"losses"`
	Draws                    int       `json:"draws"`
}

type MemberStat struct {
	UserID             uuid.UUID  `json:"user_id"`
	SportKey           string     `json:"sport_key"`
	ModeKey            string     `json:"mode_key"`
	MatchesPlayed      int        `json:"matches_played"`
	Wins               int        `json:"wins"`
	Losses             int        `json:"losses"`
	Draws              int        `json:"draws"`
	LastPlayedAt       *time.Time `json:"last_played_at,omitempty"`
	CurrentRatingMu    float64    `json:"current_rating_mu"`
	CurrentRatingSigma float64    `json:"current_rating_sigma"`
}

type MemberHistoryEntry struct {
	CompetitionMatchID uuid.UUID `json:"competition_match_id"`
	SourceResultID     uuid.UUID `json:"source_result_id"`
	CanonicalResultID  uuid.UUID `json:"canonical_result_id"`
	DisplayName        string    `json:"display_name"`
	SportKey           string    `json:"sport_key"`
	ModeKey            string    `json:"mode_key"`
	FacilityKey        string    `json:"facility_key"`
	RecordedAt         time.Time `json:"recorded_at"`
	Outcome            string    `json:"outcome"`
}

type PublicCompetitionReadiness struct {
	ContractVersion           string   `json:"contract_version"`
	ProjectionVersion         string   `json:"projection_version"`
	Status                    string   `json:"status"`
	ResultSource              string   `json:"result_source"`
	RatingSource              string   `json:"rating_source"`
	AvailableLeaderboards     int      `json:"available_leaderboards"`
	AvailableCanonicalResults int      `json:"available_canonical_results"`
	Deferred                  []string `json:"deferred"`
}

type PublicCompetitionLeaderboardInput struct {
	SportKey  string
	ModeKey   string
	StatType  string
	TeamScope string
	Limit     int
}

type PublicCompetitionLeaderboard struct {
	ContractVersion   string                            `json:"contract_version"`
	ProjectionVersion string                            `json:"projection_version"`
	ResultSource      string                            `json:"result_source"`
	RatingSource      string                            `json:"rating_source"`
	Leaderboard       []PublicCompetitionLeaderboardRow `json:"leaderboard"`
}

type PublicCompetitionLeaderboardRow struct {
	Rank         int        `json:"rank"`
	Participant  string     `json:"participant"`
	SportKey     string     `json:"sport_key"`
	ModeKey      string     `json:"mode_key"`
	FacilityKey  string     `json:"facility_key"`
	TeamScope    string     `json:"team_scope"`
	StatType     string     `json:"stat_type"`
	StatValue    float64    `json:"stat_value"`
	LastResultAt *time.Time `json:"last_result_at,omitempty"`
	ComputedAt   time.Time  `json:"computed_at"`
}

type PublicGameIdentityInput struct {
	SportKey    string
	ModeKey     string
	FacilityKey string
	TeamScope   string
	Limit       int
}

type GameIdentityProjectionInput struct {
	UserID      *uuid.UUID
	SportKey    string
	ModeKey     string
	FacilityKey string
	TeamScope   string
	Limit       int
}

type GameIdentityProjection struct {
	ContractVersion      string                      `json:"contract_version"`
	ProjectionVersion    string                      `json:"projection_version"`
	Status               string                      `json:"status"`
	ResultSource         string                      `json:"result_source"`
	RatingSource         string                      `json:"rating_source"`
	CPPolicyVersion      string                      `json:"cp_policy_version"`
	BadgePolicyVersion   string                      `json:"badge_policy_version"`
	RivalryPolicyVersion string                      `json:"rivalry_policy_version"`
	SquadPolicyVersion   string                      `json:"squad_policy_version"`
	CP                   []GameIdentityCPProjection  `json:"cp"`
	BadgeAwards          []GameIdentityBadgeAward    `json:"badge_awards"`
	RivalryStates        []GameIdentityRivalryState  `json:"rivalry_states"`
	SquadIdentities      []GameIdentitySquadIdentity `json:"squad_identities"`
}

type GameIdentityCPProjection struct {
	Rank        int                       `json:"rank"`
	Participant string                    `json:"participant"`
	SportKey    string                    `json:"sport_key"`
	ModeKey     string                    `json:"mode_key"`
	FacilityKey string                    `json:"facility_key"`
	TeamScope   string                    `json:"team_scope"`
	CP          int                       `json:"cp"`
	Components  []GameIdentityCPComponent `json:"components"`
	ComputedAt  time.Time                 `json:"computed_at"`
}

type GameIdentityCPComponent struct {
	Metric        string  `json:"metric"`
	UnitValue     float64 `json:"unit_value"`
	PointsPerUnit int     `json:"points_per_unit"`
	Points        int     `json:"points"`
}

type GameIdentityBadgeAward struct {
	Participant   string                      `json:"participant"`
	BadgeKey      string                      `json:"badge_key"`
	BadgeName     string                      `json:"badge_name"`
	PolicyVersion string                      `json:"policy_version"`
	SportKey      string                      `json:"sport_key"`
	ModeKey       string                      `json:"mode_key"`
	FacilityKey   string                      `json:"facility_key"`
	TeamScope     string                      `json:"team_scope"`
	Evidence      []GameIdentityBadgeEvidence `json:"evidence"`
	AwardedAt     *time.Time                  `json:"awarded_at,omitempty"`
	ComputedAt    time.Time                   `json:"computed_at"`
}

type GameIdentityBadgeEvidence struct {
	Metric    string  `json:"metric"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
}

type GameIdentityRivalryState struct {
	RivalryKey    string    `json:"rivalry_key"`
	State         string    `json:"state"`
	PolicyVersion string    `json:"policy_version"`
	SportKey      string    `json:"sport_key"`
	ModeKey       string    `json:"mode_key"`
	FacilityKey   string    `json:"facility_key"`
	TeamScope     string    `json:"team_scope"`
	Participants  []string  `json:"participants"`
	Leader        string    `json:"leader"`
	CPGap         int       `json:"cp_gap"`
	ComputedAt    time.Time `json:"computed_at"`
}

type GameIdentitySquadIdentity struct {
	SquadKey         string    `json:"squad_key"`
	SquadName        string    `json:"squad_name"`
	PolicyVersion    string    `json:"policy_version"`
	SportKey         string    `json:"sport_key"`
	ModeKey          string    `json:"mode_key"`
	FacilityKey      string    `json:"facility_key"`
	TeamScope        string    `json:"team_scope"`
	ParticipantCount int       `json:"participant_count"`
	CPTotal          int       `json:"cp_total"`
	ComputedAt       time.Time `json:"computed_at"`
}

type CreateSessionInput struct {
	DisplayName         string  `json:"display_name"`
	SportKey            string  `json:"sport_key"`
	FacilityKey         string  `json:"facility_key"`
	ZoneKey             *string `json:"zone_key"`
	ParticipantsPerSide int     `json:"participants_per_side"`
}

type CreateTeamInput struct {
	SideIndex int `json:"side_index"`
}

type AddRosterMemberInput struct {
	UserID    uuid.UUID `json:"user_id"`
	SlotIndex int       `json:"slot_index"`
}

type MatchSideInput struct {
	TeamID    uuid.UUID `json:"team_id"`
	SideIndex int       `json:"side_index"`
}

type QueueMemberInput struct {
	UserID uuid.UUID `json:"user_id"`
	Tier   string    `json:"tier,omitempty"`
}

type UpdateQueueIntentInput struct {
	UserID               uuid.UUID `json:"user_id"`
	Tier                 string    `json:"tier"`
	ExpectedQueueVersion int       `json:"expected_queue_version"`
}

type MatchPreviewInput struct {
	ExpectedQueueVersion int `json:"expected_queue_version"`
}

type AssignSessionInput struct {
	ExpectedQueueVersion int `json:"expected_queue_version"`
}

type CreateMatchInput struct {
	MatchIndex int              `json:"match_index"`
	SideSlots  []MatchSideInput `json:"side_slots"`
}

type RecordMatchResultInput struct {
	ExpectedResultVersion int                    `json:"expected_result_version"`
	Sides                 []MatchResultSideInput `json:"sides"`
}

type sessionRecord struct {
	ID                  uuid.UUID
	OwnerUserID         uuid.UUID
	DisplayName         string
	SportKey            string
	FacilityKey         string
	ZoneKey             *string
	ParticipantsPerSide int
	QueueVersion        int
	Status              string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	ArchivedAt          *time.Time
}

type queueRecord struct {
	UserID                uuid.UUID
	DisplayName           string
	Preferences           []byte
	LobbyMembershipStatus string
	JoinedAt              time.Time
}

type queueIntentRecord struct {
	ID          uuid.UUID
	SessionID   uuid.UUID
	UserID      uuid.UUID
	FacilityKey string
	SportKey    string
	ModeKey     string
	Tier        string
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type matchPreviewCandidateRecord struct {
	QueueIntentID        uuid.UUID
	SessionID            uuid.UUID
	UserID               uuid.UUID
	DisplayName          string
	Preferences          []byte
	JoinedAt             time.Time
	FacilityKey          string
	SportKey             string
	ModeKey              string
	Tier                 string
	QueueIntentUpdatedAt time.Time
	RatingMu             float64
	RatingSigma          float64
	RatingMatchesPlayed  int
	RatingUpdatedAt      *time.Time
	RatingFound          bool
}

type teamRecord struct {
	ID        uuid.UUID
	SessionID uuid.UUID
	SideIndex int
	CreatedAt time.Time
}

type rosterRecord struct {
	TeamID      uuid.UUID
	UserID      uuid.UUID
	DisplayName string
	SlotIndex   int
	CreatedAt   time.Time
}

type matchRecord struct {
	ID                uuid.UUID
	SessionID         uuid.UUID
	MatchIndex        int
	Status            string
	ResultVersion     int
	CanonicalResultID *uuid.UUID
	CreatedAt         time.Time
	UpdatedAt         time.Time
	ArchivedAt        *time.Time
}

type matchSideSlotRecord struct {
	MatchID   uuid.UUID
	TeamID    uuid.UUID
	SideIndex int
	CreatedAt time.Time
}

type matchResultRecord struct {
	ID                 uuid.UUID
	CompetitionMatchID uuid.UUID
	RecordedByUserID   uuid.UUID
	RecordedAt         time.Time
	ResultStatus       string
	DisputeStatus      string
	CorrectionID       *uuid.UUID
	SupersedesResultID *uuid.UUID
	FinalizedAt        *time.Time
	CorrectedAt        *time.Time
}

type matchResultSideRecord struct {
	CompetitionMatchResultID uuid.UUID
	CompetitionMatchID       uuid.UUID
	RecordedByUserID         uuid.UUID
	RecordedAt               time.Time
	ResultStatus             string
	DisputeStatus            string
	CorrectionID             *uuid.UUID
	SupersedesResultID       *uuid.UUID
	FinalizedAt              *time.Time
	CorrectedAt              *time.Time
	SideIndex                int
	CompetitionSessionTeamID uuid.UUID
	Outcome                  string
}

type memberRatingRecord struct {
	UserID        uuid.UUID
	SportKey      string
	ModeKey       string
	Mu            float64
	Sigma         float64
	MatchesPlayed int
	LastPlayedAt  *time.Time
	UpdatedAt     time.Time
}

type memberStatRowRecord struct {
	SportKey            string
	CompetitionMode     string
	SidesPerMatch       int
	ParticipantsPerSide int
	RecordedAt          time.Time
	Outcome             string
}

type memberHistoryRowRecord struct {
	CompetitionMatchID  uuid.UUID
	CompetitionResultID uuid.UUID
	DisplayName         string
	SportKey            string
	FacilityKey         string
	CompetitionMode     string
	SidesPerMatch       int
	ParticipantsPerSide int
	RecordedAt          time.Time
	Outcome             string
}

type publicCompetitionReadinessRecord struct {
	AvailableLeaderboards     int
	AvailableCanonicalResults int
}

type publicCompetitionLeaderboardRowRecord struct {
	SportKey     string
	ModeKey      string
	FacilityKey  string
	TeamScope    string
	StatType     string
	StatValue    float64
	SampleSize   int
	Confidence   float64
	LastResultAt *time.Time
	ComputedAt   time.Time
}

type gameIdentityProjectionRowRecord struct {
	UserID        uuid.UUID
	SportKey      string
	ModeKey       string
	FacilityKey   string
	TeamScope     string
	MatchesPlayed float64
	Wins          float64
	Losses        float64
	Draws         float64
	LastResultAt  *time.Time
	ComputedAt    time.Time
}

func NewService(repository Store) *Service {
	return &Service{
		repository: repository,
		now:        time.Now,
	}
}

func (s *Service) ListSessions(ctx context.Context) ([]SessionSummary, error) {
	rows, err := s.repository.ListSessions(ctx)
	if err != nil {
		return nil, err
	}

	summaries := make([]SessionSummary, 0, len(rows))
	for _, row := range rows {
		summaries = append(summaries, buildSessionSummary(row))
	}

	return summaries, nil
}

func (s *Service) GetSession(ctx context.Context, sessionID uuid.UUID) (Session, error) {
	session, err := s.repository.GetSessionByID(ctx, sessionID)
	if err != nil {
		return Session{}, err
	}
	if session == nil {
		return Session{}, ErrSessionNotFound
	}

	return s.loadSessionDetail(ctx, *session)
}

func (s *Service) CreateSession(ctx context.Context, actor StaffActor, input CreateSessionInput) (Session, error) {
	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		return Session{}, ErrSessionNameRequired
	}

	sport, err := s.repository.GetSportConfig(ctx, strings.TrimSpace(input.SportKey))
	if err != nil {
		return Session{}, err
	}
	if sport == nil {
		return Session{}, ErrSportNotFound
	}
	if input.ParticipantsPerSide < sport.ParticipantsPerSideMin || input.ParticipantsPerSide > sport.ParticipantsPerSideMax {
		return Session{}, ErrParticipantsPerSide
	}

	normalizedZone := normalizeOptionalText(input.ZoneKey)
	if err := s.validateFacilityBinding(ctx, sport.SportKey, strings.TrimSpace(input.FacilityKey), normalizedZone); err != nil {
		return Session{}, err
	}

	created, err := s.repository.CreateSession(ctx, actor, CreateSessionInput{
		DisplayName:         displayName,
		SportKey:            sport.SportKey,
		FacilityKey:         strings.TrimSpace(input.FacilityKey),
		ZoneKey:             normalizedZone,
		ParticipantsPerSide: input.ParticipantsPerSide,
	}, s.now().UTC())
	if err != nil {
		if isUniqueViolation(err) {
			return Session{}, ErrDuplicateSession
		}
		return Session{}, err
	}

	return s.loadSessionDetail(ctx, created)
}

func (s *Service) OpenQueue(ctx context.Context, actor StaffActor, sessionID uuid.UUID) (Session, error) {
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
	if session.Status != SessionStatusDraft {
		return Session{}, ErrInvalidSessionTransition
	}

	queueCount, err := s.repository.CountQueueMembersBySessionID(ctx, session.ID)
	if err != nil {
		return Session{}, err
	}
	if queueCount > 0 {
		return Session{}, ErrExecutionAlreadySeeded
	}

	if err := s.ensureExecutionContainersEmpty(ctx, session.ID); err != nil {
		return Session{}, err
	}

	opened, err := s.repository.OpenQueue(ctx, actor, *session, s.now().UTC())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrInvalidSessionTransition
		}
		return Session{}, err
	}

	return s.loadSessionDetail(ctx, opened)
}

func (s *Service) AddQueueMember(ctx context.Context, actor StaffActor, sessionID uuid.UUID, input QueueMemberInput) (Session, error) {
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
	if session.Status != SessionStatusQueueOpen {
		return Session{}, ErrQueueClosed
	}

	sport, err := s.repository.GetSportConfig(ctx, session.SportKey)
	if err != nil {
		return Session{}, err
	}
	if sport == nil {
		return Session{}, ErrSportNotFound
	}

	queueMembers, err := s.repository.ListQueueMembersBySessionID(ctx, session.ID)
	if err != nil {
		return Session{}, err
	}
	for _, queueMember := range queueMembers {
		if queueMember.UserID == input.UserID {
			return Session{}, ErrQueueMemberAlreadyJoined
		}
	}
	if len(queueMembers) >= requiredQueueCapacity(*sport, *session) {
		return Session{}, ErrQueueCapacityReached
	}

	if err := s.validateQueueCandidate(ctx, input.UserID); err != nil {
		return Session{}, err
	}

	tier, err := normalizeQueueTier(input.Tier)
	if err != nil {
		return Session{}, err
	}
	input.Tier = tier

	if err := s.repository.AddQueueMember(ctx, actor, *session, *sport, input, s.now().UTC()); err != nil {
		switch {
		case isUniqueConstraint(err, competitionSessionQueueMembersPrimaryKey), isUniqueViolation(err):
			return Session{}, ErrQueueMemberAlreadyJoined
		case errors.Is(err, pgx.ErrNoRows):
			return Session{}, ErrQueueClosed
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

func (s *Service) RemoveQueueMember(ctx context.Context, actor StaffActor, sessionID uuid.UUID, userID uuid.UUID) (Session, error) {
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
	if session.Status != SessionStatusQueueOpen {
		return Session{}, ErrQueueClosed
	}

	if err := s.repository.RemoveQueueMember(ctx, actor, *session, userID, s.now().UTC()); err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return Session{}, ErrQueueMemberNotFound
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

func (s *Service) UpdateQueueIntent(ctx context.Context, actor StaffActor, sessionID uuid.UUID, input UpdateQueueIntentInput) (Session, error) {
	if input.ExpectedQueueVersion <= 0 {
		return Session{}, ErrQueueVersionRequired
	}
	if input.UserID == uuid.Nil {
		return Session{}, ErrUserNotFound
	}
	tier, err := normalizeQueueTier(input.Tier)
	if err != nil {
		return Session{}, err
	}
	input.Tier = tier

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
	if session.Status != SessionStatusQueueOpen {
		return Session{}, ErrQueueClosed
	}
	if session.QueueVersion != input.ExpectedQueueVersion {
		return Session{}, ErrQueueStateStale
	}

	queueMembers, err := s.repository.ListQueueMembersBySessionID(ctx, session.ID)
	if err != nil {
		return Session{}, err
	}
	found := false
	for _, member := range queueMembers {
		if member.UserID == input.UserID {
			found = true
			break
		}
	}
	if !found {
		return Session{}, ErrQueueMemberNotFound
	}

	updated, _, err := s.repository.UpdateQueueIntent(ctx, actor, *session, input, s.now().UTC())
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return Session{}, ErrQueueIntentNotFound
		default:
			return Session{}, err
		}
	}
	return s.loadSessionDetail(ctx, updated)
}

func (s *Service) GenerateMatchPreview(ctx context.Context, actor StaffActor, sessionID uuid.UUID, input MatchPreviewInput) (ares.CompetitionMatchPreview, error) {
	if input.ExpectedQueueVersion <= 0 {
		return ares.CompetitionMatchPreview{}, ErrQueueVersionRequired
	}

	session, err := s.repository.GetSessionByID(ctx, sessionID)
	if err != nil {
		return ares.CompetitionMatchPreview{}, err
	}
	if session == nil {
		return ares.CompetitionMatchPreview{}, ErrSessionNotFound
	}
	if session.Status == SessionStatusArchived {
		return ares.CompetitionMatchPreview{}, ErrSessionArchived
	}
	if session.Status != SessionStatusQueueOpen {
		return ares.CompetitionMatchPreview{}, ErrInvalidSessionTransition
	}
	if session.QueueVersion != input.ExpectedQueueVersion {
		return ares.CompetitionMatchPreview{}, ErrQueueStateStale
	}

	sport, err := s.repository.GetSportConfig(ctx, session.SportKey)
	if err != nil {
		return ares.CompetitionMatchPreview{}, err
	}
	if sport == nil {
		return ares.CompetitionMatchPreview{}, ErrSportNotFound
	}

	candidates, err := s.repository.ListMatchPreviewCandidatesBySessionID(ctx, session.ID)
	if err != nil {
		return ares.CompetitionMatchPreview{}, err
	}
	if len(candidates) != requiredQueueCapacity(*sport, *session) {
		return ares.CompetitionMatchPreview{}, ErrQueueNotReady
	}

	modeKey := buildModeKey(sport.CompetitionMode, sport.SidesPerMatch, session.ParticipantsPerSide)
	previewCandidates := make([]ares.CompetitionPreviewCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if err := validateMatchPreviewCandidate(candidate, *session, modeKey); err != nil {
			return ares.CompetitionMatchPreview{}, err
		}
		previewCandidates = append(previewCandidates, buildAresPreviewCandidate(candidate))
	}

	preview := ares.BuildCompetitionMatchPreview(ares.CompetitionPreviewInput{
		SessionID:           session.ID,
		QueueVersion:        session.QueueVersion,
		FacilityKey:         session.FacilityKey,
		SportKey:            session.SportKey,
		ModeKey:             modeKey,
		SidesPerMatch:       sport.SidesPerMatch,
		ParticipantsPerSide: session.ParticipantsPerSide,
		Candidates:          previewCandidates,
	})

	if err := s.repository.RecordMatchPreview(ctx, actor, *session, preview, s.now().UTC()); err != nil {
		return ares.CompetitionMatchPreview{}, err
	}
	return preview, nil
}

func (s *Service) AssignQueue(ctx context.Context, actor StaffActor, sessionID uuid.UUID, input AssignSessionInput) (Session, error) {
	if input.ExpectedQueueVersion <= 0 {
		return Session{}, ErrQueueVersionRequired
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
	if session.Status != SessionStatusQueueOpen {
		return Session{}, ErrInvalidSessionTransition
	}
	if session.QueueVersion != input.ExpectedQueueVersion {
		return Session{}, ErrQueueStateStale
	}

	if err := s.ensureExecutionContainersEmpty(ctx, session.ID); err != nil {
		return Session{}, err
	}

	sport, err := s.repository.GetSportConfig(ctx, session.SportKey)
	if err != nil {
		return Session{}, err
	}
	if sport == nil {
		return Session{}, ErrSportNotFound
	}

	queueMembers, err := s.repository.ListQueueMembersBySessionID(ctx, session.ID)
	if err != nil {
		return Session{}, err
	}
	if len(queueMembers) != requiredQueueCapacity(*sport, *session) {
		return Session{}, ErrQueueNotReady
	}

	for _, queueMember := range queueMembers {
		if err := validateQueueRecord(queueMember); err != nil {
			return Session{}, err
		}
	}

	assigned, err := s.repository.AssignQueue(ctx, actor, *session, input, *sport, queueMembers, s.now().UTC())
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return Session{}, ErrQueueStateStale
		default:
			return Session{}, err
		}
	}

	return s.loadSessionDetail(ctx, assigned)
}

func (s *Service) StartSession(ctx context.Context, actor StaffActor, sessionID uuid.UUID) (Session, error) {
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
	if session.Status != SessionStatusAssigned {
		return Session{}, ErrInvalidSessionTransition
	}

	started, err := s.repository.StartSession(ctx, actor, *session, s.now().UTC())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrInvalidSessionTransition
		}
		return Session{}, err
	}

	return s.loadSessionDetail(ctx, started)
}

func (s *Service) ArchiveSession(ctx context.Context, actor StaffActor, sessionID uuid.UUID) (Session, error) {
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

	switch session.Status {
	case SessionStatusDraft:
		draftMatches, countErr := s.repository.CountDraftMatchesBySessionID(ctx, sessionID)
		if countErr != nil {
			return Session{}, countErr
		}
		if draftMatches > 0 {
			return Session{}, ErrSessionHasDraftMatches
		}
	case SessionStatusQueueOpen:
		queueCount, countErr := s.repository.CountQueueMembersBySessionID(ctx, session.ID)
		if countErr != nil {
			return Session{}, countErr
		}
		if queueCount > 0 {
			return Session{}, ErrQueueNotEmpty
		}
	case SessionStatusAssigned:
	case SessionStatusInProgress:
	case SessionStatusCompleted:
		// Completed sessions may be archived without mutating completed matches.
	default:
		return Session{}, ErrInvalidSessionTransition
	}

	archived, err := s.repository.ArchiveSession(ctx, actor, *session, s.now().UTC())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrInvalidSessionTransition
		}
		return Session{}, err
	}

	return s.loadSessionDetail(ctx, archived)
}

func (s *Service) CreateTeam(ctx context.Context, actor StaffActor, sessionID uuid.UUID, input CreateTeamInput) (Team, error) {
	if input.SideIndex <= 0 {
		return Team{}, ErrTeamSideIndexInvalid
	}

	session, err := s.requireDraftSession(ctx, sessionID)
	if err != nil {
		return Team{}, err
	}

	team, err := s.repository.CreateTeam(ctx, actor, session.ID, input.SideIndex, s.now().UTC())
	if err != nil {
		if isUniqueViolation(err) {
			return Team{}, ErrDuplicateTeam
		}
		return Team{}, err
	}

	detail, err := s.loadSessionDetail(ctx, session)
	if err != nil {
		return Team{}, err
	}

	return findTeam(detail.Teams, team.ID)
}

func (s *Service) RemoveTeam(ctx context.Context, actor StaffActor, sessionID uuid.UUID, teamID uuid.UUID) error {
	session, err := s.requireDraftSession(ctx, sessionID)
	if err != nil {
		return err
	}

	team, err := s.repository.GetTeamByID(ctx, teamID)
	if err != nil {
		return err
	}
	if team == nil || team.SessionID != session.ID {
		return ErrTeamNotFound
	}

	referenced, err := s.repository.TeamHasMatchReference(ctx, teamID)
	if err != nil {
		return err
	}
	if referenced {
		return ErrTeamReferencedByMatch
	}

	deleted, err := s.repository.DeleteTeam(ctx, actor, session.ID, teamID, s.now().UTC())
	if err != nil {
		return err
	}
	if deleted == 0 {
		return ErrTeamNotFound
	}

	return nil
}

func (s *Service) AddRosterMember(ctx context.Context, actor StaffActor, sessionID uuid.UUID, teamID uuid.UUID, input AddRosterMemberInput) (Team, error) {
	if input.SlotIndex <= 0 {
		return Team{}, ErrRosterSlotIndexInvalid
	}

	session, err := s.requireDraftSession(ctx, sessionID)
	if err != nil {
		return Team{}, err
	}
	if input.SlotIndex > session.ParticipantsPerSide {
		return Team{}, ErrRosterSlotOutOfRange
	}

	team, err := s.repository.GetTeamByID(ctx, teamID)
	if err != nil {
		return Team{}, err
	}
	if team == nil || team.SessionID != session.ID {
		return Team{}, ErrTeamNotFound
	}

	referenced, err := s.repository.TeamHasMatchReference(ctx, teamID)
	if err != nil {
		return Team{}, err
	}
	if referenced {
		return Team{}, ErrTeamReferencedByMatch
	}

	user, err := s.repository.GetUserByID(ctx, input.UserID)
	if err != nil {
		return Team{}, err
	}
	if user == nil {
		return Team{}, ErrUserNotFound
	}

	conflict, err := s.repository.SessionHasRosterMemberUser(ctx, session.ID, input.UserID)
	if err != nil {
		return Team{}, err
	}
	if conflict {
		return Team{}, ErrRosterConflict
	}

	memberCount, err := s.repository.CountRosterMembersByTeamID(ctx, teamID)
	if err != nil {
		return Team{}, err
	}
	if memberCount >= int64(session.ParticipantsPerSide) {
		return Team{}, ErrTeamSizeMismatch
	}

	if _, err := s.repository.CreateRosterMember(ctx, actor, session.ID, teamID, input.UserID, input.SlotIndex, s.now().UTC()); err != nil {
		switch {
		case isUniqueConstraint(err, competitionTeamRosterMembersPrimaryKey, competitionTeamRosterMembersSessionUserUnique):
			return Team{}, ErrRosterConflict
		case isUniqueConstraint(err, competitionTeamRosterMembersTeamSlotUnique), isUniqueViolation(err):
			return Team{}, ErrDuplicateRosterSlot
		default:
			return Team{}, err
		}
	}

	detail, err := s.loadSessionDetail(ctx, session)
	if err != nil {
		return Team{}, err
	}

	return findTeam(detail.Teams, teamID)
}

func (s *Service) RemoveRosterMember(ctx context.Context, actor StaffActor, sessionID uuid.UUID, teamID uuid.UUID, userID uuid.UUID) error {
	session, err := s.requireDraftSession(ctx, sessionID)
	if err != nil {
		return err
	}

	team, err := s.repository.GetTeamByID(ctx, teamID)
	if err != nil {
		return err
	}
	if team == nil || team.SessionID != session.ID {
		return ErrTeamNotFound
	}

	referenced, err := s.repository.TeamHasMatchReference(ctx, teamID)
	if err != nil {
		return err
	}
	if referenced {
		return ErrTeamReferencedByMatch
	}

	deleted, err := s.repository.DeleteRosterMember(ctx, actor, session.ID, teamID, userID, s.now().UTC())
	if err != nil {
		return err
	}
	if deleted == 0 {
		return ErrRosterMemberNotFound
	}

	return nil
}

func (s *Service) CreateMatch(ctx context.Context, actor StaffActor, sessionID uuid.UUID, input CreateMatchInput) (Match, error) {
	if input.MatchIndex <= 0 {
		return Match{}, ErrMatchIndexInvalid
	}

	session, err := s.requireDraftSession(ctx, sessionID)
	if err != nil {
		return Match{}, err
	}

	sport, err := s.repository.GetSportConfig(ctx, session.SportKey)
	if err != nil {
		return Match{}, err
	}
	if sport == nil {
		return Match{}, ErrSportNotFound
	}

	if len(input.SideSlots) != sport.SidesPerMatch {
		return Match{}, ErrMatchSideCountMismatch
	}
	if err := validateMatchSideInputs(input.SideSlots); err != nil {
		return Match{}, err
	}

	teams, err := s.repository.ListTeamsBySessionID(ctx, session.ID)
	if err != nil {
		return Match{}, err
	}
	teamSet := make(map[uuid.UUID]teamRecord, len(teams))
	for _, team := range teams {
		teamSet[team.ID] = team
	}

	for _, slot := range input.SideSlots {
		if _, exists := teamSet[slot.TeamID]; !exists {
			return Match{}, ErrTeamNotFound
		}
		memberCount, countErr := s.repository.CountRosterMembersByTeamID(ctx, slot.TeamID)
		if countErr != nil {
			return Match{}, countErr
		}
		if memberCount != int64(session.ParticipantsPerSide) {
			return Match{}, ErrTeamSizeMismatch
		}
	}

	match, err := s.repository.CreateMatchWithSideSlots(ctx, actor, session.ID, input.MatchIndex, input.SideSlots, s.now().UTC())
	if err != nil {
		if isUniqueViolation(err) {
			return Match{}, ErrDuplicateMatch
		}
		return Match{}, err
	}

	detail, err := s.loadSessionDetail(ctx, session)
	if err != nil {
		return Match{}, err
	}

	return findMatch(detail.Matches, match.ID)
}

func (s *Service) ArchiveMatch(ctx context.Context, actor StaffActor, sessionID uuid.UUID, matchID uuid.UUID) (Match, error) {
	session, err := s.requireDraftSession(ctx, sessionID)
	if err != nil {
		return Match{}, err
	}

	match, err := s.repository.GetMatchByID(ctx, matchID)
	if err != nil {
		return Match{}, err
	}
	if match == nil || match.SessionID != session.ID {
		return Match{}, ErrMatchNotFound
	}
	if match.Status == MatchStatusArchived {
		return Match{}, ErrMatchArchived
	}

	if _, err := s.repository.ArchiveMatch(ctx, actor, session.ID, matchID, s.now().UTC()); err != nil {
		return Match{}, err
	}

	detail, err := s.loadSessionDetail(ctx, session)
	if err != nil {
		return Match{}, err
	}

	return findMatch(detail.Matches, matchID)
}

func (s *Service) requireDraftSession(ctx context.Context, sessionID uuid.UUID) (sessionRecord, error) {
	session, err := s.repository.GetSessionByID(ctx, sessionID)
	if err != nil {
		return sessionRecord{}, err
	}
	if session == nil {
		return sessionRecord{}, ErrSessionNotFound
	}
	if session.Status == SessionStatusArchived {
		return sessionRecord{}, ErrSessionArchived
	}
	if session.Status != SessionStatusDraft {
		return sessionRecord{}, ErrInvalidSessionTransition
	}

	return *session, nil
}

func (s *Service) loadSessionDetail(ctx context.Context, session sessionRecord) (Session, error) {
	queueRows, err := s.repository.ListQueueMembersBySessionID(ctx, session.ID)
	if err != nil {
		return Session{}, err
	}
	teams, err := s.repository.ListTeamsBySessionID(ctx, session.ID)
	if err != nil {
		return Session{}, err
	}
	rosterRows, err := s.repository.ListRosterMembersBySessionID(ctx, session.ID)
	if err != nil {
		return Session{}, err
	}
	matches, err := s.repository.ListMatchesBySessionID(ctx, session.ID)
	if err != nil {
		return Session{}, err
	}
	sideSlots, err := s.repository.ListMatchSideSlotsBySessionID(ctx, session.ID)
	if err != nil {
		return Session{}, err
	}
	resultRows, err := s.repository.ListMatchResultsBySessionID(ctx, session.ID)
	if err != nil {
		return Session{}, err
	}

	teamValues := make([]Team, 0, len(teams))
	teamIndex := make(map[uuid.UUID]int, len(teams))
	for _, team := range teams {
		teamIndex[team.ID] = len(teamValues)
		teamValues = append(teamValues, Team{
			ID:        team.ID,
			SideIndex: team.SideIndex,
			CreatedAt: team.CreatedAt,
			Roster:    nil,
		})
	}
	for _, row := range rosterRows {
		index, exists := teamIndex[row.TeamID]
		if !exists {
			continue
		}
		teamValues[index].Roster = append(teamValues[index].Roster, RosterMember{
			UserID:      row.UserID,
			DisplayName: row.DisplayName,
			SlotIndex:   row.SlotIndex,
			CreatedAt:   row.CreatedAt,
		})
	}

	matchValues := make([]Match, 0, len(matches))
	matchIndex := make(map[uuid.UUID]int, len(matches))
	for _, match := range matches {
		matchIndex[match.ID] = len(matchValues)
		matchValues = append(matchValues, Match{
			ID:                match.ID,
			MatchIndex:        match.MatchIndex,
			Status:            match.Status,
			MatchStatus:       match.Status,
			ResultVersion:     match.ResultVersion,
			CanonicalResultID: match.CanonicalResultID,
			CreatedAt:         match.CreatedAt,
			UpdatedAt:         match.UpdatedAt,
			ArchivedAt:        match.ArchivedAt,
			SideSlots:         nil,
			Result:            nil,
		})
	}
	for _, slot := range sideSlots {
		index, exists := matchIndex[slot.MatchID]
		if !exists {
			continue
		}
		matchValues[index].SideSlots = append(matchValues[index].SideSlots, MatchSideRef{
			TeamID:    slot.TeamID,
			SideIndex: slot.SideIndex,
			CreatedAt: slot.CreatedAt,
		})
	}
	matchResults := buildMatchResults(resultRows)
	for matchID, result := range matchResults {
		index, exists := matchIndex[matchID]
		if !exists {
			continue
		}
		matchValues[index].Result = result
	}

	queueValues := make([]QueueMember, 0, len(queueRows))
	for _, queueRow := range queueRows {
		queueValues = append(queueValues, QueueMember{
			UserID:      queueRow.UserID,
			DisplayName: queueRow.DisplayName,
			JoinedAt:    queueRow.JoinedAt,
		})
	}

	return Session{
		SessionSummary: buildSessionSummary(session),
		Queue:          queueValues,
		Teams:          teamValues,
		Matches:        matchValues,
		Standings:      buildStandings(session.ID, teamValues, matchResults),
	}, nil
}

func (s *Service) ensureExecutionContainersEmpty(ctx context.Context, sessionID uuid.UUID) error {
	teams, err := s.repository.ListTeamsBySessionID(ctx, sessionID)
	if err != nil {
		return err
	}
	if len(teams) > 0 {
		return ErrExecutionAlreadySeeded
	}

	matches, err := s.repository.ListMatchesBySessionID(ctx, sessionID)
	if err != nil {
		return err
	}
	if len(matches) > 0 {
		return ErrExecutionAlreadySeeded
	}

	return nil
}

func (s *Service) validateQueueCandidate(ctx context.Context, userID uuid.UUID) error {
	user, err := s.repository.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}

	membershipRecord, err := s.repository.GetLobbyMembershipByUserID(ctx, userID)
	if err != nil {
		return err
	}
	if membershipRecord == nil || membershipRecord.Status != membership.StatusJoined {
		return ErrQueueMemberNotJoined
	}

	lobbyEligibility := eligibility.FromPreferenceModes(profile.ReadPreferenceModes(user.Preferences))
	if !lobbyEligibility.Eligible {
		return ErrQueueMemberIneligible
	}

	return nil
}

func validateQueueRecord(queueMember queueRecord) error {
	if queueMember.LobbyMembershipStatus != membership.StatusJoined {
		return ErrQueueMemberNotJoined
	}

	lobbyEligibility := eligibility.FromPreferenceModes(profile.ReadPreferenceModes(queueMember.Preferences))
	if !lobbyEligibility.Eligible {
		return ErrQueueMemberIneligible
	}

	return nil
}

func validateMatchPreviewCandidate(candidate matchPreviewCandidateRecord, session sessionRecord, modeKey string) error {
	if candidate.FacilityKey != session.FacilityKey || candidate.SportKey != session.SportKey || candidate.ModeKey != modeKey {
		return ErrQueueIntentNotFound
	}

	lobbyEligibility := eligibility.FromPreferenceModes(profile.ReadPreferenceModes(candidate.Preferences))
	if !lobbyEligibility.Eligible {
		return ErrQueueMemberIneligible
	}

	if _, err := normalizeQueueTier(candidate.Tier); err != nil {
		return err
	}
	return nil
}

func buildAresPreviewCandidate(candidate matchPreviewCandidateRecord) ares.CompetitionPreviewCandidate {
	ratingMu := candidate.RatingMu
	ratingSigma := candidate.RatingSigma
	ratingMatchesPlayed := candidate.RatingMatchesPlayed
	ratingSource := ares.RatingSourceLegacyProjection
	watermark := candidate.QueueIntentUpdatedAt
	if candidate.RatingUpdatedAt != nil && candidate.RatingUpdatedAt.After(watermark) {
		watermark = *candidate.RatingUpdatedAt
	}
	if !candidate.RatingFound {
		ratingMu = rating.InitialMu
		ratingSigma = rating.InitialSigma
		ratingMatchesPlayed = 0
		ratingSource = ares.RatingSourceInitialRating
	}
	if candidate.JoinedAt.After(watermark) {
		watermark = candidate.JoinedAt
	}

	return ares.CompetitionPreviewCandidate{
		QueueIntentID:       candidate.QueueIntentID,
		UserID:              candidate.UserID,
		DisplayName:         candidate.DisplayName,
		Tier:                candidate.Tier,
		RatingMu:            ratingMu,
		RatingSigma:         ratingSigma,
		RatingMatchesPlayed: ratingMatchesPlayed,
		RatingSource:        ratingSource,
		JoinedAt:            candidate.JoinedAt,
		InputWatermark:      watermark.UTC(),
	}
}

func normalizeQueueTier(value string) (string, error) {
	tier := strings.TrimSpace(value)
	if tier == "" {
		return "open", nil
	}
	if len(tier) > 64 {
		return "", ErrQueueIntentTier
	}
	for _, char := range tier {
		if char >= 'a' && char <= 'z' {
			continue
		}
		if char >= 'A' && char <= 'Z' {
			continue
		}
		if char >= '0' && char <= '9' {
			continue
		}
		if char == '_' || char == '-' || char == ':' || char == '.' {
			continue
		}
		return "", ErrQueueIntentTier
	}
	return strings.ToLower(tier), nil
}

func requiredQueueCapacity(sport SportConfig, session sessionRecord) int {
	return sport.SidesPerMatch * session.ParticipantsPerSide
}

func (s *Service) validateFacilityBinding(ctx context.Context, sportKey string, facilityKey string, zoneKey *string) error {
	capabilities, err := s.repository.ListFacilityCapabilities(ctx)
	if err != nil {
		return err
	}

	for _, capability := range capabilities {
		if capability.SportKey != sportKey || capability.FacilityKey != facilityKey {
			continue
		}
		if zoneKey == nil {
			return nil
		}
		if len(capability.ZoneKeys) == 0 {
			break
		}
		for _, candidate := range capability.ZoneKeys {
			if candidate == *zoneKey {
				return nil
			}
		}
		return ErrZoneUnsupported
	}

	return ErrFacilityUnsupported
}

func buildSessionSummary(session sessionRecord) SessionSummary {
	return SessionSummary{
		ID:                  session.ID,
		DisplayName:         session.DisplayName,
		SportKey:            session.SportKey,
		FacilityKey:         session.FacilityKey,
		ZoneKey:             session.ZoneKey,
		ParticipantsPerSide: session.ParticipantsPerSide,
		QueueVersion:        session.QueueVersion,
		Status:              session.Status,
		CreatedAt:           session.CreatedAt,
		UpdatedAt:           session.UpdatedAt,
		ArchivedAt:          session.ArchivedAt,
	}
}

func buildModeKey(competitionMode string, sidesPerMatch int, participantsPerSide int) string {
	return fmt.Sprintf("%s:s%d-p%d", competitionMode, sidesPerMatch, participantsPerSide)
}

func validateMatchSideInputs(sideSlots []MatchSideInput) error {
	sideIndexSeen := make(map[int]struct{}, len(sideSlots))
	teamSeen := make(map[uuid.UUID]struct{}, len(sideSlots))
	ordered := make([]int, 0, len(sideSlots))

	for _, sideSlot := range sideSlots {
		if sideSlot.SideIndex <= 0 {
			return ErrMatchSideIndexInvalid
		}
		if _, exists := sideIndexSeen[sideSlot.SideIndex]; exists {
			return ErrDuplicateMatchSideIndex
		}
		if _, exists := teamSeen[sideSlot.TeamID]; exists {
			return ErrDuplicateMatchTeam
		}
		sideIndexSeen[sideSlot.SideIndex] = struct{}{}
		teamSeen[sideSlot.TeamID] = struct{}{}
		ordered = append(ordered, sideSlot.SideIndex)
	}

	slices.Sort(ordered)
	for index, sideIndex := range ordered {
		if sideIndex != index+1 {
			return ErrMatchSideIndexInvalid
		}
	}

	return nil
}

func findTeam(teams []Team, teamID uuid.UUID) (Team, error) {
	for _, team := range teams {
		if team.ID == teamID {
			return team, nil
		}
	}

	return Team{}, ErrTeamNotFound
}

func findMatch(matches []Match, matchID uuid.UUID) (Match, error) {
	for _, match := range matches {
		if match.ID == matchID {
			return match, nil
		}
	}

	return Match{}, ErrMatchNotFound
}

func normalizeOptionalText(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isUniqueConstraint(err error, names ...string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return false
	}

	for _, name := range names {
		if pgErr.ConstraintName == name {
			return true
		}
	}

	return false
}
