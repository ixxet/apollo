package competition

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/ares"
	"github.com/ixxet/apollo/internal/authz"
)

type CommandName string

const (
	CommandCreateSession        CommandName = "create_session"
	CommandOpenQueue            CommandName = "open_queue"
	CommandAddQueueMember       CommandName = "add_queue_member"
	CommandRemoveQueueMember    CommandName = "remove_queue_member"
	CommandUpdateQueueIntent    CommandName = "update_queue_intent"
	CommandGenerateMatchPreview CommandName = "generate_match_preview"
	CommandAssignQueue          CommandName = "assign_queue"
	CommandStartSession         CommandName = "start_session"
	CommandArchiveSession       CommandName = "archive_session"
	CommandCreateTeam           CommandName = "create_team"
	CommandRemoveTeam           CommandName = "remove_team"
	CommandAddRosterMember      CommandName = "add_roster_member"
	CommandRemoveRosterMember   CommandName = "remove_roster_member"
	CommandCreateMatch          CommandName = "create_match"
	CommandArchiveMatch         CommandName = "archive_match"
	CommandRecordMatchResult    CommandName = "record_match_result"
	CommandFinalizeMatchResult  CommandName = "finalize_match_result"
	CommandDisputeMatchResult   CommandName = "dispute_match_result"
	CommandCorrectMatchResult   CommandName = "correct_match_result"
	CommandVoidMatchResult      CommandName = "void_match_result"
)

const (
	CommandStatusPlanned   = "planned"
	CommandStatusSucceeded = "succeeded"
	CommandStatusDenied    = "denied"
	CommandStatusRejected  = "rejected"
	CommandStatusFailed    = "failed"
)

var (
	ErrCommandNameRequired       = errors.New("competition command name is required")
	ErrCommandUnsupported        = errors.New("competition command is unsupported")
	ErrCommandSessionIDRequired  = errors.New("competition command session_id is required")
	ErrCommandTeamIDRequired     = errors.New("competition command team_id is required")
	ErrCommandMatchIDRequired    = errors.New("competition command match_id is required")
	ErrCommandUserIDRequired     = errors.New("competition command user_id is required")
	ErrCommandActorRequired      = errors.New("competition command actor is required")
	ErrCommandActorSession       = errors.New("competition command actor_session_id is required")
	ErrCommandTrustedSurface     = errors.New("competition command trusted surface is required")
	ErrCommandApplyUnsupported   = errors.New("competition command apply is unsupported")
	ErrCommandExpectedVersion    = errors.New("competition command expected_version is required")
	ErrCommandCreateSessionInput = errors.New("competition command create_session input is required")
	ErrCommandCreateTeamInput    = errors.New("competition command create_team input is required")
	ErrCommandRosterInput        = errors.New("competition command roster input is required")
	ErrCommandQueueMemberInput   = errors.New("competition command queue member input is required")
	ErrCommandCreateMatchInput   = errors.New("competition command create_match input is required")
	ErrCommandMatchResultInput   = errors.New("competition command record_match_result input is required")
)

type CompetitionCommand struct {
	Name            CommandName             `json:"name"`
	DryRun          bool                    `json:"dry_run"`
	IdempotencyKey  string                  `json:"idempotency_key,omitempty"`
	ExpectedVersion *int                    `json:"expected_version,omitempty"`
	Actor           CompetitionCommandActor `json:"actor,omitempty"`
	SessionID       uuid.UUID               `json:"session_id,omitempty"`
	TeamID          uuid.UUID               `json:"team_id,omitempty"`
	MatchID         uuid.UUID               `json:"match_id,omitempty"`
	UserID          uuid.UUID               `json:"user_id,omitempty"`
	CreateSession   *CreateSessionInput     `json:"create_session,omitempty"`
	CreateTeam      *CreateTeamInput        `json:"create_team,omitempty"`
	QueueMember     *QueueMemberInput       `json:"queue_member,omitempty"`
	RosterMember    *AddRosterMemberInput   `json:"roster_member,omitempty"`
	CreateMatch     *CreateMatchInput       `json:"create_match,omitempty"`
	MatchResult     *RecordMatchResultInput `json:"match_result,omitempty"`
}

