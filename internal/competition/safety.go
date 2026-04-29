package competition

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/store"
)

const (
	SafetyTargetCompetitionSession    = "competition_session"
	SafetyTargetCompetitionMatch      = "competition_match"
	SafetyTargetCompetitionTeam       = "competition_team"
	SafetyTargetCompetitionTournament = "competition_tournament"
	SafetyTargetCompetitionMember     = "competition_member"

	SafetyReasonConduct     = "conduct"
	SafetyReasonHarassment  = "harassment"
	SafetyReasonUnsafePlay  = "unsafe_play"
	SafetyReasonNoShow      = "no_show"
	SafetyReasonReliability = "reliability"
	SafetyReasonEligibility = "eligibility"
	SafetyReasonOther       = "other"

	ReliabilityTypeLateArrival        = "late_arrival"
	ReliabilityTypeNoShow             = "no_show"
	ReliabilityTypeForfeit            = "forfeit"
	ReliabilityTypeDisconnect         = "disconnect"
	ReliabilityTypeOpsDelay           = "ops_delay"
	ReliabilityTypeEquipmentIssue     = "equipment_issue"
	ReliabilityTypeUnsafeInterruption = "unsafe_interruption"
	ReliabilityTypeOther              = "other"

	ReliabilitySeverityInfo     = "info"
	ReliabilitySeverityWarning  = "warning"
	ReliabilitySeverityCritical = "critical"

	safetyReportEventRecorded      = "competition.safety.report_recorded"
	safetyBlockEventRecorded       = "competition.safety.block_recorded"
	reliabilityEventRecordRecorded = "competition.reliability.event_recorded"

	competitionSafetyBlocksActivePairUnique = "idx_competition_safety_blocks_active_pair"
)

var (
	ErrSafetyStoreUnavailable    = errors.New("competition safety store is unavailable")
	ErrSafetyActorAttribution    = errors.New("competition safety actor attribution is required")
	ErrSafetyActorTrustedSurface = errors.New("competition safety trusted surface is required")
	ErrSafetyTargetType          = errors.New("competition safety target_type is invalid")
	ErrSafetyTargetRequired      = errors.New("competition safety target is required")
	ErrSafetyReasonCode          = errors.New("competition safety reason_code is invalid")
	ErrSafetyReporterRequired    = errors.New("competition safety reporter_user_id is required")
	ErrSafetySubjectRequired     = errors.New("competition safety subject_user_id is required")
	ErrSafetyUserOutOfScope      = errors.New("competition safety user is outside competition scope")
	ErrSafetyBlockPairInvalid    = errors.New("competition safety block pair is invalid")
	ErrSafetyBlockAlreadyExists  = errors.New("competition safety block already exists")
	ErrReliabilityType           = errors.New("competition reliability_type is invalid")
	ErrReliabilitySeverity       = errors.New("competition reliability severity is invalid")
	ErrSafetyReviewLimit         = errors.New("competition safety review limit is invalid")
)

type safetyStore interface {
	CreateSafetyReport(ctx context.Context, actor StaffActor, input normalizedSafetyReportInput, occurredAt time.Time) (safetyReportRecord, safetyEventRecord, error)
	CreateSafetyBlock(ctx context.Context, actor StaffActor, input normalizedSafetyBlockInput, occurredAt time.Time) (safetyBlockRecord, safetyEventRecord, error)
	CreateReliabilityEvent(ctx context.Context, actor StaffActor, input normalizedReliabilityEventInput, occurredAt time.Time) (reliabilityEventRecord, safetyEventRecord, error)
	GetSafetyReviewSummary(ctx context.Context) (safetyReviewSummaryRecord, error)
	ListSafetyReportsForReview(ctx context.Context, limit int) ([]safetyReportRecord, error)
	ListSafetyBlocksForReview(ctx context.Context, limit int) ([]safetyBlockRecord, error)
	ListReliabilityEventsForReview(ctx context.Context, limit int) ([]reliabilityEventRecord, error)
	ListSafetyAuditEventsForReview(ctx context.Context, limit int) ([]safetyEventRecord, error)
}

type RecordSafetyReportInput struct {
	CompetitionSessionID     uuid.UUID `json:"competition_session_id,omitempty"`
	CompetitionMatchID       uuid.UUID `json:"competition_match_id,omitempty"`
	CompetitionSessionTeamID uuid.UUID `json:"competition_session_team_id,omitempty"`
	CompetitionTournamentID  uuid.UUID `json:"competition_tournament_id,omitempty"`
	ReporterUserID           uuid.UUID `json:"reporter_user_id"`
	SubjectUserID            uuid.UUID `json:"subject_user_id,omitempty"`
	TargetType               string    `json:"target_type"`
	TargetID                 uuid.UUID `json:"target_id,omitempty"`
	ReasonCode               string    `json:"reason_code"`
	Note                     string    `json:"note,omitempty"`
}

type RecordSafetyBlockInput struct {
	CompetitionSessionID uuid.UUID `json:"competition_session_id"`
	CompetitionMatchID   uuid.UUID `json:"competition_match_id,omitempty"`
	BlockerUserID        uuid.UUID `json:"blocker_user_id"`
	BlockedUserID        uuid.UUID `json:"blocked_user_id"`
	ReasonCode           string    `json:"reason_code"`
}

type RecordReliabilityEventInput struct {
	CompetitionSessionID uuid.UUID `json:"competition_session_id"`
	CompetitionMatchID   uuid.UUID `json:"competition_match_id,omitempty"`
	SubjectUserID        uuid.UUID `json:"subject_user_id,omitempty"`
	ReliabilityType      string    `json:"reliability_type"`
	Severity             string    `json:"severity"`
	Note                 string    `json:"note,omitempty"`
}

