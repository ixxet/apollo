package competition

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	TournamentFormatSingleElimination = "single_elimination"
	TournamentVisibilityInternal      = "internal"
	TournamentStatusDraft             = "draft"
	TournamentStatusSeeded            = "seeded"
	TournamentStatusLocked            = "locked"
	TournamentStatusInProgress        = "in_progress"
	TournamentStatusCompleted         = "completed"
	TournamentStatusArchived          = "archived"
	AdvanceReasonCanonicalResultWin   = "canonical_result_win"

	tournamentEventCreated       = "competition.tournament.created"
	tournamentEventSeeded        = "competition.tournament.seeded"
	tournamentEventTeamLocked    = "competition.tournament.team_locked"
	tournamentEventMatchBound    = "competition.tournament.match_bound"
	tournamentEventRoundAdvanced = "competition.tournament.round_advanced"
)

var (
	ErrTournamentStoreUnavailable      = errors.New("competition tournament store is unavailable")
	ErrTournamentNotFound              = errors.New("competition tournament not found")
	ErrTournamentNameRequired          = errors.New("competition tournament display_name is required")
	ErrTournamentFormatUnsupported     = errors.New("competition tournament format is unsupported")
	ErrTournamentVersionRequired       = errors.New("expected_tournament_version is required")
	ErrTournamentStateStale            = errors.New("competition tournament state is stale")
	ErrTournamentStatus                = errors.New("competition tournament status is invalid for this operation")
	ErrDuplicateTournament             = errors.New("competition tournament already exists for this owner and sport")
	ErrTournamentSeedInvalid           = errors.New("competition tournament seed is invalid")
	ErrTournamentSeedCount             = errors.New("competition tournament seed count is invalid")
	ErrTournamentSeedDuplicate         = errors.New("competition tournament seed is duplicated")
	ErrTournamentSeedNotFound          = errors.New("competition tournament seed not found")
	ErrTournamentTeamSnapshotNotFound  = errors.New("competition tournament team snapshot not found")
	ErrTournamentTeamSnapshotLocked    = errors.New("competition tournament team snapshot is already locked")
	ErrTournamentTeamMismatch          = errors.New("competition tournament team does not match tournament truth")
	ErrTournamentBracketNotFound       = errors.New("competition tournament bracket not found")
	ErrTournamentRoundInvalid          = errors.New("competition tournament round is invalid")
	ErrTournamentMatchBindingNotFound  = errors.New("competition tournament match binding not found")
	ErrTournamentMatchBindingDuplicate = errors.New("competition tournament match binding already exists")
	ErrTournamentAdvanceReason         = errors.New("competition tournament advance_reason is invalid")
	ErrTournamentAdvanceDuplicate      = errors.New("competition tournament round advancement already exists")
	ErrTournamentAdvanceResultRequired = errors.New("competition tournament advancement requires canonical result truth")
	ErrTournamentAdvanceResultOutcome  = errors.New("competition tournament advancement result outcome is invalid")
)

type tournamentStore interface {
	ListTournaments(ctx context.Context) ([]tournamentRecord, error)
	GetTournamentByID(ctx context.Context, tournamentID uuid.UUID) (*tournamentRecord, error)
	CreateTournament(ctx context.Context, actor StaffActor, input CreateTournamentInput, createdAt time.Time) (tournamentRecord, error)
	SeedTournament(ctx context.Context, actor StaffActor, tournament tournamentRecord, bracket tournamentBracketRecord, input SeedTournamentInput, seededAt time.Time) (tournamentRecord, error)
	LockTournamentTeam(ctx context.Context, actor StaffActor, tournament tournamentRecord, bracket tournamentBracketRecord, seed tournamentSeedRecord, team teamRecord, roster []rosterRecord, rosterHash string, lockedAt time.Time) (tournamentRecord, error)
	BindTournamentMatch(ctx context.Context, actor StaffActor, tournament tournamentRecord, bracket tournamentBracketRecord, input BindTournamentMatchInput, sideOne tournamentTeamSnapshotRecord, sideTwo tournamentTeamSnapshotRecord, boundAt time.Time) (tournamentRecord, error)
	AdvanceTournamentRound(ctx context.Context, actor StaffActor, tournament tournamentRecord, bracket tournamentBracketRecord, binding tournamentMatchBindingRecord, input AdvanceTournamentRoundInput, winner tournamentTeamSnapshotRecord, loser tournamentTeamSnapshotRecord, canonicalResultID uuid.UUID, finalRound bool, advancedAt time.Time) (tournamentRecord, error)
	ListTournamentBracketsByTournamentID(ctx context.Context, tournamentID uuid.UUID) ([]tournamentBracketRecord, error)
	ListTournamentSeedsByBracketID(ctx context.Context, bracketID uuid.UUID) ([]tournamentSeedRecord, error)
	GetTournamentSeedByBracketSeed(ctx context.Context, bracketID uuid.UUID, seed int) (*tournamentSeedRecord, error)
	ListTournamentTeamSnapshotsByBracketID(ctx context.Context, bracketID uuid.UUID) ([]tournamentTeamSnapshotRecord, error)
	GetTournamentTeamSnapshotByID(ctx context.Context, teamSnapshotID uuid.UUID) (*tournamentTeamSnapshotRecord, error)
	ListTournamentTeamSnapshotMembersByBracketID(ctx context.Context, bracketID uuid.UUID) ([]tournamentSnapshotMemberRecord, error)
	CountTournamentTeamSnapshotsByBracketID(ctx context.Context, bracketID uuid.UUID) (int64, error)
	ListTournamentMatchBindingsByBracketID(ctx context.Context, bracketID uuid.UUID) ([]tournamentMatchBindingRecord, error)
	GetTournamentMatchBindingByID(ctx context.Context, matchBindingID uuid.UUID) (*tournamentMatchBindingRecord, error)
	ListTournamentAdvancementsByBracketID(ctx context.Context, bracketID uuid.UUID) ([]tournamentAdvancementRecord, error)
	ListMatchResultSidesByResultID(ctx context.Context, resultID uuid.UUID) ([]matchResultSideRecord, error)
}