type CompetitionCommandActor struct {
	UserID              uuid.UUID        `json:"user_id,omitempty"`
	Role                authz.Role       `json:"role"`
	SessionID           uuid.UUID        `json:"session_id,omitempty"`
	Capability          authz.Capability `json:"capability,omitempty"`
	TrustedSurfaceKey   string           `json:"trusted_surface_key,omitempty"`
	TrustedSurfaceLabel string           `json:"trusted_surface_label,omitempty"`
}

type CompetitionCommandDefinition struct {
	Name                   CommandName      `json:"name"`
	RequiredCapability     authz.Capability `json:"required_capability"`
	TrustedSurfaceRequired bool             `json:"trusted_surface_required"`
	Mutating               bool             `json:"mutating"`
	DryRunSupported        bool             `json:"dry_run_supported"`
	ApplySupported         bool             `json:"apply_supported"`
	IdempotencySupported   bool             `json:"idempotency_supported"`
	VersionField           string           `json:"version_field,omitempty"`
	Description            string           `json:"description"`
}

type CompetitionCommandReadiness struct {
	Status       string                         `json:"status"`
	Message      string                         `json:"message"`
	Actor        CompetitionCommandActor        `json:"actor"`
	Capabilities []authz.Capability             `json:"capabilities"`
	Commands     []CompetitionCommandCapability `json:"commands"`
}

type CompetitionCommandCapability struct {
	CompetitionCommandDefinition
	Available         bool   `json:"available"`
	UnavailableReason string `json:"unavailable_reason,omitempty"`
}

type CompetitionCommandOutcome struct {
	Name                 CommandName             `json:"name"`
	Status               string                  `json:"status"`
	DryRun               bool                    `json:"dry_run"`
	Mutated              bool                    `json:"mutated"`
	Message              string                  `json:"message"`
	Actor                CompetitionCommandActor `json:"actor"`
	RequiredCapability   authz.Capability        `json:"required_capability"`
	IdempotencyKey       string                  `json:"idempotency_key,omitempty"`
	IdempotencySupported bool                    `json:"idempotency_supported"`
	ExpectedVersion      *int                    `json:"expected_version,omitempty"`
	ActualVersion        *int                    `json:"actual_version,omitempty"`
	Plan                 []CompetitionPlanStep   `json:"plan,omitempty"`
	Resource             *CompetitionResourceRef `json:"resource,omitempty"`
	Result               any                     `json:"result,omitempty"`
	Error                string                  `json:"error,omitempty"`
}

type CompetitionPlanStep struct {
	Action       string `json:"action"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id,omitempty"`
	Description  string `json:"description"`
}

type CompetitionResourceRef struct {
	Type string    `json:"type"`
	ID   uuid.UUID `json:"id,omitempty"`
}