type CompetitionSafetyReadiness struct {
	Status       string                         `json:"status"`
	Message      string                         `json:"message"`
	Actor        CompetitionCommandActor        `json:"actor"`
	Capabilities []authz.Capability             `json:"capabilities"`
	Commands     []CompetitionCommandCapability `json:"commands"`
	Summary      CompetitionSafetySummary       `json:"summary"`
}

type CompetitionSafetyReview struct {
	Summary           CompetitionSafetySummary `json:"summary"`
	Reports           []SafetyReport           `json:"reports"`
	Blocks            []SafetyBlock            `json:"blocks"`
	ReliabilityEvents []ReliabilityEvent       `json:"reliability_events"`
	AuditEvents       []SafetyAuditEvent       `json:"audit_events"`
}

type CompetitionSafetySummary struct {
	ReportCount           int64 `json:"report_count"`
	BlockCount            int64 `json:"block_count"`
	ReliabilityEventCount int64 `json:"reliability_event_count"`
	AuditEventCount       int64 `json:"audit_event_count"`
}

type SafetyReport struct {
	ID                       uuid.UUID      `json:"id"`
	CompetitionSessionID     *uuid.UUID     `json:"competition_session_id,omitempty"`
	CompetitionMatchID       *uuid.UUID     `json:"competition_match_id,omitempty"`
	CompetitionSessionTeamID *uuid.UUID     `json:"competition_session_team_id,omitempty"`
	CompetitionTournamentID  *uuid.UUID     `json:"competition_tournament_id,omitempty"`
	ReporterUserID           uuid.UUID      `json:"reporter_user_id"`
	SubjectUserID            *uuid.UUID     `json:"subject_user_id,omitempty"`
	TargetType               string         `json:"target_type"`
	TargetID                 uuid.UUID      `json:"target_id"`
	ReasonCode               string         `json:"reason_code"`
	Status                   string         `json:"status"`
	PrivacyScope             string         `json:"privacy_scope"`
	Note                     *string        `json:"note,omitempty"`
	Actor                    SafetyActorRef `json:"actor"`
	OccurredAt               time.Time      `json:"occurred_at"`
	CreatedAt                time.Time      `json:"created_at"`
}

type SafetyBlock struct {
	ID                   uuid.UUID      `json:"id"`
	CompetitionSessionID uuid.UUID      `json:"competition_session_id"`
	CompetitionMatchID   *uuid.UUID     `json:"competition_match_id,omitempty"`
	BlockerUserID        uuid.UUID      `json:"blocker_user_id"`
	BlockedUserID        uuid.UUID      `json:"blocked_user_id"`
	ReasonCode           string         `json:"reason_code"`
	Status               string         `json:"status"`
	PrivacyScope         string         `json:"privacy_scope"`
	Actor                SafetyActorRef `json:"actor"`
	OccurredAt           time.Time      `json:"occurred_at"`
	CreatedAt            time.Time      `json:"created_at"`
}

type ReliabilityEvent struct {
	ID                   uuid.UUID      `json:"id"`
	CompetitionSessionID uuid.UUID      `json:"competition_session_id"`
	CompetitionMatchID   *uuid.UUID     `json:"competition_match_id,omitempty"`
	SubjectUserID        *uuid.UUID     `json:"subject_user_id,omitempty"`
	ReliabilityType      string         `json:"reliability_type"`
	Severity             string         `json:"severity"`
	PrivacyScope         string         `json:"privacy_scope"`
	Note                 *string        `json:"note,omitempty"`
	Actor                SafetyActorRef `json:"actor"`
	OccurredAt           time.Time      `json:"occurred_at"`
	CreatedAt            time.Time      `json:"created_at"`
}

type SafetyAuditEvent struct {
	ID                       uuid.UUID      `json:"id"`
	EventType                string         `json:"event_type"`
	CompetitionSessionID     *uuid.UUID     `json:"competition_session_id,omitempty"`
	CompetitionMatchID       *uuid.UUID     `json:"competition_match_id,omitempty"`
	CompetitionSessionTeamID *uuid.UUID     `json:"competition_session_team_id,omitempty"`
	CompetitionTournamentID  *uuid.UUID     `json:"competition_tournament_id,omitempty"`
	SafetyReportID           *uuid.UUID     `json:"safety_report_id,omitempty"`
	SafetyBlockID            *uuid.UUID     `json:"safety_block_id,omitempty"`
	ReliabilityEventID       *uuid.UUID     `json:"reliability_event_id,omitempty"`
	ReporterUserID           *uuid.UUID     `json:"reporter_user_id,omitempty"`
	SubjectUserID            *uuid.UUID     `json:"subject_user_id,omitempty"`
	BlockerUserID            *uuid.UUID     `json:"blocker_user_id,omitempty"`
	BlockedUserID            *uuid.UUID     `json:"blocked_user_id,omitempty"`
	TargetType               *string        `json:"target_type,omitempty"`
	TargetID                 *uuid.UUID     `json:"target_id,omitempty"`
	ReasonCode               *string        `json:"reason_code,omitempty"`
	ReliabilityType          *string        `json:"reliability_type,omitempty"`
	Severity                 *string        `json:"severity,omitempty"`
	PrivacyScope             string         `json:"privacy_scope"`
	Actor                    SafetyActorRef `json:"actor"`
	OccurredAt               time.Time      `json:"occurred_at"`
	CreatedAt                time.Time      `json:"created_at"`
}

type SafetyActorRef struct {
	UserID              uuid.UUID        `json:"user_id"`
	Role                authz.Role       `json:"role"`
	SessionID           uuid.UUID        `json:"session_id"`
	Capability          authz.Capability `json:"capability"`
	TrustedSurfaceKey   string           `json:"trusted_surface_key"`
	TrustedSurfaceLabel string           `json:"trusted_surface_label,omitempty"`
}

