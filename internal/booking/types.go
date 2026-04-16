package booking

import (
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/schedule"
)

const (
	StatusRequested    = "requested"
	StatusUnderReview  = "under_review"
	StatusNeedsChanges = "needs_changes"
	StatusApproved     = "approved"
	StatusRejected     = "rejected"
	StatusCancelled    = "cancelled"

	AvailabilityAvailable     = "available"
	AvailabilityConflict      = "conflict"
	AvailabilityReserved      = "reserved"
	AvailabilityNotApplicable = "not_applicable"
)

type StaffActor struct {
	UserID              uuid.UUID
	Role                authz.Role
	SessionID           uuid.UUID
	Capability          authz.Capability
	TrustedSurfaceKey   string
	TrustedSurfaceLabel string
}

type RequestInput struct {
	FacilityKey      string    `json:"facility_key"`
	ZoneKey          *string   `json:"zone_key,omitempty"`
	ResourceKey      *string   `json:"resource_key,omitempty"`
	RequestedStartAt time.Time `json:"requested_start_at"`
	RequestedEndAt   time.Time `json:"requested_end_at"`
	ContactName      string    `json:"contact_name"`
	ContactEmail     *string   `json:"contact_email,omitempty"`
	ContactPhone     *string   `json:"contact_phone,omitempty"`
	Organization     *string   `json:"organization,omitempty"`
	Purpose          *string   `json:"purpose,omitempty"`
	AttendeeCount    *int      `json:"attendee_count,omitempty"`
	InternalNotes    *string   `json:"internal_notes,omitempty"`
}

type TransitionInput struct {
	ExpectedVersion int     `json:"expected_version"`
	InternalNotes   *string `json:"internal_notes,omitempty"`
}

type AvailabilityDecision struct {
	Status    string              `json:"status"`
	Available bool                `json:"available"`
	Conflicts []schedule.Conflict `json:"conflicts,omitempty"`
	Reason    *string             `json:"reason,omitempty"`
}

type Request struct {
	ID                         uuid.UUID            `json:"id"`
	FacilityKey                string               `json:"facility_key"`
	ZoneKey                    *string              `json:"zone_key,omitempty"`
	ResourceKey                *string              `json:"resource_key,omitempty"`
	Scope                      string               `json:"scope"`
	RequestedStartAt           time.Time            `json:"requested_start_at"`
	RequestedEndAt             time.Time            `json:"requested_end_at"`
	ContactName                string               `json:"contact_name"`
	ContactEmail               *string              `json:"contact_email,omitempty"`
	ContactPhone               *string              `json:"contact_phone,omitempty"`
	Organization               *string              `json:"organization,omitempty"`
	Purpose                    *string              `json:"purpose,omitempty"`
	AttendeeCount              *int                 `json:"attendee_count,omitempty"`
	InternalNotes              *string              `json:"internal_notes,omitempty"`
	Status                     string               `json:"status"`
	Version                    int                  `json:"version"`
	ScheduleBlockID            *uuid.UUID           `json:"schedule_block_id,omitempty"`
	Availability               AvailabilityDecision `json:"availability"`
	CreatedByUserID            uuid.UUID            `json:"created_by_user_id"`
	CreatedBySessionID         uuid.UUID            `json:"created_by_session_id"`
	CreatedByRole              string               `json:"created_by_role"`
	CreatedByCapability        string               `json:"created_by_capability"`
	CreatedTrustedSurfaceKey   string               `json:"created_trusted_surface_key"`
	CreatedTrustedSurfaceLabel *string              `json:"created_trusted_surface_label,omitempty"`
	UpdatedByUserID            uuid.UUID            `json:"updated_by_user_id"`
	UpdatedBySessionID         uuid.UUID            `json:"updated_by_session_id"`
	UpdatedByRole              string               `json:"updated_by_role"`
	UpdatedByCapability        string               `json:"updated_by_capability"`
	UpdatedTrustedSurfaceKey   string               `json:"updated_trusted_surface_key"`
	UpdatedTrustedSurfaceLabel *string              `json:"updated_trusted_surface_label,omitempty"`
	CreatedAt                  time.Time            `json:"created_at"`
	UpdatedAt                  time.Time            `json:"updated_at"`
}