type CreateTournamentInput struct {
	DisplayName         string  `json:"display_name"`
	Format              string  `json:"format"`
	SportKey            string  `json:"sport_key"`
	FacilityKey         string  `json:"facility_key"`
	ZoneKey             *string `json:"zone_key,omitempty"`
	ParticipantsPerSide int     `json:"participants_per_side"`
}

type SeedTournamentInput struct {
	ExpectedTournamentVersion int                   `json:"expected_tournament_version"`
	Seeds                     []TournamentSeedInput `json:"seeds"`
}

type TournamentSeedInput struct {
	Seed                     int       `json:"seed"`
	CompetitionSessionTeamID uuid.UUID `json:"competition_session_team_id"`
}

type LockTournamentTeamInput struct {
	ExpectedTournamentVersion int `json:"expected_tournament_version"`
	Seed                      int `json:"seed"`
}

type BindTournamentMatchInput struct {
	ExpectedTournamentVersion int         `json:"expected_tournament_version"`
	Round                     int         `json:"round"`
	MatchNumber               int         `json:"match_number"`
	CompetitionMatchID        uuid.UUID   `json:"competition_match_id"`
	TeamSnapshotIDs           []uuid.UUID `json:"team_snapshot_ids"`
}

type AdvanceTournamentRoundInput struct {
	ExpectedTournamentVersion int       `json:"expected_tournament_version"`
	MatchBindingID            uuid.UUID `json:"match_binding_id"`
	AdvanceReason             string    `json:"advance_reason"`
}