type normalizedSafetyReportInput struct {
	CompetitionSessionID     *uuid.UUID
	CompetitionMatchID       *uuid.UUID
	CompetitionSessionTeamID *uuid.UUID
	CompetitionTournamentID  *uuid.UUID
	ReporterUserID           uuid.UUID
	SubjectUserID            *uuid.UUID
	TargetType               string
	TargetID                 uuid.UUID
	ReasonCode               string
	Note                     string
}

type normalizedSafetyBlockInput struct {
	CompetitionSessionID uuid.UUID
	CompetitionMatchID   *uuid.UUID
	BlockerUserID        uuid.UUID
	BlockedUserID        uuid.UUID
	ReasonCode           string
}

type normalizedReliabilityEventInput struct {
	CompetitionSessionID uuid.UUID
	CompetitionMatchID   *uuid.UUID
	SubjectUserID        *uuid.UUID
	ReliabilityType      string
	Severity             string
	Note                 string
}

type safetyReviewSummaryRecord struct {
	ReportCount           int64
	BlockCount            int64
	ReliabilityEventCount int64
	AuditEventCount       int64
}

type safetyReportRecord struct {
	ID                       uuid.UUID
	CompetitionSessionID     *uuid.UUID
	CompetitionMatchID       *uuid.UUID
	CompetitionSessionTeamID *uuid.UUID
	CompetitionTournamentID  *uuid.UUID
	ReporterUserID           uuid.UUID
	SubjectUserID            *uuid.UUID
	TargetType               string
	TargetID                 uuid.UUID
	ReasonCode               string
	Status                   string
	PrivacyScope             string
	Note                     *string
	Actor                    SafetyActorRef
	OccurredAt               time.Time
	CreatedAt                time.Time
}

type safetyBlockRecord struct {
	ID                   uuid.UUID
	CompetitionSessionID uuid.UUID
	CompetitionMatchID   *uuid.UUID
	BlockerUserID        uuid.UUID
	BlockedUserID        uuid.UUID
	ReasonCode           string
	Status               string
	PrivacyScope         string
	Actor                SafetyActorRef
	OccurredAt           time.Time
	CreatedAt            time.Time
}

type reliabilityEventRecord struct {
	ID                   uuid.UUID
	CompetitionSessionID uuid.UUID
	CompetitionMatchID   *uuid.UUID
	SubjectUserID        *uuid.UUID
	ReliabilityType      string
	Severity             string
	PrivacyScope         string
	Note                 *string
	Actor                SafetyActorRef
	OccurredAt           time.Time
	CreatedAt            time.Time
}

type safetyEventRecord struct {
	ID                       uuid.UUID
	EventType                string
	CompetitionSessionID     *uuid.UUID
	CompetitionMatchID       *uuid.UUID
	CompetitionSessionTeamID *uuid.UUID
	CompetitionTournamentID  *uuid.UUID
	SafetyReportID           *uuid.UUID
	SafetyBlockID            *uuid.UUID
	ReliabilityEventID       *uuid.UUID
	ReporterUserID           *uuid.UUID
	SubjectUserID            *uuid.UUID
	BlockerUserID            *uuid.UUID
	BlockedUserID            *uuid.UUID
	TargetType               *string
	TargetID                 *uuid.UUID
	ReasonCode               *string
	ReliabilityType          *string
	Severity                 *string
	PrivacyScope             string
	Actor                    SafetyActorRef
	OccurredAt               time.Time
	CreatedAt                time.Time
}

func (s *Service) CompetitionSafetyReadiness(ctx context.Context, actor StaffActor) (CompetitionSafetyReadiness, error) {
	summary, err := s.safetySummary(ctx)
	if err != nil {
		return CompetitionSafetyReadiness{}, err
	}

	commandActor := commandActorFromStaffActor(actor)
	capabilities := authz.CapabilitiesForRole(actor.Role)
	status := "ready"
	message := "Competition safety and reliability contracts are available for this actor."
	if !authz.HasCapability(capabilities, authz.CapabilityCompetitionSafetyReview) {
		status = "unsupported_role"
		message = "This actor has no APOLLO competition safety/reliability capability."
	}

	commands := make([]CompetitionCommandCapability, 0, 3)
	for _, definition := range commandDefinitions {
		switch definition.Name {
		case CommandRecordSafetyReport, CommandRecordSafetyBlock, CommandRecordReliabilityEvent:
			available := authz.HasCapability(capabilities, definition.RequiredCapability)
			reason := ""
			if !available {
				reason = "required capability is missing"
			}
			commands = append(commands, CompetitionCommandCapability{
				CompetitionCommandDefinition: definition,
				Available:                    available,
				UnavailableReason:            reason,
			})
		}
	}

	return CompetitionSafetyReadiness{
		Status:       status,
		Message:      message,
		Actor:        commandActor,
		Capabilities: capabilities,
		Commands:     commands,
		Summary:      summary,
	}, nil
}

