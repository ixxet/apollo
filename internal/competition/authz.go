package competition

import (
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/authz"
)

type StaffActor struct {
	UserID              uuid.UUID
	Role                authz.Role
	SessionID           uuid.UUID
	Capability          authz.Capability
	TrustedSurfaceKey   string
	TrustedSurfaceLabel string
}

type staffActionAttribution struct {
	Actor                StaffActor
	Action               string
	CompetitionSessionID *uuid.UUID
	CompetitionTeamID    *uuid.UUID
	CompetitionMatchID   *uuid.UUID
	SubjectUserID        *uuid.UUID
	OccurredAt           time.Time
}

func newStaffActionAttribution(actor StaffActor, action string, occurredAt time.Time) staffActionAttribution {
	return staffActionAttribution{
		Actor:      actor,
		Action:     action,
		OccurredAt: occurredAt.UTC(),
	}
}

func uuidPtr(value uuid.UUID) *uuid.UUID {
	if value == uuid.Nil {
		return nil
	}

	clone := value
	return &clone
}