var commandDefinitions = []CompetitionCommandDefinition{
	{Name: CommandCreateSession, RequiredCapability: authz.CapabilityCompetitionStructureManage, TrustedSurfaceRequired: true, Mutating: true, DryRunSupported: true, ApplySupported: true, Description: "Create a competition session container."},
	{Name: CommandOpenQueue, RequiredCapability: authz.CapabilityCompetitionLiveManage, TrustedSurfaceRequired: true, Mutating: true, DryRunSupported: true, ApplySupported: true, Description: "Open a draft session queue."},
	{Name: CommandAddQueueMember, RequiredCapability: authz.CapabilityCompetitionLiveManage, TrustedSurfaceRequired: true, Mutating: true, DryRunSupported: true, ApplySupported: true, VersionField: "queue_version", Description: "Add an eligible member to an open queue."},
	{Name: CommandRemoveQueueMember, RequiredCapability: authz.CapabilityCompetitionLiveManage, TrustedSurfaceRequired: true, Mutating: true, DryRunSupported: true, ApplySupported: true, VersionField: "queue_version", Description: "Remove a member from an open queue."},
	{Name: CommandUpdateQueueIntent, RequiredCapability: authz.CapabilityCompetitionLiveManage, TrustedSurfaceRequired: true, Mutating: true, DryRunSupported: true, ApplySupported: true, VersionField: "queue_version", Description: "Update explicit sport, facility, mode, and tier queue intent facts for a queued member."},
	{Name: CommandGenerateMatchPreview, RequiredCapability: authz.CapabilityCompetitionLiveManage, TrustedSurfaceRequired: true, Mutating: true, DryRunSupported: true, ApplySupported: true, VersionField: "queue_version", Description: "Generate an internal ARES v2 match preview proposal from trusted queue and rating facts."},
	{Name: CommandAssignQueue, RequiredCapability: authz.CapabilityCompetitionLiveManage, TrustedSurfaceRequired: true, Mutating: true, DryRunSupported: true, ApplySupported: true, VersionField: "queue_version", Description: "Assign a ready queue into teams and matches."},
	{Name: CommandStartSession, RequiredCapability: authz.CapabilityCompetitionLiveManage, TrustedSurfaceRequired: true, Mutating: true, DryRunSupported: true, ApplySupported: true, Description: "Start an assigned competition session."},
	{Name: CommandArchiveSession, RequiredCapability: authz.CapabilityCompetitionLiveManage, TrustedSurfaceRequired: true, Mutating: true, DryRunSupported: true, ApplySupported: true, Description: "Archive an eligible competition session."},
	{Name: CommandCreateTeam, RequiredCapability: authz.CapabilityCompetitionStructureManage, TrustedSurfaceRequired: true, Mutating: true, DryRunSupported: true, ApplySupported: true, Description: "Create a draft session team."},
	{Name: CommandRemoveTeam, RequiredCapability: authz.CapabilityCompetitionStructureManage, TrustedSurfaceRequired: true, Mutating: true, DryRunSupported: true, ApplySupported: true, Description: "Remove an unreferenced draft session team."},
	{Name: CommandAddRosterMember, RequiredCapability: authz.CapabilityCompetitionStructureManage, TrustedSurfaceRequired: true, Mutating: true, DryRunSupported: true, ApplySupported: true, Description: "Add a member to a draft team roster."},
	{Name: CommandRemoveRosterMember, RequiredCapability: authz.CapabilityCompetitionStructureManage, TrustedSurfaceRequired: true, Mutating: true, DryRunSupported: true, ApplySupported: true, Description: "Remove a member from a draft team roster."},
	{Name: CommandCreateMatch, RequiredCapability: authz.CapabilityCompetitionStructureManage, TrustedSurfaceRequired: true, Mutating: true, DryRunSupported: true, ApplySupported: true, Description: "Create a draft match and side-slot references."},
	{Name: CommandArchiveMatch, RequiredCapability: authz.CapabilityCompetitionStructureManage, TrustedSurfaceRequired: true, Mutating: true, DryRunSupported: true, ApplySupported: true, Description: "Archive an eligible draft match."},
	{Name: CommandRecordMatchResult, RequiredCapability: authz.CapabilityCompetitionLiveManage, TrustedSurfaceRequired: true, Mutating: true, DryRunSupported: true, ApplySupported: true, VersionField: "result_version", Description: "Record a match result as non-final canonical result truth."},
	{Name: CommandFinalizeMatchResult, RequiredCapability: authz.CapabilityCompetitionLiveManage, TrustedSurfaceRequired: true, Mutating: true, DryRunSupported: true, ApplySupported: true, VersionField: "result_version", Description: "Finalize a recorded match result so ratings may consume it."},
	{Name: CommandDisputeMatchResult, RequiredCapability: authz.CapabilityCompetitionLiveManage, TrustedSurfaceRequired: true, Mutating: true, DryRunSupported: true, ApplySupported: true, VersionField: "result_version", Description: "Mark the canonical match result as disputed."},
	{Name: CommandCorrectMatchResult, RequiredCapability: authz.CapabilityCompetitionLiveManage, TrustedSurfaceRequired: true, Mutating: true, DryRunSupported: true, ApplySupported: true, VersionField: "result_version", Description: "Create a corrected canonical result that supersedes the previous result."},
	{Name: CommandVoidMatchResult, RequiredCapability: authz.CapabilityCompetitionLiveManage, TrustedSurfaceRequired: true, Mutating: true, DryRunSupported: true, ApplySupported: true, VersionField: "result_version", Description: "Void the canonical match result so ratings cannot consume it."},
}