func (s *Service) GetCompetitionSafetyReview(ctx context.Context, actor StaffActor, limit int) (CompetitionSafetyReview, error) {
	if err := validateSafetyActor(actor); err != nil {
		return CompetitionSafetyReview{}, err
	}
	limit, err := normalizeSafetyReviewLimit(limit)
	if err != nil {
		return CompetitionSafetyReview{}, err
	}

	store, err := s.safetyStore()
	if err != nil {
		return CompetitionSafetyReview{}, err
	}
	summary, err := s.safetySummary(ctx)
	if err != nil {
		return CompetitionSafetyReview{}, err
	}
	reportRows, err := store.ListSafetyReportsForReview(ctx, limit)
	if err != nil {
		return CompetitionSafetyReview{}, err
	}
	blockRows, err := store.ListSafetyBlocksForReview(ctx, limit)
	if err != nil {
		return CompetitionSafetyReview{}, err
	}
	reliabilityRows, err := store.ListReliabilityEventsForReview(ctx, limit)
	if err != nil {
		return CompetitionSafetyReview{}, err
	}
	auditRows, err := store.ListSafetyAuditEventsForReview(ctx, limit)
	if err != nil {
		return CompetitionSafetyReview{}, err
	}

	review := CompetitionSafetyReview{Summary: summary}
	for _, row := range reportRows {
		review.Reports = append(review.Reports, buildSafetyReport(row))
	}
	for _, row := range blockRows {
		review.Blocks = append(review.Blocks, buildSafetyBlock(row))
	}
	for _, row := range reliabilityRows {
		review.ReliabilityEvents = append(review.ReliabilityEvents, buildReliabilityEvent(row))
	}
	for _, row := range auditRows {
		review.AuditEvents = append(review.AuditEvents, buildSafetyAuditEvent(row))
	}
	return review, nil
}

func (s *Service) RecordSafetyReport(ctx context.Context, actor StaffActor, input RecordSafetyReportInput) (SafetyReport, error) {
	if err := validateSafetyActor(actor); err != nil {
		return SafetyReport{}, err
	}
	normalized, err := s.normalizeSafetyReportInput(ctx, input)
	if err != nil {
		return SafetyReport{}, err
	}
	store, err := s.safetyStore()
	if err != nil {
		return SafetyReport{}, err
	}
	record, _, err := store.CreateSafetyReport(ctx, actor, normalized, s.now().UTC())
	if err != nil {
		return SafetyReport{}, mapSafetyStoreError(err)
	}
	return buildSafetyReport(record), nil
}

func (s *Service) RecordSafetyBlock(ctx context.Context, actor StaffActor, input RecordSafetyBlockInput) (SafetyBlock, error) {
	if err := validateSafetyActor(actor); err != nil {
		return SafetyBlock{}, err
	}
	normalized, err := s.normalizeSafetyBlockInput(ctx, input)
	if err != nil {
		return SafetyBlock{}, err
	}
	store, err := s.safetyStore()
	if err != nil {
		return SafetyBlock{}, err
	}
	record, _, err := store.CreateSafetyBlock(ctx, actor, normalized, s.now().UTC())
	if err != nil {
		return SafetyBlock{}, mapSafetyStoreError(err)
	}
	return buildSafetyBlock(record), nil
}

func (s *Service) RecordReliabilityEvent(ctx context.Context, actor StaffActor, input RecordReliabilityEventInput) (ReliabilityEvent, error) {
	if err := validateSafetyActor(actor); err != nil {
		return ReliabilityEvent{}, err
	}
	normalized, err := s.normalizeReliabilityEventInput(ctx, input)
	if err != nil {
		return ReliabilityEvent{}, err
	}
	store, err := s.safetyStore()
	if err != nil {
		return ReliabilityEvent{}, err
	}
	record, _, err := store.CreateReliabilityEvent(ctx, actor, normalized, s.now().UTC())
	if err != nil {
		return ReliabilityEvent{}, mapSafetyStoreError(err)
	}
	return buildReliabilityEvent(record), nil
}

func (s *Service) safetySummary(ctx context.Context) (CompetitionSafetySummary, error) {
	store, err := s.safetyStore()
	if err != nil {
		return CompetitionSafetySummary{}, err
	}
	summary, err := store.GetSafetyReviewSummary(ctx)
	if err != nil {
		return CompetitionSafetySummary{}, err
	}
	return CompetitionSafetySummary{
		ReportCount:           summary.ReportCount,
		BlockCount:            summary.BlockCount,
		ReliabilityEventCount: summary.ReliabilityEventCount,
		AuditEventCount:       summary.AuditEventCount,
	}, nil
}