type TournamentSummary struct {
	ID                  uuid.UUID  `json:"id"`
	DisplayName         string     `json:"display_name"`
	Format              string     `json:"format"`
	Visibility          string     `json:"visibility"`
	SportKey            string     `json:"sport_key"`
	FacilityKey         string     `json:"facility_key"`
	ZoneKey             *string    `json:"zone_key,omitempty"`
	ParticipantsPerSide int        `json:"participants_per_side"`
	Status              string     `json:"status"`
	TournamentVersion   int        `json:"tournament_version"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	ArchivedAt          *time.Time `json:"archived_at,omitempty"`
}

type Tournament struct {
	TournamentSummary
	Brackets []TournamentBracket `json:"brackets"`
}

type TournamentBracket struct {
	ID            uuid.UUID                `json:"id"`
	TournamentID  uuid.UUID                `json:"tournament_id"`
	BracketIndex  int                      `json:"bracket_index"`
	Format        string                   `json:"format"`
	Status        string                   `json:"status"`
	CreatedAt     time.Time                `json:"created_at"`
	UpdatedAt     time.Time                `json:"updated_at"`
	Seeds         []TournamentSeed         `json:"seeds"`
	TeamSnapshots []TournamentTeamSnapshot `json:"team_snapshots"`
	MatchBindings []TournamentMatchBinding `json:"match_bindings"`
	Advancements  []TournamentAdvancement  `json:"advancements"`
}

type TournamentSeed struct {
	ID                       uuid.UUID `json:"id"`
	TournamentID             uuid.UUID `json:"tournament_id"`
	BracketID                uuid.UUID `json:"bracket_id"`
	Seed                     int       `json:"seed"`
	CompetitionSessionTeamID uuid.UUID `json:"competition_session_team_id"`
	SeededAt                 time.Time `json:"seeded_at"`
	CreatedAt                time.Time `json:"created_at"`
}

type TournamentTeamSnapshot struct {
	ID                       uuid.UUID                  `json:"id"`
	TournamentID             uuid.UUID                  `json:"tournament_id"`
	BracketID                uuid.UUID                  `json:"bracket_id"`
	TournamentSeedID         uuid.UUID                  `json:"tournament_seed_id"`
	Seed                     int                        `json:"seed"`
	CompetitionSessionID     uuid.UUID                  `json:"competition_session_id"`
	CompetitionSessionTeamID uuid.UUID                  `json:"competition_session_team_id"`
	RosterHash               string                     `json:"roster_hash"`
	LockedAt                 time.Time                  `json:"locked_at"`
	CreatedAt                time.Time                  `json:"created_at"`
	Members                  []TournamentSnapshotMember `json:"members"`
}

type TournamentSnapshotMember struct {
	TeamSnapshotID uuid.UUID `json:"team_snapshot_id"`
	UserID         uuid.UUID `json:"user_id"`
	DisplayName    string    `json:"display_name"`
	SlotIndex      int       `json:"slot_index"`
	CreatedAt      time.Time `json:"created_at"`
}

type TournamentMatchBinding struct {
	ID                 uuid.UUID   `json:"id"`
	TournamentID       uuid.UUID   `json:"tournament_id"`
	BracketID          uuid.UUID   `json:"bracket_id"`
	Round              int         `json:"round"`
	MatchNumber        int         `json:"match_number"`
	CompetitionMatchID uuid.UUID   `json:"competition_match_id"`
	TeamSnapshotIDs    []uuid.UUID `json:"team_snapshot_ids"`
	BoundAt            time.Time   `json:"bound_at"`
	CreatedAt          time.Time   `json:"created_at"`
}

type TournamentAdvancement struct {
	ID                    uuid.UUID `json:"id"`
	TournamentID          uuid.UUID `json:"tournament_id"`
	BracketID             uuid.UUID `json:"bracket_id"`
	MatchBindingID        uuid.UUID `json:"match_binding_id"`
	Round                 int       `json:"round"`
	WinningTeamSnapshotID uuid.UUID `json:"winning_team_snapshot_id"`
	LosingTeamSnapshotID  uuid.UUID `json:"losing_team_snapshot_id"`
	CompetitionMatchID    uuid.UUID `json:"competition_match_id"`
	CanonicalResultID     uuid.UUID `json:"canonical_result_id"`
	AdvanceReason         string    `json:"advance_reason"`
	AdvancedAt            time.Time `json:"advanced_at"`
	CreatedAt             time.Time `json:"created_at"`
}

type tournamentRecord struct {
	ID                  uuid.UUID
	OwnerUserID         uuid.UUID
	DisplayName         string
	Format              string
	Visibility          string
	SportKey            string
	FacilityKey         string
	ZoneKey             *string
	ParticipantsPerSide int
	Status              string
	TournamentVersion   int
	CreatedAt           time.Time
	UpdatedAt           time.Time
	ArchivedAt          *time.Time
}

type tournamentBracketRecord struct {
	ID           uuid.UUID
	TournamentID uuid.UUID
	BracketIndex int
	Format       string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type tournamentSeedRecord struct {
	ID                       uuid.UUID
	TournamentID             uuid.UUID
	BracketID                uuid.UUID
	Seed                     int
	CompetitionSessionTeamID uuid.UUID
	SeededAt                 time.Time
	CreatedAt                time.Time
}

type tournamentTeamSnapshotRecord struct {
	ID                       uuid.UUID
	TournamentID             uuid.UUID
	BracketID                uuid.UUID
	TournamentSeedID         uuid.UUID
	Seed                     int
	CompetitionSessionID     uuid.UUID
	CompetitionSessionTeamID uuid.UUID
	RosterHash               string
	LockedAt                 time.Time
	CreatedAt                time.Time
}

type tournamentSnapshotMemberRecord struct {
	TeamSnapshotID uuid.UUID
	UserID         uuid.UUID
	DisplayName    string
	SlotIndex      int
	CreatedAt      time.Time
}

type tournamentMatchBindingRecord struct {
	ID                    uuid.UUID
	TournamentID          uuid.UUID
	BracketID             uuid.UUID
	Round                 int
	MatchNumber           int
	CompetitionMatchID    uuid.UUID
	SideOneTeamSnapshotID uuid.UUID
	SideTwoTeamSnapshotID uuid.UUID
	BoundAt               time.Time
	CreatedAt             time.Time
}

type tournamentAdvancementRecord struct {
	ID                    uuid.UUID
	TournamentID          uuid.UUID
	BracketID             uuid.UUID
	MatchBindingID        uuid.UUID
	Round                 int
	WinningTeamSnapshotID uuid.UUID
	LosingTeamSnapshotID  uuid.UUID
	CompetitionMatchID    uuid.UUID
	CanonicalResultID     uuid.UUID
	AdvanceReason         string
	AdvancedAt            time.Time
	CreatedAt             time.Time
}

func (s *Service) ListTournaments(ctx context.Context) ([]TournamentSummary, error) {
	store, err := s.tournamentStore()
	if err != nil {
		return nil, err
	}

	rows, err := store.ListTournaments(ctx)
	if err != nil {
		return nil, err
	}

	summaries := make([]TournamentSummary, 0, len(rows))
	for _, row := range rows {
		summaries = append(summaries, buildTournamentSummary(row))
	}
	return summaries, nil
}

func (s *Service) GetTournament(ctx context.Context, tournamentID uuid.UUID) (Tournament, error) {
	tournament, err := s.loadTournament(ctx, tournamentID)
	if err != nil {
		return Tournament{}, err
	}
	return s.loadTournamentDetail(ctx, tournament)
}

func (s *Service) CreateTournament(ctx context.Context, actor StaffActor, input CreateTournamentInput) (Tournament, error) {
	store, err := s.tournamentStore()
	if err != nil {
		return Tournament{}, err
	}

	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		return Tournament{}, ErrTournamentNameRequired
	}
	format := normalizeTournamentFormat(input.Format)
	if format != TournamentFormatSingleElimination {
		return Tournament{}, ErrTournamentFormatUnsupported
	}

	sport, err := s.repository.GetSportConfig(ctx, strings.TrimSpace(input.SportKey))
	if err != nil {
		return Tournament{}, err
	}
	if sport == nil {
		return Tournament{}, ErrSportNotFound
	}
	if input.ParticipantsPerSide < sport.ParticipantsPerSideMin || input.ParticipantsPerSide > sport.ParticipantsPerSideMax {
		return Tournament{}, ErrParticipantsPerSide
	}
	if sport.SidesPerMatch != 2 {
		return Tournament{}, ErrTournamentFormatUnsupported
	}

	normalizedZone := normalizeOptionalText(input.ZoneKey)
	if err := s.validateFacilityBinding(ctx, sport.SportKey, strings.TrimSpace(input.FacilityKey), normalizedZone); err != nil {
		return Tournament{}, err
	}

	created, err := store.CreateTournament(ctx, actor, CreateTournamentInput{
		DisplayName:         displayName,
		Format:              format,
		SportKey:            sport.SportKey,
		FacilityKey:         strings.TrimSpace(input.FacilityKey),
		ZoneKey:             normalizedZone,
		ParticipantsPerSide: input.ParticipantsPerSide,
	}, s.now().UTC())
	if err != nil {
		if isUniqueViolation(err) {
			return Tournament{}, ErrDuplicateTournament
		}
		return Tournament{}, err
	}
	return s.loadTournamentDetail(ctx, created)
}

func (s *Service) SeedTournament(ctx context.Context, actor StaffActor, tournamentID uuid.UUID, input SeedTournamentInput) (Tournament, error) {
	store, err := s.tournamentStore()
	if err != nil {
		return Tournament{}, err
	}
	if input.ExpectedTournamentVersion <= 0 {
		return Tournament{}, ErrTournamentVersionRequired
	}

	tournament, bracket, err := s.loadTournamentMutationContext(ctx, tournamentID, input.ExpectedTournamentVersion)
	if err != nil {
		return Tournament{}, err
	}
	if tournament.Status != TournamentStatusDraft {
		return Tournament{}, ErrTournamentStatus
	}
	if err := s.validateTournamentSeedInput(ctx, tournament, input.Seeds); err != nil {
		return Tournament{}, err
	}

	updated, err := store.SeedTournament(ctx, actor, tournament, bracket, input, s.now().UTC())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Tournament{}, ErrTournamentStateStale
		}
		if isUniqueViolation(err) {
			return Tournament{}, ErrTournamentSeedDuplicate
		}
		return Tournament{}, err
	}
	return s.loadTournamentDetail(ctx, updated)
}

func (s *Service) LockTournamentTeam(ctx context.Context, actor StaffActor, tournamentID uuid.UUID, input LockTournamentTeamInput) (Tournament, error) {
	store, err := s.tournamentStore()
	if err != nil {
		return Tournament{}, err
	}
	if input.ExpectedTournamentVersion <= 0 {
		return Tournament{}, ErrTournamentVersionRequired
	}
	if input.Seed <= 0 {
		return Tournament{}, ErrTournamentSeedInvalid
	}

	tournament, bracket, err := s.loadTournamentMutationContext(ctx, tournamentID, input.ExpectedTournamentVersion)
	if err != nil {
		return Tournament{}, err
	}
	if tournament.Status != TournamentStatusSeeded && tournament.Status != TournamentStatusLocked {
		return Tournament{}, ErrTournamentStatus
	}

	seed, err := store.GetTournamentSeedByBracketSeed(ctx, bracket.ID, input.Seed)
	if err != nil {
		return Tournament{}, err
	}
	if seed == nil {
		return Tournament{}, ErrTournamentSeedNotFound
	}
	snapshots, err := store.ListTournamentTeamSnapshotsByBracketID(ctx, bracket.ID)
	if err != nil {
		return Tournament{}, err
	}
	for _, snapshot := range snapshots {
		if snapshot.Seed == seed.Seed || snapshot.CompetitionSessionTeamID == seed.CompetitionSessionTeamID {
			return Tournament{}, ErrTournamentTeamSnapshotLocked
		}
	}

	team, err := s.repository.GetTeamByID(ctx, seed.CompetitionSessionTeamID)
	if err != nil {
		return Tournament{}, err
	}
	if team == nil {
		return Tournament{}, ErrTeamNotFound
	}
	sourceSession, err := s.repository.GetSessionByID(ctx, team.SessionID)
	if err != nil {
		return Tournament{}, err
	}
	if sourceSession == nil {
		return Tournament{}, ErrSessionNotFound
	}
	if !tournamentMatchesSession(tournament, *sourceSession) {
		return Tournament{}, ErrTournamentTeamMismatch
	}

	allRoster, err := s.repository.ListRosterMembersBySessionID(ctx, sourceSession.ID)
	if err != nil {
		return Tournament{}, err
	}
	roster := rosterForTeam(allRoster, team.ID)
	if len(roster) != tournament.ParticipantsPerSide {
		return Tournament{}, ErrTeamSizeMismatch
	}
	slices.SortFunc(roster, func(a, b rosterRecord) int {
		if a.SlotIndex != b.SlotIndex {
			return a.SlotIndex - b.SlotIndex
		}
		return strings.Compare(a.UserID.String(), b.UserID.String())
	})

	updated, err := store.LockTournamentTeam(ctx, actor, tournament, bracket, *seed, *team, roster, tournamentRosterHash(roster), s.now().UTC())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Tournament{}, ErrTournamentStateStale
		}
		if isUniqueViolation(err) {
			return Tournament{}, ErrTournamentTeamSnapshotLocked
		}
		return Tournament{}, err
	}
	return s.loadTournamentDetail(ctx, updated)
}

func (s *Service) BindTournamentMatch(ctx context.Context, actor StaffActor, tournamentID uuid.UUID, input BindTournamentMatchInput) (Tournament, error) {
	store, err := s.tournamentStore()
	if err != nil {
		return Tournament{}, err
	}
	if input.ExpectedTournamentVersion <= 0 {
		return Tournament{}, ErrTournamentVersionRequired
	}
	if input.Round <= 0 || input.MatchNumber <= 0 {
		return Tournament{}, ErrTournamentRoundInvalid
	}
	if input.CompetitionMatchID == uuid.Nil || len(input.TeamSnapshotIDs) != 2 {
		return Tournament{}, ErrTournamentTeamSnapshotNotFound
	}
	if input.TeamSnapshotIDs[0] == input.TeamSnapshotIDs[1] {
		return Tournament{}, ErrTournamentTeamMismatch
	}

	tournament, bracket, err := s.loadTournamentMutationContext(ctx, tournamentID, input.ExpectedTournamentVersion)
	if err != nil {
		return Tournament{}, err
	}
	if tournament.Status != TournamentStatusLocked && tournament.Status != TournamentStatusInProgress {
		return Tournament{}, ErrTournamentStatus
	}

	seeds, err := store.ListTournamentSeedsByBracketID(ctx, bracket.ID)
	if err != nil {
		return Tournament{}, err
	}
	if !isPowerOfTwo(len(seeds)) || len(seeds) < 2 {
		return Tournament{}, ErrTournamentSeedCount
	}
	totalRounds := tournamentTotalRounds(len(seeds))
	if input.Round > totalRounds || input.MatchNumber > tournamentMatchesInRound(len(seeds), input.Round) {
		return Tournament{}, ErrTournamentRoundInvalid
	}

	sideOne, err := store.GetTournamentTeamSnapshotByID(ctx, input.TeamSnapshotIDs[0])
	if err != nil {
		return Tournament{}, err
	}
	sideTwo, err := store.GetTournamentTeamSnapshotByID(ctx, input.TeamSnapshotIDs[1])
	if err != nil {
		return Tournament{}, err
	}
	if sideOne == nil || sideTwo == nil || sideOne.BracketID != bracket.ID || sideTwo.BracketID != bracket.ID {
		return Tournament{}, ErrTournamentTeamSnapshotNotFound
	}
	if input.Round == 1 && !validFirstRoundPairing(len(seeds), input.MatchNumber, sideOne.Seed, sideTwo.Seed) {
		return Tournament{}, ErrTournamentTeamMismatch
	}
	if input.Round > 1 {
		if err := s.ensureSnapshotsAdvancedFromPreviousRound(ctx, store, bracket.ID, input.Round, input.MatchNumber, []uuid.UUID{sideOne.ID, sideTwo.ID}); err != nil {
			return Tournament{}, err
		}
	}

	match, err := s.repository.GetMatchByID(ctx, input.CompetitionMatchID)
	if err != nil {
		return Tournament{}, err
	}
	if match == nil {
		return Tournament{}, ErrMatchNotFound
	}
	if match.Status == MatchStatusArchived {
		return Tournament{}, ErrMatchArchived
	}
	sourceSession, err := s.repository.GetSessionByID(ctx, match.SessionID)
	if err != nil {
		return Tournament{}, err
	}
	if sourceSession == nil {
		return Tournament{}, ErrSessionNotFound
	}
	if !tournamentMatchesSession(tournament, *sourceSession) {
		return Tournament{}, ErrTournamentTeamMismatch
	}
	if err := s.validateTournamentMatchSlots(ctx, *match, *sideOne, *sideTwo); err != nil {
		return Tournament{}, err
	}

	updated, err := store.BindTournamentMatch(ctx, actor, tournament, bracket, input, *sideOne, *sideTwo, s.now().UTC())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Tournament{}, ErrTournamentStateStale
		}
		if isUniqueViolation(err) {
			return Tournament{}, ErrTournamentMatchBindingDuplicate
		}
		return Tournament{}, err
	}
	return s.loadTournamentDetail(ctx, updated)
}

func (s *Service) AdvanceTournamentRound(ctx context.Context, actor StaffActor, tournamentID uuid.UUID, input AdvanceTournamentRoundInput) (Tournament, error) {
	store, err := s.tournamentStore()
	if err != nil {
		return Tournament{}, err
	}
	if input.ExpectedTournamentVersion <= 0 {
		return Tournament{}, ErrTournamentVersionRequired
	}
	if input.MatchBindingID == uuid.Nil {
		return Tournament{}, ErrTournamentMatchBindingNotFound
	}
	reason := strings.TrimSpace(input.AdvanceReason)
	if reason != AdvanceReasonCanonicalResultWin {
		return Tournament{}, ErrTournamentAdvanceReason
	}

	tournament, bracket, err := s.loadTournamentMutationContext(ctx, tournamentID, input.ExpectedTournamentVersion)
	if err != nil {
		return Tournament{}, err
	}
	if tournament.Status != TournamentStatusInProgress && tournament.Status != TournamentStatusLocked {
		return Tournament{}, ErrTournamentStatus
	}

	binding, err := store.GetTournamentMatchBindingByID(ctx, input.MatchBindingID)
	if err != nil {
		return Tournament{}, err
	}
	if binding == nil || binding.BracketID != bracket.ID {
		return Tournament{}, ErrTournamentMatchBindingNotFound
	}

	match, err := s.repository.GetMatchByID(ctx, binding.CompetitionMatchID)
	if err != nil {
		return Tournament{}, err
	}
	if match == nil {
		return Tournament{}, ErrMatchNotFound
	}
	if match.CanonicalResultID == nil {
		return Tournament{}, ErrTournamentAdvanceResultRequired
	}
	result, err := s.repository.GetMatchResultByMatchID(ctx, match.ID)
	if err != nil {
		return Tournament{}, err
	}
	if result == nil || result.ID != *match.CanonicalResultID {
		return Tournament{}, ErrMatchResultNotCanonical
	}
	if result.ResultStatus != ResultStatusFinalized && result.ResultStatus != ResultStatusCorrected {
		return Tournament{}, ErrMatchResultNotFinal
	}

	resultSides, err := store.ListMatchResultSidesByResultID(ctx, result.ID)
	if err != nil {
		return Tournament{}, err
	}
	sideOne, err := store.GetTournamentTeamSnapshotByID(ctx, binding.SideOneTeamSnapshotID)
	if err != nil {
		return Tournament{}, err
	}
	sideTwo, err := store.GetTournamentTeamSnapshotByID(ctx, binding.SideTwoTeamSnapshotID)
	if err != nil {
		return Tournament{}, err
	}
	if sideOne == nil || sideTwo == nil {
		return Tournament{}, ErrTournamentTeamSnapshotNotFound
	}
	winner, loser, err := resolveTournamentWinner(resultSides, *sideOne, *sideTwo)
	if err != nil {
		return Tournament{}, err
	}

	seeds, err := store.ListTournamentSeedsByBracketID(ctx, bracket.ID)
	if err != nil {
		return Tournament{}, err
	}
	finalRound := binding.Round == tournamentTotalRounds(len(seeds))

	updated, err := store.AdvanceTournamentRound(ctx, actor, tournament, bracket, *binding, AdvanceTournamentRoundInput{
		ExpectedTournamentVersion: input.ExpectedTournamentVersion,
		MatchBindingID:            input.MatchBindingID,
		AdvanceReason:             reason,
	}, winner, loser, result.ID, finalRound, s.now().UTC())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Tournament{}, ErrTournamentStateStale
		}
		if isUniqueViolation(err) {
			return Tournament{}, ErrTournamentAdvanceDuplicate
		}
		return Tournament{}, err
	}
	return s.loadTournamentDetail(ctx, updated)
}

func (s *Service) loadTournament(ctx context.Context, tournamentID uuid.UUID) (tournamentRecord, error) {
	store, err := s.tournamentStore()
	if err != nil {
		return tournamentRecord{}, err
	}
	tournament, err := store.GetTournamentByID(ctx, tournamentID)
	if err != nil {
		return tournamentRecord{}, err
	}
	if tournament == nil {
		return tournamentRecord{}, ErrTournamentNotFound
	}
	return *tournament, nil
}

func (s *Service) loadTournamentMutationContext(ctx context.Context, tournamentID uuid.UUID, expectedVersion int) (tournamentRecord, tournamentBracketRecord, error) {
	tournament, err := s.loadTournament(ctx, tournamentID)
	if err != nil {
		return tournamentRecord{}, tournamentBracketRecord{}, err
	}
	if tournament.Status == TournamentStatusArchived || tournament.Status == TournamentStatusCompleted {
		return tournamentRecord{}, tournamentBracketRecord{}, ErrTournamentStatus
	}
	if tournament.TournamentVersion != expectedVersion {
		return tournamentRecord{}, tournamentBracketRecord{}, ErrTournamentStateStale
	}

	store, err := s.tournamentStore()
	if err != nil {
		return tournamentRecord{}, tournamentBracketRecord{}, err
	}
	brackets, err := store.ListTournamentBracketsByTournamentID(ctx, tournament.ID)
	if err != nil {
		return tournamentRecord{}, tournamentBracketRecord{}, err
	}
	if len(brackets) != 1 {
		return tournamentRecord{}, tournamentBracketRecord{}, ErrTournamentBracketNotFound
	}
	return tournament, brackets[0], nil
}

func (s *Service) loadTournamentDetail(ctx context.Context, tournament tournamentRecord) (Tournament, error) {
	store, err := s.tournamentStore()
	if err != nil {
		return Tournament{}, err
	}

	bracketRecords, err := store.ListTournamentBracketsByTournamentID(ctx, tournament.ID)
	if err != nil {
		return Tournament{}, err
	}

	brackets := make([]TournamentBracket, 0, len(bracketRecords))
	for _, bracket := range bracketRecords {
		seeds, err := store.ListTournamentSeedsByBracketID(ctx, bracket.ID)
		if err != nil {
			return Tournament{}, err
		}
		snapshots, err := store.ListTournamentTeamSnapshotsByBracketID(ctx, bracket.ID)
		if err != nil {
			return Tournament{}, err
		}
		members, err := store.ListTournamentTeamSnapshotMembersByBracketID(ctx, bracket.ID)
		if err != nil {
			return Tournament{}, err
		}
		bindings, err := store.ListTournamentMatchBindingsByBracketID(ctx, bracket.ID)
		if err != nil {
			return Tournament{}, err
		}
		advancements, err := store.ListTournamentAdvancementsByBracketID(ctx, bracket.ID)
		if err != nil {
			return Tournament{}, err
		}

		brackets = append(brackets, buildTournamentBracket(bracket, seeds, snapshots, members, bindings, advancements))
	}

	return Tournament{
		TournamentSummary: buildTournamentSummary(tournament),
		Brackets:          brackets,
	}, nil
}

func (s *Service) tournamentStore() (tournamentStore, error) {
	store, ok := s.repository.(tournamentStore)
	if !ok {
		return nil, ErrTournamentStoreUnavailable
	}
	return store, nil
}

func (s *Service) validateTournamentSeedInput(ctx context.Context, tournament tournamentRecord, seeds []TournamentSeedInput) error {
	if len(seeds) < 2 || !isPowerOfTwo(len(seeds)) {
		return ErrTournamentSeedCount
	}

	seedSeen := make(map[int]struct{}, len(seeds))
	teamSeen := make(map[uuid.UUID]struct{}, len(seeds))
	ordered := make([]int, 0, len(seeds))
	for _, seed := range seeds {
		if seed.Seed <= 0 || seed.CompetitionSessionTeamID == uuid.Nil {
			return ErrTournamentSeedInvalid
		}
		if _, exists := seedSeen[seed.Seed]; exists {
			return ErrTournamentSeedDuplicate
		}
		if _, exists := teamSeen[seed.CompetitionSessionTeamID]; exists {
			return ErrTournamentSeedDuplicate
		}
		seedSeen[seed.Seed] = struct{}{}
		teamSeen[seed.CompetitionSessionTeamID] = struct{}{}
		ordered = append(ordered, seed.Seed)
	}
	slices.Sort(ordered)
	for index, seed := range ordered {
		if seed != index+1 {
			return ErrTournamentSeedInvalid
		}
	}

	for _, seed := range seeds {
		team, err := s.repository.GetTeamByID(ctx, seed.CompetitionSessionTeamID)
		if err != nil {
			return err
		}
		if team == nil {
			return ErrTeamNotFound
		}
		session, err := s.repository.GetSessionByID(ctx, team.SessionID)
		if err != nil {
			return err
		}
		if session == nil {
			return ErrSessionNotFound
		}
		if !tournamentMatchesSession(tournament, *session) {
			return ErrTournamentTeamMismatch
		}
		memberCount, err := s.repository.CountRosterMembersByTeamID(ctx, team.ID)
		if err != nil {
			return err
		}
		if memberCount != int64(tournament.ParticipantsPerSide) {
			return ErrTeamSizeMismatch
		}
	}
	return nil
}

func (s *Service) validateTournamentMatchSlots(ctx context.Context, match matchRecord, sideOne tournamentTeamSnapshotRecord, sideTwo tournamentTeamSnapshotRecord) error {
	slots, err := s.repository.ListMatchSideSlotsBySessionID(ctx, match.SessionID)
	if err != nil {
		return err
	}
	matched := make(map[uuid.UUID]struct{}, 2)
	for _, slot := range slots {
		if slot.MatchID != match.ID {
			continue
		}
		switch slot.TeamID {
		case sideOne.CompetitionSessionTeamID:
			matched[sideOne.CompetitionSessionTeamID] = struct{}{}
		case sideTwo.CompetitionSessionTeamID:
			matched[sideTwo.CompetitionSessionTeamID] = struct{}{}
		default:
			return ErrTournamentTeamMismatch
		}
	}
	if len(matched) != 2 {
		return ErrTournamentTeamMismatch
	}
	return nil
}

func (s *Service) ensureSnapshotsAdvancedFromPreviousRound(ctx context.Context, store tournamentStore, bracketID uuid.UUID, round int, matchNumber int, snapshotIDs []uuid.UUID) error {
	advancements, err := store.ListTournamentAdvancementsByBracketID(ctx, bracketID)
	if err != nil {
		return err
	}
	bindings, err := store.ListTournamentMatchBindingsByBracketID(ctx, bracketID)
	if err != nil {
		return err
	}
	bindingsByID := make(map[uuid.UUID]tournamentMatchBindingRecord, len(bindings))
	for _, binding := range bindings {
		bindingsByID[binding.ID] = binding
	}

	firstPreviousMatch, secondPreviousMatch := previousRoundMatchNumbers(matchNumber)
	winnersByPreviousMatch := make(map[int]uuid.UUID, 2)
	for _, advancement := range advancements {
		if advancement.Round != round-1 {
			continue
		}
		binding, ok := bindingsByID[advancement.MatchBindingID]
		if !ok || binding.Round != round-1 {
			return ErrTournamentMatchBindingNotFound
		}
		if binding.MatchNumber == firstPreviousMatch || binding.MatchNumber == secondPreviousMatch {
			winnersByPreviousMatch[binding.MatchNumber] = advancement.WinningTeamSnapshotID
		}
	}

	firstWinner, firstReady := winnersByPreviousMatch[firstPreviousMatch]
	secondWinner, secondReady := winnersByPreviousMatch[secondPreviousMatch]
	if !firstReady || !secondReady {
		return ErrTournamentTeamMismatch
	}
	snapshotSet := make(map[uuid.UUID]struct{}, len(snapshotIDs))
	for _, snapshotID := range snapshotIDs {
		snapshotSet[snapshotID] = struct{}{}
	}
	if len(snapshotSet) != 2 {
		return ErrTournamentTeamMismatch
	}
	_, hasFirstWinner := snapshotSet[firstWinner]
	_, hasSecondWinner := snapshotSet[secondWinner]
	if !hasFirstWinner || !hasSecondWinner {
		return ErrTournamentTeamMismatch
	}
	return nil
}

func buildTournamentSummary(record tournamentRecord) TournamentSummary {
	return TournamentSummary{
		ID:                  record.ID,
		DisplayName:         record.DisplayName,
		Format:              record.Format,
		Visibility:          record.Visibility,
		SportKey:            record.SportKey,
		FacilityKey:         record.FacilityKey,
		ZoneKey:             record.ZoneKey,
		ParticipantsPerSide: record.ParticipantsPerSide,
		Status:              record.Status,
		TournamentVersion:   record.TournamentVersion,
		CreatedAt:           record.CreatedAt,
		UpdatedAt:           record.UpdatedAt,
		ArchivedAt:          record.ArchivedAt,
	}
}

func buildTournamentBracket(bracket tournamentBracketRecord, seeds []tournamentSeedRecord, snapshots []tournamentTeamSnapshotRecord, members []tournamentSnapshotMemberRecord, bindings []tournamentMatchBindingRecord, advancements []tournamentAdvancementRecord) TournamentBracket {
	membersBySnapshot := make(map[uuid.UUID][]TournamentSnapshotMember, len(snapshots))
	for _, member := range members {
		membersBySnapshot[member.TeamSnapshotID] = append(membersBySnapshot[member.TeamSnapshotID], TournamentSnapshotMember{
			TeamSnapshotID: member.TeamSnapshotID,
			UserID:         member.UserID,
			DisplayName:    member.DisplayName,
			SlotIndex:      member.SlotIndex,
			CreatedAt:      member.CreatedAt,
		})
	}

	return TournamentBracket{
		ID:            bracket.ID,
		TournamentID:  bracket.TournamentID,
		BracketIndex:  bracket.BracketIndex,
		Format:        bracket.Format,
		Status:        bracket.Status,
		CreatedAt:     bracket.CreatedAt,
		UpdatedAt:     bracket.UpdatedAt,
		Seeds:         buildTournamentSeeds(seeds),
		TeamSnapshots: buildTournamentTeamSnapshots(snapshots, membersBySnapshot),
		MatchBindings: buildTournamentMatchBindings(bindings),
		Advancements:  buildTournamentAdvancements(advancements),
	}
}

func buildTournamentSeeds(records []tournamentSeedRecord) []TournamentSeed {
	seeds := make([]TournamentSeed, 0, len(records))
	for _, record := range records {
		seeds = append(seeds, TournamentSeed{
			ID:                       record.ID,
			TournamentID:             record.TournamentID,
			BracketID:                record.BracketID,
			Seed:                     record.Seed,
			CompetitionSessionTeamID: record.CompetitionSessionTeamID,
			SeededAt:                 record.SeededAt,
			CreatedAt:                record.CreatedAt,
		})
	}
	return seeds
}

func buildTournamentTeamSnapshots(records []tournamentTeamSnapshotRecord, membersBySnapshot map[uuid.UUID][]TournamentSnapshotMember) []TournamentTeamSnapshot {
	snapshots := make([]TournamentTeamSnapshot, 0, len(records))
	for _, record := range records {
		snapshots = append(snapshots, TournamentTeamSnapshot{
			ID:                       record.ID,
			TournamentID:             record.TournamentID,
			BracketID:                record.BracketID,
			TournamentSeedID:         record.TournamentSeedID,
			Seed:                     record.Seed,
			CompetitionSessionID:     record.CompetitionSessionID,
			CompetitionSessionTeamID: record.CompetitionSessionTeamID,
			RosterHash:               record.RosterHash,
			LockedAt:                 record.LockedAt,
			CreatedAt:                record.CreatedAt,
			Members:                  membersBySnapshot[record.ID],
		})
	}
	return snapshots
}

func buildTournamentMatchBindings(records []tournamentMatchBindingRecord) []TournamentMatchBinding {
	bindings := make([]TournamentMatchBinding, 0, len(records))
	for _, record := range records {
		bindings = append(bindings, TournamentMatchBinding{
			ID:                 record.ID,
			TournamentID:       record.TournamentID,
			BracketID:          record.BracketID,
			Round:              record.Round,
			MatchNumber:        record.MatchNumber,
			CompetitionMatchID: record.CompetitionMatchID,
			TeamSnapshotIDs:    []uuid.UUID{record.SideOneTeamSnapshotID, record.SideTwoTeamSnapshotID},
			BoundAt:            record.BoundAt,
			CreatedAt:          record.CreatedAt,
		})
	}
	return bindings
}

func buildTournamentAdvancements(records []tournamentAdvancementRecord) []TournamentAdvancement {
	advancements := make([]TournamentAdvancement, 0, len(records))
	for _, record := range records {
		advancements = append(advancements, TournamentAdvancement{
			ID:                    record.ID,
			TournamentID:          record.TournamentID,
			BracketID:             record.BracketID,
			MatchBindingID:        record.MatchBindingID,
			Round:                 record.Round,
			WinningTeamSnapshotID: record.WinningTeamSnapshotID,
			LosingTeamSnapshotID:  record.LosingTeamSnapshotID,
			CompetitionMatchID:    record.CompetitionMatchID,
			CanonicalResultID:     record.CanonicalResultID,
			AdvanceReason:         record.AdvanceReason,
			AdvancedAt:            record.AdvancedAt,
			CreatedAt:             record.CreatedAt,
		})
	}
	return advancements
}

func normalizeTournamentFormat(format string) string {
	normalized := strings.ToLower(strings.TrimSpace(format))
	if normalized == "" {
		return TournamentFormatSingleElimination
	}
	return normalized
}

func tournamentMatchesSession(tournament tournamentRecord, session sessionRecord) bool {
	if tournament.SportKey != session.SportKey ||
		tournament.FacilityKey != session.FacilityKey ||
		tournament.ParticipantsPerSide != session.ParticipantsPerSide {
		return false
	}
	if tournament.ZoneKey != nil {
		return session.ZoneKey != nil && *session.ZoneKey == *tournament.ZoneKey
	}
	return true
}

func rosterForTeam(roster []rosterRecord, teamID uuid.UUID) []rosterRecord {
	filtered := make([]rosterRecord, 0, len(roster))
	for _, member := range roster {
		if member.TeamID == teamID {
			filtered = append(filtered, member)
		}
	}
	return filtered
}

func tournamentRosterHash(roster []rosterRecord) string {
	hash := sha256.New()
	for _, member := range roster {
		_, _ = fmt.Fprintf(hash, "%d:%s:%s\n", member.SlotIndex, member.UserID, member.DisplayName)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func isPowerOfTwo(value int) bool {
	return value > 0 && value&(value-1) == 0
}

func tournamentTotalRounds(seedCount int) int {
	rounds := 0
	for seedCount > 1 {
		rounds++
		seedCount /= 2
	}
	return rounds
}

func tournamentMatchesInRound(seedCount int, round int) int {
	if round <= 0 {
		return 0
	}
	matches := seedCount
	for i := 0; i < round; i++ {
		matches /= 2
	}
	return matches
}

func previousRoundMatchNumbers(matchNumber int) (int, int) {
	return matchNumber*2 - 1, matchNumber * 2
}

func validFirstRoundPairing(seedCount int, matchNumber int, sideOneSeed int, sideTwoSeed int) bool {
	if matchNumber <= 0 || matchNumber > seedCount/2 {
		return false
	}
	expectedA := matchNumber
	expectedB := seedCount + 1 - matchNumber
	return (sideOneSeed == expectedA && sideTwoSeed == expectedB) ||
		(sideOneSeed == expectedB && sideTwoSeed == expectedA)
}

func resolveTournamentWinner(sides []matchResultSideRecord, sideOne tournamentTeamSnapshotRecord, sideTwo tournamentTeamSnapshotRecord) (tournamentTeamSnapshotRecord, tournamentTeamSnapshotRecord, error) {
	var winnerTeamID uuid.UUID
	var loserTeamID uuid.UUID
	for _, side := range sides {
		if side.CompetitionSessionTeamID != sideOne.CompetitionSessionTeamID && side.CompetitionSessionTeamID != sideTwo.CompetitionSessionTeamID {
			return tournamentTeamSnapshotRecord{}, tournamentTeamSnapshotRecord{}, ErrTournamentTeamMismatch
		}
		switch side.Outcome {
		case matchOutcomeWin:
			if winnerTeamID != uuid.Nil {
				return tournamentTeamSnapshotRecord{}, tournamentTeamSnapshotRecord{}, ErrTournamentAdvanceResultOutcome
			}
			winnerTeamID = side.CompetitionSessionTeamID
		case matchOutcomeLoss:
			if loserTeamID != uuid.Nil {
				return tournamentTeamSnapshotRecord{}, tournamentTeamSnapshotRecord{}, ErrTournamentAdvanceResultOutcome
			}
			loserTeamID = side.CompetitionSessionTeamID
		default:
			return tournamentTeamSnapshotRecord{}, tournamentTeamSnapshotRecord{}, ErrTournamentAdvanceResultOutcome
		}
	}
	if winnerTeamID == uuid.Nil || loserTeamID == uuid.Nil || winnerTeamID == loserTeamID {
		return tournamentTeamSnapshotRecord{}, tournamentTeamSnapshotRecord{}, ErrTournamentAdvanceResultOutcome
	}
	if winnerTeamID == sideOne.CompetitionSessionTeamID && loserTeamID == sideTwo.CompetitionSessionTeamID {
		return sideOne, sideTwo, nil
	}
	if winnerTeamID == sideTwo.CompetitionSessionTeamID && loserTeamID == sideOne.CompetitionSessionTeamID {
		return sideTwo, sideOne, nil
	}
	return tournamentTeamSnapshotRecord{}, tournamentTeamSnapshotRecord{}, ErrTournamentTeamMismatch
}