func CompetitionCommandDefinitions() []CompetitionCommandDefinition {
	definitions := append([]CompetitionCommandDefinition(nil), commandDefinitions...)
	return definitions
}

func CompetitionCommandDefinitionFor(name CommandName) (CompetitionCommandDefinition, bool) {
	normalized := CommandName(strings.TrimSpace(string(name)))
	for _, definition := range commandDefinitions {
		if definition.Name == normalized {
			return definition, true
		}
	}
	return CompetitionCommandDefinition{}, false
}

func (s *Service) CompetitionReadiness(actor StaffActor) CompetitionCommandReadiness {
	commandActor := commandActorFromStaffActor(actor)
	capabilities := authz.CapabilitiesForRole(actor.Role)
	status := "ready"
	message := "Competition command contracts are available for this actor."
	if len(capabilities) == 0 {
		status = "unsupported_role"
		message = "This actor has no APOLLO competition capabilities."
	}

	commands := make([]CompetitionCommandCapability, 0, len(commandDefinitions))
	for _, definition := range commandDefinitions {
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

	return CompetitionCommandReadiness{
		Status:       status,
		Message:      message,
		Actor:        commandActor,
		Capabilities: capabilities,
		Commands:     commands,
	}
}

func (s *Service) ExecuteCommand(ctx context.Context, command CompetitionCommand) (CompetitionCommandOutcome, error) {
	command.Name = CommandName(strings.TrimSpace(string(command.Name)))
	definition, ok := CompetitionCommandDefinitionFor(command.Name)
	outcome := CompetitionCommandOutcome{
		Name:            command.Name,
		DryRun:          command.DryRun,
		Actor:           command.Actor,
		IdempotencyKey:  strings.TrimSpace(command.IdempotencyKey),
		ExpectedVersion: command.ExpectedVersion,
	}
	if ok {
		outcome.RequiredCapability = definition.RequiredCapability
		outcome.IdempotencySupported = definition.IdempotencySupported
	}

	if !ok {
		outcome.Status = CommandStatusRejected
		outcome.Message = "Competition command is not supported."
		outcome.Error = ErrCommandUnsupported.Error()
		return outcome, ErrCommandUnsupported
	}
	command.Actor.Capability = definition.RequiredCapability
	outcome.Actor = command.Actor

	if err := validateCommandActor(command.Actor, command.DryRun); err != nil {
		outcome.Status = CommandStatusDenied
		outcome.Message = "Competition command actor is incomplete."
		outcome.Error = err.Error()
		return outcome, err
	}
	if !authz.HasCapability(authz.CapabilitiesForRole(command.Actor.Role), definition.RequiredCapability) {
		outcome.Status = CommandStatusDenied
		outcome.Message = "Competition command capability is denied."
		outcome.Error = authz.ErrCapabilityDenied.Error()
		return outcome, authz.ErrCapabilityDenied
	}
	if definition.TrustedSurfaceRequired && !command.DryRun && strings.TrimSpace(command.Actor.TrustedSurfaceKey) == "" {
		outcome.Status = CommandStatusDenied
		outcome.Message = "Competition command requires trusted-surface proof."
		outcome.Error = ErrCommandTrustedSurface.Error()
		return outcome, ErrCommandTrustedSurface
	}
	if err := validateCompetitionCommand(command, definition); err != nil {
		outcome.Status = CommandStatusRejected
		outcome.Message = "Competition command payload is invalid."
		outcome.Error = err.Error()
		return outcome, err
	}

	actualVersion, err := s.commandActualVersion(ctx, command)
	if err != nil {
		outcome.Status = CommandStatusRejected
		outcome.Message = "Competition command resource is unavailable."
		outcome.Error = err.Error()
		return outcome, err
	}
	outcome.ActualVersion = actualVersion
	outcome.Resource = commandResource(command)

	if command.DryRun {
		outcome.Status = CommandStatusPlanned
		outcome.Message = "Dry run only; no competition mutation was applied."
		outcome.Plan = commandPlan(command, definition)
		return outcome, nil
	}
	if !definition.ApplySupported {
		outcome.Status = CommandStatusRejected
		outcome.Message = "Competition command apply is not supported in this packet; use dry_run for the plan shape."
		outcome.Error = ErrCommandApplyUnsupported.Error()
		return outcome, ErrCommandApplyUnsupported
	}

	result, err := s.applyCompetitionCommand(ctx, command)
	if err != nil {
		outcome.Status = CommandStatusRejected
		outcome.Message = "Competition command was rejected by APOLLO runtime truth."
		outcome.Error = err.Error()
		return outcome, err
	}

	outcome.Status = CommandStatusSucceeded
	outcome.Message = "Competition command applied."
	outcome.Mutated = definition.Mutating
	outcome.Result = result
	if session, ok := result.(Session); ok {
		if command.MatchID != uuid.Nil {
			for _, match := range session.Matches {
				if match.ID == command.MatchID {
					version := match.ResultVersion
					outcome.ActualVersion = &version
					return outcome, nil
				}
			}
		}
		version := session.QueueVersion
		outcome.ActualVersion = &version
	}
	if preview, ok := result.(ares.CompetitionMatchPreview); ok {
		version := preview.QueueVersion
		outcome.ActualVersion = &version
	}
	return outcome, nil
}

func validateCommandActor(actor CompetitionCommandActor, dryRun bool) error {
	if actor.Role == "" {
		return ErrCommandActorRequired
	}
	if _, err := authz.NormalizeRole(string(actor.Role)); err != nil {
		return err
	}
	if dryRun {
		return nil
	}
	if actor.UserID == uuid.Nil {
		return ErrCommandActorRequired
	}
	if actor.SessionID == uuid.Nil {
		return ErrCommandActorSession
	}
	return nil
}

func validateCompetitionCommand(command CompetitionCommand, definition CompetitionCommandDefinition) error {
	if definition.Name == "" {
		return ErrCommandUnsupported
	}

	switch command.Name {
	case CommandCreateSession:
		if command.CreateSession == nil {
			return ErrCommandCreateSessionInput
		}
	case CommandOpenQueue, CommandStartSession, CommandArchiveSession:
		return requireSessionID(command)
	case CommandAddQueueMember:
		if err := requireSessionID(command); err != nil {
			return err
		}
		if command.QueueMember == nil {
			return ErrCommandQueueMemberInput
		}
		if command.QueueMember.UserID == uuid.Nil {
			return ErrCommandUserIDRequired
		}
	case CommandRemoveQueueMember:
		if err := requireSessionID(command); err != nil {
			return err
		}
		if command.UserID == uuid.Nil {
			return ErrCommandUserIDRequired
		}
	case CommandUpdateQueueIntent:
		if err := requireSessionID(command); err != nil {
			return err
		}
		if command.QueueMember == nil {
			return ErrCommandQueueMemberInput
		}
		if command.QueueMember.UserID == uuid.Nil {
			return ErrCommandUserIDRequired
		}
		if command.ExpectedVersion == nil || *command.ExpectedVersion <= 0 {
			return ErrCommandExpectedVersion
		}
	case CommandGenerateMatchPreview:
		if err := requireSessionID(command); err != nil {
			return err
		}
		if command.ExpectedVersion == nil || *command.ExpectedVersion <= 0 {
			return ErrCommandExpectedVersion
		}
	case CommandAssignQueue:
		if err := requireSessionID(command); err != nil {
			return err
		}
		if command.ExpectedVersion == nil || *command.ExpectedVersion <= 0 {
			return ErrCommandExpectedVersion
		}
	case CommandCreateTeam:
		if err := requireSessionID(command); err != nil {
			return err
		}
		if command.CreateTeam == nil {
			return ErrCommandCreateTeamInput
		}
	case CommandRemoveTeam:
		if err := requireSessionID(command); err != nil {
			return err
		}
		if command.TeamID == uuid.Nil {
			return ErrCommandTeamIDRequired
		}
	case CommandAddRosterMember:
		if err := requireSessionID(command); err != nil {
			return err
		}
		if command.TeamID == uuid.Nil {
			return ErrCommandTeamIDRequired
		}
		if command.RosterMember == nil {
			return ErrCommandRosterInput
		}
		if command.RosterMember.UserID == uuid.Nil {
			return ErrCommandUserIDRequired
		}
	case CommandRemoveRosterMember:
		if err := requireSessionID(command); err != nil {
			return err
		}
		if command.TeamID == uuid.Nil {
			return ErrCommandTeamIDRequired
		}
		if command.UserID == uuid.Nil {
			return ErrCommandUserIDRequired
		}
	case CommandCreateMatch:
		if err := requireSessionID(command); err != nil {
			return err
		}
		if command.CreateMatch == nil {
			return ErrCommandCreateMatchInput
		}
	case CommandArchiveMatch:
		if err := requireSessionID(command); err != nil {
			return err
		}
		if command.MatchID == uuid.Nil {
			return ErrCommandMatchIDRequired
		}
	case CommandRecordMatchResult, CommandCorrectMatchResult:
		if err := requireSessionID(command); err != nil {
			return err
		}
		if command.MatchID == uuid.Nil {
			return ErrCommandMatchIDRequired
		}
		if command.ExpectedVersion == nil || *command.ExpectedVersion < 0 {
			return ErrCommandExpectedVersion
		}
		if command.MatchResult == nil {
			return ErrCommandMatchResultInput
		}
		command.MatchResult.ExpectedResultVersion = *command.ExpectedVersion
	case CommandFinalizeMatchResult, CommandDisputeMatchResult, CommandVoidMatchResult:
		if err := requireSessionID(command); err != nil {
			return err
		}
		if command.MatchID == uuid.Nil {
			return ErrCommandMatchIDRequired
		}
		if command.ExpectedVersion == nil || *command.ExpectedVersion < 0 {
			return ErrCommandExpectedVersion
		}
	default:
		return ErrCommandUnsupported
	}
	return nil
}

func requireSessionID(command CompetitionCommand) error {
	if command.SessionID == uuid.Nil {
		return ErrCommandSessionIDRequired
	}
	return nil
}

func (s *Service) commandActualVersion(ctx context.Context, command CompetitionCommand) (*int, error) {
	if command.MatchID != uuid.Nil {
		match, err := s.repository.GetMatchByID(ctx, command.MatchID)
		if err != nil {
			return nil, err
		}
		if match == nil {
			return nil, ErrMatchNotFound
		}
		version := match.ResultVersion
		return &version, nil
	}
	if command.SessionID == uuid.Nil {
		return nil, nil
	}
	session, err := s.repository.GetSessionByID(ctx, command.SessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrSessionNotFound
	}
	version := session.QueueVersion
	return &version, nil
}

func commandResource(command CompetitionCommand) *CompetitionResourceRef {
	switch {
	case command.MatchID != uuid.Nil:
		return &CompetitionResourceRef{Type: "competition_match", ID: command.MatchID}
	case command.TeamID != uuid.Nil:
		return &CompetitionResourceRef{Type: "competition_team", ID: command.TeamID}
	case command.SessionID != uuid.Nil:
		return &CompetitionResourceRef{Type: "competition_session", ID: command.SessionID}
	default:
		return nil
	}
}

func commandPlan(command CompetitionCommand, definition CompetitionCommandDefinition) []CompetitionPlanStep {
	resource := commandResource(command)
	step := CompetitionPlanStep{
		Action:       string(definition.Name),
		ResourceType: "competition",
		Description:  definition.Description,
	}
	if resource != nil {
		step.ResourceType = resource.Type
		if resource.ID != uuid.Nil {
			step.ResourceID = resource.ID.String()
		}
	}
	return []CompetitionPlanStep{step}
}

func (s *Service) applyCompetitionCommand(ctx context.Context, command CompetitionCommand) (any, error) {
	actor := staffActorFromCommandActor(command.Actor)
	switch command.Name {
	case CommandCreateSession:
		return s.CreateSession(ctx, actor, *command.CreateSession)
	case CommandOpenQueue:
		return s.OpenQueue(ctx, actor, command.SessionID)
	case CommandAddQueueMember:
		return s.AddQueueMember(ctx, actor, command.SessionID, *command.QueueMember)
	case CommandRemoveQueueMember:
		return s.RemoveQueueMember(ctx, actor, command.SessionID, command.UserID)
	case CommandUpdateQueueIntent:
		return s.UpdateQueueIntent(ctx, actor, command.SessionID, UpdateQueueIntentInput{
			UserID:               command.QueueMember.UserID,
			Tier:                 command.QueueMember.Tier,
			ExpectedQueueVersion: *command.ExpectedVersion,
		})
	case CommandGenerateMatchPreview:
		return s.GenerateMatchPreview(ctx, actor, command.SessionID, MatchPreviewInput{ExpectedQueueVersion: *command.ExpectedVersion})
	case CommandAssignQueue:
		return s.AssignQueue(ctx, actor, command.SessionID, AssignSessionInput{ExpectedQueueVersion: *command.ExpectedVersion})
	case CommandStartSession:
		return s.StartSession(ctx, actor, command.SessionID)
	case CommandArchiveSession:
		return s.ArchiveSession(ctx, actor, command.SessionID)
	case CommandCreateTeam:
		return s.CreateTeam(ctx, actor, command.SessionID, *command.CreateTeam)
	case CommandRemoveTeam:
		return nil, s.RemoveTeam(ctx, actor, command.SessionID, command.TeamID)
	case CommandAddRosterMember:
		return s.AddRosterMember(ctx, actor, command.SessionID, command.TeamID, *command.RosterMember)
	case CommandRemoveRosterMember:
		return nil, s.RemoveRosterMember(ctx, actor, command.SessionID, command.TeamID, command.UserID)
	case CommandCreateMatch:
		return s.CreateMatch(ctx, actor, command.SessionID, *command.CreateMatch)
	case CommandArchiveMatch:
		return s.ArchiveMatch(ctx, actor, command.SessionID, command.MatchID)
	case CommandRecordMatchResult:
		input := *command.MatchResult
		input.ExpectedResultVersion = *command.ExpectedVersion
		return s.RecordMatchResult(ctx, actor, command.SessionID, command.MatchID, input)
	case CommandFinalizeMatchResult:
		return s.FinalizeMatchResult(ctx, actor, command.SessionID, command.MatchID, *command.ExpectedVersion)
	case CommandDisputeMatchResult:
		return s.DisputeMatchResult(ctx, actor, command.SessionID, command.MatchID, *command.ExpectedVersion)
	case CommandCorrectMatchResult:
		input := *command.MatchResult
		input.ExpectedResultVersion = *command.ExpectedVersion
		return s.CorrectMatchResult(ctx, actor, command.SessionID, command.MatchID, input)
	case CommandVoidMatchResult:
		return s.VoidMatchResult(ctx, actor, command.SessionID, command.MatchID, *command.ExpectedVersion)
	default:
		return nil, fmt.Errorf("%w: %s", ErrCommandUnsupported, command.Name)
	}
}

func commandActorFromStaffActor(actor StaffActor) CompetitionCommandActor {
	return CompetitionCommandActor{
		UserID:              actor.UserID,
		Role:                actor.Role,
		SessionID:           actor.SessionID,
		Capability:          actor.Capability,
		TrustedSurfaceKey:   actor.TrustedSurfaceKey,
		TrustedSurfaceLabel: actor.TrustedSurfaceLabel,
	}
}

func staffActorFromCommandActor(actor CompetitionCommandActor) StaffActor {
	return StaffActor{
		UserID:              actor.UserID,
		Role:                actor.Role,
		SessionID:           actor.SessionID,
		Capability:          actor.Capability,
		TrustedSurfaceKey:   actor.TrustedSurfaceKey,
		TrustedSurfaceLabel: actor.TrustedSurfaceLabel,
	}
}