func (s *Service) normalizeSafetyReportInput(ctx context.Context, input RecordSafetyReportInput) (normalizedSafetyReportInput, error) {
	reasonCode := strings.TrimSpace(input.ReasonCode)
	if !validSafetyReason(reasonCode) {
		return normalizedSafetyReportInput{}, ErrSafetyReasonCode
	}
	if input.ReporterUserID == uuid.Nil {
		return normalizedSafetyReportInput{}, ErrSafetyReporterRequired
	}
	if err := s.requireUser(ctx, input.ReporterUserID); err != nil {
		return normalizedSafetyReportInput{}, err
	}
	subjectUserID := uuidPtr(input.SubjectUserID)
	if subjectUserID != nil {
		if err := s.requireUser(ctx, *subjectUserID); err != nil {
			return normalizedSafetyReportInput{}, err
		}
	}

	targetType := strings.TrimSpace(input.TargetType)
	targetID := input.TargetID
	sessionID := uuidPtr(input.CompetitionSessionID)
	matchID := uuidPtr(input.CompetitionMatchID)
	teamID := uuidPtr(input.CompetitionSessionTeamID)
	tournamentID := uuidPtr(input.CompetitionTournamentID)
	var tournament *tournamentRecord

	switch targetType {
	case SafetyTargetCompetitionSession:
		if targetID == uuid.Nil {
			targetID = input.CompetitionSessionID
		}
		if targetID == uuid.Nil || (sessionID != nil && *sessionID != targetID) {
			return normalizedSafetyReportInput{}, ErrSafetyTargetRequired
		}
		if err := s.requireSession(ctx, targetID); err != nil {
			return normalizedSafetyReportInput{}, err
		}
		sessionID = &targetID
		matchID = nil
		teamID = nil
		tournamentID = nil
	case SafetyTargetCompetitionMatch:
		if targetID == uuid.Nil {
			targetID = input.CompetitionMatchID
		}
		if targetID == uuid.Nil || sessionID == nil {
			return normalizedSafetyReportInput{}, ErrSafetyTargetRequired
		}
		match, err := s.requireMatch(ctx, targetID)
		if err != nil {
			return normalizedSafetyReportInput{}, err
		}
		if match.SessionID != *sessionID {
			return normalizedSafetyReportInput{}, ErrSafetyTargetRequired
		}
		matchID = &targetID
		teamID = nil
		tournamentID = nil
	case SafetyTargetCompetitionTeam:
		if targetID == uuid.Nil {
			targetID = input.CompetitionSessionTeamID
		}
		if targetID == uuid.Nil || sessionID == nil {
			return normalizedSafetyReportInput{}, ErrSafetyTargetRequired
		}
		team, err := s.requireTeam(ctx, targetID)
		if err != nil {
			return normalizedSafetyReportInput{}, err
		}
		if team.SessionID != *sessionID {
			return normalizedSafetyReportInput{}, ErrSafetyTargetRequired
		}
		matchID = nil
		teamID = &targetID
		tournamentID = nil
	case SafetyTargetCompetitionTournament:
		if targetID == uuid.Nil {
			targetID = input.CompetitionTournamentID
		}
		if targetID == uuid.Nil {
			return normalizedSafetyReportInput{}, ErrSafetyTargetRequired
		}
		loadedTournament, err := s.loadTournament(ctx, targetID)
		if err != nil {
			return normalizedSafetyReportInput{}, err
		}
		tournament = &loadedTournament
		sessionID = nil
		matchID = nil
		teamID = nil
		tournamentID = &targetID
	case SafetyTargetCompetitionMember:
		if subjectUserID == nil {
			return normalizedSafetyReportInput{}, ErrSafetySubjectRequired
		}
		if targetID == uuid.Nil {
			targetID = *subjectUserID
		}
		if targetID != *subjectUserID || sessionID == nil {
			return normalizedSafetyReportInput{}, ErrSafetyTargetRequired
		}
		if err := s.requireSession(ctx, *sessionID); err != nil {
			return normalizedSafetyReportInput{}, err
		}
		matchID = nil
		teamID = nil
		tournamentID = nil
	default:
		return normalizedSafetyReportInput{}, ErrSafetyTargetType
	}

	if tournament != nil {
		if err := s.requireSafetyUserInTournament(ctx, *tournament, input.ReporterUserID); err != nil {
			return normalizedSafetyReportInput{}, err
		}
		if subjectUserID != nil {
			if err := s.requireSafetyUserInTournament(ctx, *tournament, *subjectUserID); err != nil {
				return normalizedSafetyReportInput{}, err
			}
		}
	} else {
		if err := s.requireSafetyUserInScope(ctx, sessionID, matchID, input.ReporterUserID); err != nil {
			return normalizedSafetyReportInput{}, err
		}
		if subjectUserID != nil {
			if err := s.requireSafetyUserInScope(ctx, sessionID, matchID, *subjectUserID); err != nil {
				return normalizedSafetyReportInput{}, err
			}
		}
	}

	return normalizedSafetyReportInput{
		CompetitionSessionID:     sessionID,
		CompetitionMatchID:       matchID,
		CompetitionSessionTeamID: teamID,
		CompetitionTournamentID:  tournamentID,
		ReporterUserID:           input.ReporterUserID,
		SubjectUserID:            subjectUserID,
		TargetType:               targetType,
		TargetID:                 targetID,
		ReasonCode:               reasonCode,
		Note:                     strings.TrimSpace(input.Note),
	}, nil
}

func (s *Service) normalizeSafetyBlockInput(ctx context.Context, input RecordSafetyBlockInput) (normalizedSafetyBlockInput, error) {
	reasonCode := strings.TrimSpace(input.ReasonCode)
	if !validSafetyReason(reasonCode) {
		return normalizedSafetyBlockInput{}, ErrSafetyReasonCode
	}
	if input.CompetitionSessionID == uuid.Nil {
		return normalizedSafetyBlockInput{}, ErrSafetyTargetRequired
	}
	if input.BlockerUserID == uuid.Nil || input.BlockedUserID == uuid.Nil || input.BlockerUserID == input.BlockedUserID {
		return normalizedSafetyBlockInput{}, ErrSafetyBlockPairInvalid
	}
	if err := s.requireSession(ctx, input.CompetitionSessionID); err != nil {
		return normalizedSafetyBlockInput{}, err
	}
	if err := s.requireUser(ctx, input.BlockerUserID); err != nil {
		return normalizedSafetyBlockInput{}, err
	}
	if err := s.requireUser(ctx, input.BlockedUserID); err != nil {
		return normalizedSafetyBlockInput{}, err
	}
	matchID := uuidPtr(input.CompetitionMatchID)
	if matchID != nil {
		match, err := s.requireMatch(ctx, *matchID)
		if err != nil {
			return normalizedSafetyBlockInput{}, err
		}
		if match.SessionID != input.CompetitionSessionID {
			return normalizedSafetyBlockInput{}, ErrSafetyTargetRequired
		}
	}
	if err := s.requireSafetyUserInScope(ctx, &input.CompetitionSessionID, matchID, input.BlockerUserID); err != nil {
		return normalizedSafetyBlockInput{}, err
	}
	if err := s.requireSafetyUserInScope(ctx, &input.CompetitionSessionID, matchID, input.BlockedUserID); err != nil {
		return normalizedSafetyBlockInput{}, err
	}
	return normalizedSafetyBlockInput{
		CompetitionSessionID: input.CompetitionSessionID,
		CompetitionMatchID:   matchID,
		BlockerUserID:        input.BlockerUserID,
		BlockedUserID:        input.BlockedUserID,
		ReasonCode:           reasonCode,
	}, nil
}

