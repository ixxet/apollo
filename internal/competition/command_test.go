package competition

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/authz"
)

func TestCompetitionReadinessExposesRoleCapabilities(t *testing.T) {
	svc := NewService(stubStore{})

	readiness := svc.CompetitionReadiness(StaffActor{Role: authz.RoleSupervisor})

	if readiness.Status != "ready" {
		t.Fatalf("readiness.Status = %q, want ready", readiness.Status)
	}
	assertCommandAvailable(t, readiness, CommandOpenQueue, true)
	assertCommandAvailable(t, readiness, CommandCreateTeam, false)

	resultDefinition, ok := commandCapability(readiness, CommandRecordMatchResult)
	if !ok {
		t.Fatalf("readiness missing %s", CommandRecordMatchResult)
	}
	if resultDefinition.ApplySupported {
		t.Fatal("record_match_result ApplySupported = true, want false for 3B.11")
	}
	if !resultDefinition.DryRunSupported {
		t.Fatal("record_match_result DryRunSupported = false, want true")
	}
}

func TestCompetitionCommandDryRunPlansWithoutMutation(t *testing.T) {
	sessionID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	svc := NewService(stubStore{
		getSessionByID: func(ctx context.Context, requested uuid.UUID) (*sessionRecord, error) {
			if requested != sessionID {
				t.Fatalf("GetSessionByID(%s), want %s", requested, sessionID)
			}
			return &sessionRecord{
				ID:           sessionID,
				QueueVersion: 3,
				Status:       SessionStatusDraft,
			}, nil
		},
		createTeam: func(context.Context, StaffActor, uuid.UUID, int, time.Time) (teamRecord, error) {
			t.Fatal("CreateTeam called during dry-run")
			return teamRecord{}, nil
		},
	})

	outcome, err := svc.ExecuteCommand(context.Background(), CompetitionCommand{
		Name:      CommandCreateTeam,
		DryRun:    true,
		SessionID: sessionID,
		Actor: CompetitionCommandActor{
			Role: authz.RoleManager,
		},
		CreateTeam: &CreateTeamInput{SideIndex: 1},
	})
	if err != nil {
		t.Fatalf("ExecuteCommand(dry-run) error = %v", err)
	}
	if outcome.Status != CommandStatusPlanned {
		t.Fatalf("outcome.Status = %q, want %q", outcome.Status, CommandStatusPlanned)
	}
	if outcome.Mutated {
		t.Fatal("outcome.Mutated = true, want false")
	}
	if outcome.ActualVersion == nil || *outcome.ActualVersion != 3 {
		t.Fatalf("outcome.ActualVersion = %v, want 3", outcome.ActualVersion)
	}
	if len(outcome.Plan) != 1 || outcome.Plan[0].Action != string(CommandCreateTeam) {
		t.Fatalf("outcome.Plan = %#v, want create_team plan", outcome.Plan)
	}
}

func TestCompetitionCommandDeniesMissingCapability(t *testing.T) {
	svc := NewService(stubStore{})

	outcome, err := svc.ExecuteCommand(context.Background(), CompetitionCommand{
		Name:   CommandCreateSession,
		DryRun: true,
		Actor: CompetitionCommandActor{
			Role: authz.RoleMember,
		},
		CreateSession: &CreateSessionInput{
			DisplayName:         "Member Attempt",
			SportKey:            "badminton",
			FacilityKey:         "ashtonbee",
			ParticipantsPerSide: 1,
		},
	})
	if !errors.Is(err, authz.ErrCapabilityDenied) {
		t.Fatalf("ExecuteCommand error = %v, want capability denial", err)
	}
	if outcome.Status != CommandStatusDenied {
		t.Fatalf("outcome.Status = %q, want %q", outcome.Status, CommandStatusDenied)
	}
	if outcome.Mutated {
		t.Fatal("outcome.Mutated = true, want false")
	}
}

func TestCompetitionCommandRecordResultApplyUnsupported(t *testing.T) {
	sessionID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	matchID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	teamOneID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	teamTwoID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	svc := NewService(stubStore{
		getSessionByID: func(context.Context, uuid.UUID) (*sessionRecord, error) {
			return &sessionRecord{ID: sessionID, QueueVersion: 4, Status: SessionStatusInProgress}, nil
		},
		recordMatchResult: func(context.Context, StaffActor, sessionRecord, SportConfig, matchRecord, RecordMatchResultInput, time.Time) error {
			t.Fatal("RecordMatchResult called through 3B.11 command apply")
			return nil
		},
	})

	outcome, err := svc.ExecuteCommand(context.Background(), CompetitionCommand{
		Name:      CommandRecordMatchResult,
		SessionID: sessionID,
		MatchID:   matchID,
		Actor: CompetitionCommandActor{
			UserID:            uuid.MustParse("55555555-5555-5555-5555-555555555555"),
			Role:              authz.RoleManager,
			SessionID:         uuid.MustParse("66666666-6666-6666-6666-666666666666"),
			TrustedSurfaceKey: "staff-console",
		},
		MatchResult: &RecordMatchResultInput{Sides: []MatchResultSideInput{
			{SideIndex: 1, CompetitionSessionTeamID: teamOneID, Outcome: matchOutcomeWin},
			{SideIndex: 2, CompetitionSessionTeamID: teamTwoID, Outcome: matchOutcomeLoss},
		}},
	})
	if !errors.Is(err, ErrCommandApplyUnsupported) {
		t.Fatalf("ExecuteCommand error = %v, want ErrCommandApplyUnsupported", err)
	}
	if outcome.Status != CommandStatusRejected {
		t.Fatalf("outcome.Status = %q, want %q", outcome.Status, CommandStatusRejected)
	}
	if outcome.Mutated {
		t.Fatal("outcome.Mutated = true, want false")
	}
}

func assertCommandAvailable(t *testing.T, readiness CompetitionCommandReadiness, name CommandName, want bool) {
	t.Helper()
	capability, ok := commandCapability(readiness, name)
	if !ok {
		t.Fatalf("readiness missing command %s", name)
	}
	if capability.Available != want {
		t.Fatalf("command %s available = %t, want %t", name, capability.Available, want)
	}
}

func commandCapability(readiness CompetitionCommandReadiness, name CommandName) (CompetitionCommandCapability, bool) {
	for _, capability := range readiness.Commands {
		if capability.Name == name {
			return capability, true
		}
	}
	return CompetitionCommandCapability{}, false
}