func (s *Service) normalizeReliabilityEventInput(ctx context.Context, input RecordReliabilityEventInput) (normalizedReliabilityEventInput, error) {
	reliabilityType := strings.TrimSpace(input.ReliabilityType)
	if !validReliabilityType(reliabilityType) {
		return normalizedReliabilityEventInput{}, ErrReliabilityType
	}
	severity := strings.TrimSpace(input.Severity)
	if !validReliabilitySeverity(severity) {
		return normalizedReliabilityEventInput{}, ErrReliabilitySeverity
	}
	if input.CompetitionSessionID == uuid.Nil {
		return normalizedReliabilityEventInput{}, ErrSafetyTargetRequired
	}
	if err := s.requireSession(ctx, input.CompetitionSessionID); err != nil {
		return normalizedReliabilityEventInput{}, err
	}
	matchID := uuidPtr(input.CompetitionMatchID)
	if matchID != nil {
		match, err := s.requireMatch(ctx, *matchID)
		if err != nil {
			return normalizedReliabilityEventInput{}, err
		}
		if match.SessionID != input.CompetitionSessionID {
			return normalizedReliabilityEventInput{}, ErrSafetyTargetRequired
		}
	}
	subjectUserID := uuidPtr(input.SubjectUserID)
	if subjectUserID != nil {
		if err := s.requireUser(ctx, *subjectUserID); err != nil {
			return normalizedReliabilityEventInput{}, err
		}
		if err := s.requireSafetyUserInScope(ctx, &input.CompetitionSessionID, matchID, *subjectUserID); err != nil {
			return normalizedReliabilityEventInput{}, err
		}
	}
	return normalizedReliabilityEventInput{
		CompetitionSessionID: input.CompetitionSessionID,
		CompetitionMatchID:   matchID,
		SubjectUserID:        subjectUserID,
		ReliabilityType:      reliabilityType,
		Severity:             severity,
		Note:                 strings.TrimSpace(input.Note),
	}, nil
}

func (s *Service) requireUser(ctx context.Context, userID uuid.UUID) error {
	user, err := s.repository.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}
	return nil
}

func (s *Service) requireSession(ctx context.Context, sessionID uuid.UUID) error {
	session, err := s.repository.GetSessionByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if session == nil {
		return ErrSessionNotFound
	}
	return nil
}

func (s *Service) requireMatch(ctx context.Context, matchID uuid.UUID) (matchRecord, error) {
	match, err := s.repository.GetMatchByID(ctx, matchID)
	if err != nil {
		return matchRecord{}, err
	}
	if match == nil {
		return matchRecord{}, ErrMatchNotFound
	}
	return *match, nil
}

func (s *Service) requireTeam(ctx context.Context, teamID uuid.UUID) (teamRecord, error) {
	team, err := s.repository.GetTeamByID(ctx, teamID)
	if err != nil {
		return teamRecord{}, err
	}
	if team == nil {
		return teamRecord{}, ErrTeamNotFound
	}
	return *team, nil
}

func (s *Service) requireSafetyUserInScope(ctx context.Context, sessionID *uuid.UUID, matchID *uuid.UUID, userID uuid.UUID) error {
	if matchID != nil {
		if sessionID == nil {
			return ErrSafetyTargetRequired
		}
		return s.requireSafetyUserInMatch(ctx, *sessionID, *matchID, userID)
	}
	if sessionID == nil {
		return ErrSafetyTargetRequired
	}
	return s.requireSafetyUserInSession(ctx, *sessionID, userID)
}

func (s *Service) requireSafetyUserInSession(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID) error {
	ok, err := s.repository.SessionHasRosterMemberUser(ctx, sessionID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrSafetyUserOutOfScope
	}
	return nil
}

func (s *Service) requireSafetyUserInMatch(ctx context.Context, sessionID uuid.UUID, matchID uuid.UUID, userID uuid.UUID) error {
	sideSlots, err := s.repository.ListMatchSideSlotsBySessionID(ctx, sessionID)
	if err != nil {
		return err
	}
	matchTeams := make(map[uuid.UUID]struct{})
	for _, slot := range sideSlots {
		if slot.MatchID == matchID {
			matchTeams[slot.TeamID] = struct{}{}
		}
	}
	if len(matchTeams) == 0 {
		return ErrSafetyUserOutOfScope
	}

	roster, err := s.repository.ListRosterMembersBySessionID(ctx, sessionID)
	if err != nil {
		return err
	}
	for _, member := range roster {
		if member.UserID != userID {
			continue
		}
		if _, ok := matchTeams[member.TeamID]; ok {
			return nil
		}
	}
	return ErrSafetyUserOutOfScope
}

func (s *Service) requireSafetyUserInTournament(ctx context.Context, tournament tournamentRecord, userID uuid.UUID) error {
	if tournament.OwnerUserID == userID {
		return nil
	}
	store, err := s.tournamentStore()
	if err != nil {
		return err
	}
	brackets, err := store.ListTournamentBracketsByTournamentID(ctx, tournament.ID)
	if err != nil {
		return err
	}
	for _, bracket := range brackets {
		members, err := store.ListTournamentTeamSnapshotMembersByBracketID(ctx, bracket.ID)
		if err != nil {
			return err
		}
		for _, member := range members {
			if member.UserID == userID {
				return nil
			}
		}

		seeds, err := store.ListTournamentSeedsByBracketID(ctx, bracket.ID)
		if err != nil {
			return err
		}
		for _, seed := range seeds {
			team, err := s.repository.GetTeamByID(ctx, seed.CompetitionSessionTeamID)
			if err != nil {
				return err
			}
			if team == nil {
				continue
			}
			roster, err := s.repository.ListRosterMembersBySessionID(ctx, team.SessionID)
			if err != nil {
				return err
			}
			for _, member := range roster {
				if member.TeamID == team.ID && member.UserID == userID {
					return nil
				}
			}
		}
	}
	return ErrSafetyUserOutOfScope
}

func (s *Service) safetyStore() (safetyStore, error) {
	store, ok := s.repository.(safetyStore)
	if !ok {
		return nil, ErrSafetyStoreUnavailable
	}
	return store, nil
}

func validateSafetyActor(actor StaffActor) error {
	if !authz.HasCapability(authz.CapabilitiesForRole(actor.Role), authz.CapabilityCompetitionSafetyReview) {
		return authz.ErrCapabilityDenied
	}
	if actor.UserID == uuid.Nil || actor.SessionID == uuid.Nil {
		return ErrSafetyActorAttribution
	}
	if actor.Capability != authz.CapabilityCompetitionSafetyReview {
		return authz.ErrCapabilityDenied
	}
	if strings.TrimSpace(actor.TrustedSurfaceKey) == "" {
		return ErrSafetyActorTrustedSurface
	}
	return nil
}

func normalizeSafetyReviewLimit(limit int) (int, error) {
	if limit == 0 {
		return 50, nil
	}
	if limit < 0 || limit > 100 {
		return 0, ErrSafetyReviewLimit
	}
	return limit, nil
}

func validSafetyReason(reason string) bool {
	switch reason {
	case SafetyReasonConduct, SafetyReasonHarassment, SafetyReasonUnsafePlay, SafetyReasonNoShow, SafetyReasonReliability, SafetyReasonEligibility, SafetyReasonOther:
		return true
	default:
		return false
	}
}

func validReliabilityType(reliabilityType string) bool {
	switch reliabilityType {
	case ReliabilityTypeLateArrival, ReliabilityTypeNoShow, ReliabilityTypeForfeit, ReliabilityTypeDisconnect, ReliabilityTypeOpsDelay, ReliabilityTypeEquipmentIssue, ReliabilityTypeUnsafeInterruption, ReliabilityTypeOther:
		return true
	default:
		return false
	}
}

func validReliabilitySeverity(severity string) bool {
	switch severity {
	case ReliabilitySeverityInfo, ReliabilitySeverityWarning, ReliabilitySeverityCritical:
		return true
	default:
		return false
	}
}

func mapSafetyStoreError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == competitionSafetyBlocksActivePairUnique {
		return ErrSafetyBlockAlreadyExists
	}
	return err
}

func buildSafetyReport(row safetyReportRecord) SafetyReport {
	return SafetyReport{
		ID:                       row.ID,
		CompetitionSessionID:     row.CompetitionSessionID,
		CompetitionMatchID:       row.CompetitionMatchID,
		CompetitionSessionTeamID: row.CompetitionSessionTeamID,
		CompetitionTournamentID:  row.CompetitionTournamentID,
		ReporterUserID:           row.ReporterUserID,
		SubjectUserID:            row.SubjectUserID,
		TargetType:               row.TargetType,
		TargetID:                 row.TargetID,
		ReasonCode:               row.ReasonCode,
		Status:                   row.Status,
		PrivacyScope:             row.PrivacyScope,
		Note:                     row.Note,
		Actor:                    row.Actor,
		OccurredAt:               row.OccurredAt,
		CreatedAt:                row.CreatedAt,
	}
}

func buildSafetyBlock(row safetyBlockRecord) SafetyBlock {
	return SafetyBlock{
		ID:                   row.ID,
		CompetitionSessionID: row.CompetitionSessionID,
		CompetitionMatchID:   row.CompetitionMatchID,
		BlockerUserID:        row.BlockerUserID,
		BlockedUserID:        row.BlockedUserID,
		ReasonCode:           row.ReasonCode,
		Status:               row.Status,
		PrivacyScope:         row.PrivacyScope,
		Actor:                row.Actor,
		OccurredAt:           row.OccurredAt,
		CreatedAt:            row.CreatedAt,
	}
}

func buildReliabilityEvent(row reliabilityEventRecord) ReliabilityEvent {
	return ReliabilityEvent{
		ID:                   row.ID,
		CompetitionSessionID: row.CompetitionSessionID,
		CompetitionMatchID:   row.CompetitionMatchID,
		SubjectUserID:        row.SubjectUserID,
		ReliabilityType:      row.ReliabilityType,
		Severity:             row.Severity,
		PrivacyScope:         row.PrivacyScope,
		Note:                 row.Note,
		Actor:                row.Actor,
		OccurredAt:           row.OccurredAt,
		CreatedAt:            row.CreatedAt,
	}
}

func buildSafetyAuditEvent(row safetyEventRecord) SafetyAuditEvent {
	return SafetyAuditEvent{
		ID:                       row.ID,
		EventType:                row.EventType,
		CompetitionSessionID:     row.CompetitionSessionID,
		CompetitionMatchID:       row.CompetitionMatchID,
		CompetitionSessionTeamID: row.CompetitionSessionTeamID,
		CompetitionTournamentID:  row.CompetitionTournamentID,
		SafetyReportID:           row.SafetyReportID,
		SafetyBlockID:            row.SafetyBlockID,
		ReliabilityEventID:       row.ReliabilityEventID,
		ReporterUserID:           row.ReporterUserID,
		SubjectUserID:            row.SubjectUserID,
		BlockerUserID:            row.BlockerUserID,
		BlockedUserID:            row.BlockedUserID,
		TargetType:               row.TargetType,
		TargetID:                 row.TargetID,
		ReasonCode:               row.ReasonCode,
		ReliabilityType:          row.ReliabilityType,
		Severity:                 row.Severity,
		PrivacyScope:             row.PrivacyScope,
		Actor:                    row.Actor,
		OccurredAt:               row.OccurredAt,
		CreatedAt:                row.CreatedAt,
	}
}

func safetyActorFromStore(userID uuid.UUID, role string, sessionID uuid.UUID, capability string, trustedSurfaceKey string, trustedSurfaceLabel *string) SafetyActorRef {
	actor := SafetyActorRef{
		UserID:            userID,
		Role:              authz.Role(role),
		SessionID:         sessionID,
		Capability:        authz.Capability(capability),
		TrustedSurfaceKey: trustedSurfaceKey,
	}
	if trustedSurfaceLabel != nil {
		actor.TrustedSurfaceLabel = *trustedSurfaceLabel
	}
	return actor
}

func safetyReportRecordFromStore(row store.ApolloCompetitionSafetyReport) safetyReportRecord {
	return safetyReportRecord{
		ID:                       row.ID,
		CompetitionSessionID:     uuidFromPgtype(row.CompetitionSessionID),
		CompetitionMatchID:       uuidFromPgtype(row.CompetitionMatchID),
		CompetitionSessionTeamID: uuidFromPgtype(row.CompetitionSessionTeamID),
		CompetitionTournamentID:  uuidFromPgtype(row.CompetitionTournamentID),
		ReporterUserID:           row.ReporterUserID,
		SubjectUserID:            uuidFromPgtype(row.SubjectUserID),
		TargetType:               row.TargetType,
		TargetID:                 row.TargetID,
		ReasonCode:               row.ReasonCode,
		Status:                   row.Status,
		PrivacyScope:             row.PrivacyScope,
		Note:                     row.Note,
		Actor:                    safetyActorFromStore(row.ActorUserID, row.ActorRole, row.ActorSessionID, row.Capability, row.TrustedSurfaceKey, row.TrustedSurfaceLabel),
		OccurredAt:               row.OccurredAt.Time.UTC(),
		CreatedAt:                row.CreatedAt.Time.UTC(),
	}
}

func safetyBlockRecordFromStore(row store.ApolloCompetitionSafetyBlock) safetyBlockRecord {
	return safetyBlockRecord{
		ID:                   row.ID,
		CompetitionSessionID: row.CompetitionSessionID,
		CompetitionMatchID:   uuidFromPgtype(row.CompetitionMatchID),
		BlockerUserID:        row.BlockerUserID,
		BlockedUserID:        row.BlockedUserID,
		ReasonCode:           row.ReasonCode,
		Status:               row.Status,
		PrivacyScope:         row.PrivacyScope,
		Actor:                safetyActorFromStore(row.ActorUserID, row.ActorRole, row.ActorSessionID, row.Capability, row.TrustedSurfaceKey, row.TrustedSurfaceLabel),
		OccurredAt:           row.OccurredAt.Time.UTC(),
		CreatedAt:            row.CreatedAt.Time.UTC(),
	}
}

func reliabilityEventRecordFromStore(row store.ApolloCompetitionReliabilityEvent) reliabilityEventRecord {
	return reliabilityEventRecord{
		ID:                   row.ID,
		CompetitionSessionID: row.CompetitionSessionID,
		CompetitionMatchID:   uuidFromPgtype(row.CompetitionMatchID),
		SubjectUserID:        uuidFromPgtype(row.SubjectUserID),
		ReliabilityType:      row.ReliabilityType,
		Severity:             row.Severity,
		PrivacyScope:         row.PrivacyScope,
		Note:                 row.Note,
		Actor:                safetyActorFromStore(row.ActorUserID, row.ActorRole, row.ActorSessionID, row.Capability, row.TrustedSurfaceKey, row.TrustedSurfaceLabel),
		OccurredAt:           row.OccurredAt.Time.UTC(),
		CreatedAt:            row.CreatedAt.Time.UTC(),
	}
}

func safetyEventRecordFromStore(row store.ApolloCompetitionSafetyEvent) safetyEventRecord {
	return safetyEventRecord{
		ID:                       row.ID,
		EventType:                row.EventType,
		CompetitionSessionID:     uuidFromPgtype(row.CompetitionSessionID),
		CompetitionMatchID:       uuidFromPgtype(row.CompetitionMatchID),
		CompetitionSessionTeamID: uuidFromPgtype(row.CompetitionSessionTeamID),
		CompetitionTournamentID:  uuidFromPgtype(row.CompetitionTournamentID),
		SafetyReportID:           uuidFromPgtype(row.SafetyReportID),
		SafetyBlockID:            uuidFromPgtype(row.SafetyBlockID),
		ReliabilityEventID:       uuidFromPgtype(row.ReliabilityEventID),
		ReporterUserID:           uuidFromPgtype(row.ReporterUserID),
		SubjectUserID:            uuidFromPgtype(row.SubjectUserID),
		BlockerUserID:            uuidFromPgtype(row.BlockerUserID),
		BlockedUserID:            uuidFromPgtype(row.BlockedUserID),
		TargetType:               row.TargetType,
		TargetID:                 uuidFromPgtype(row.TargetID),
		ReasonCode:               row.ReasonCode,
		ReliabilityType:          row.ReliabilityType,
		Severity:                 row.Severity,
		PrivacyScope:             row.PrivacyScope,
		Actor:                    safetyActorFromStore(row.ActorUserID, row.ActorRole, row.ActorSessionID, row.Capability, row.TrustedSurfaceKey, row.TrustedSurfaceLabel),
		OccurredAt:               row.OccurredAt.Time.UTC(),
		CreatedAt:                row.CreatedAt.Time.UTC(),
	}
}
