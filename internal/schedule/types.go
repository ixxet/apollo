package schedule

import (
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/authz"
)

const (
	ScopeFacility = "facility"
	ScopeZone     = "zone"
	ScopeResource = "resource"

	ScheduleTypeOneOff = "one_off"
	ScheduleTypeWeekly  = "weekly"

	KindOperatingHours = "operating_hours"
	KindClosure        = "closure"
	KindEvent          = "event"
	KindHold           = "hold"
	KindReservation    = "reservation"

	EffectInformational = "informational"
	EffectSoftHold      = "soft_hold"
	EffectHardReserve   = "hard_reserve"
	EffectClosed        = "closed"

	VisibilityInternal      = "internal"
	VisibilityPublicBusy    = "public_busy"
	VisibilityPublicLabeled = "public_labeled"

	StatusScheduled = "scheduled"
	StatusCancelled = "cancelled"

	EdgeContains      = "contains"
	EdgeComposes      = "composes"
	EdgeExclusiveWith = "exclusive_with"

	DefaultCalendarWindowDays = 90
)

type StaffActor struct {
	UserID              uuid.UUID
	Role                authz.Role
	SessionID           uuid.UUID
	Capability          authz.Capability
	TrustedSurfaceKey   string
	TrustedSurfaceLabel string
}

type Resource struct {
	ResourceKey  string     `json:"resource_key"`
	FacilityKey  string     `json:"facility_key"`
	ZoneKey      *string    `json:"zone_key,omitempty"`
	ResourceType string     `json:"resource_type"`
	DisplayName  string     `json:"display_name"`
	PublicLabel  *string    `json:"public_label,omitempty"`
	Bookable     bool       `json:"bookable"`
	Active       bool       `json:"active"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	Edges        []ResourceEdge `json:"edges,omitempty"`
}

type ResourceInput struct {
	ResourceKey  string  `json:"resource_key"`
	FacilityKey  string  `json:"facility_key"`
	ZoneKey      *string `json:"zone_key,omitempty"`
	ResourceType string  `json:"resource_type"`
	DisplayName  string  `json:"display_name"`
	PublicLabel  *string `json:"public_label,omitempty"`
	Bookable     bool    `json:"bookable"`
	Active       bool    `json:"active"`
}

type ResourceEdge struct {
	ResourceKey        string    `json:"resource_key"`
	RelatedResourceKey  string    `json:"related_resource_key"`
	EdgeType           string    `json:"edge_type"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type ResourceEdgeInput struct {
	ResourceKey       string `json:"resource_key"`
	RelatedResourceKey string `json:"related_resource_key"`
	EdgeType          string `json:"edge_type"`
}

type OneOffInput struct {
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
}

type WeeklyInput struct {
	Weekday             int     `json:"weekday"`
	StartTime           string  `json:"start_time"`
	EndTime             string  `json:"end_time"`
	Timezone            string  `json:"timezone"`
	RecurrenceStartDate string  `json:"recurrence_start_date"`
	RecurrenceEndDate   *string `json:"recurrence_end_date,omitempty"`
}

type BlockInput struct {
	FacilityKey string  `json:"facility_key"`
	ZoneKey     *string `json:"zone_key,omitempty"`
	ResourceKey *string `json:"resource_key,omitempty"`
	Scope       string  `json:"scope"`
	Kind        string  `json:"kind"`
	Effect      string  `json:"effect"`
	Visibility  string  `json:"visibility"`
	OneOff      *OneOffInput `json:"one_off,omitempty"`
	Weekly      *WeeklyInput  `json:"weekly,omitempty"`
}

type BlockExceptionInput struct {
	ExceptionDate string `json:"exception_date"`
}

type BlockCancellationInput struct {
	ExpectedVersion int `json:"expected_version"`
}

type CalendarWindow struct {
	From  time.Time
	Until time.Time
}

type Conflict struct {
	BlockID   uuid.UUID `json:"block_id"`
	Reason    string    `json:"reason"`
	Scope     string    `json:"scope"`
	Kind      string    `json:"kind"`
	Effect    string    `json:"effect"`
	Visibility string    `json:"visibility"`
}

type Block struct {
	ID                           uuid.UUID   `json:"id"`
	FacilityKey                  string      `json:"facility_key"`
	ZoneKey                      *string     `json:"zone_key,omitempty"`
	ResourceKey                  *string     `json:"resource_key,omitempty"`
	Scope                        string      `json:"scope"`
	ScheduleType                 string      `json:"schedule_type"`
	Kind                         string      `json:"kind"`
	Effect                       string      `json:"effect"`
	Visibility                   string      `json:"visibility"`
	Status                       string      `json:"status"`
	Version                      int         `json:"version"`
	Weekday                      *int        `json:"weekday,omitempty"`
	StartTime                    *string     `json:"start_time,omitempty"`
	EndTime                      *string     `json:"end_time,omitempty"`
	Timezone                     *string     `json:"timezone,omitempty"`
	RecurrenceStartDate          *string     `json:"recurrence_start_date,omitempty"`
	RecurrenceEndDate            *string     `json:"recurrence_end_date,omitempty"`
	StartAt                      *time.Time  `json:"start_at,omitempty"`
	EndAt                        *time.Time  `json:"end_at,omitempty"`
	CreatedByUserID              uuid.UUID   `json:"created_by_user_id"`
	CreatedBySessionID           uuid.UUID   `json:"created_by_session_id"`
	CreatedByRole                string      `json:"created_by_role"`
	CreatedByCapability          string      `json:"created_by_capability"`
	CreatedTrustedSurfaceKey     string      `json:"created_trusted_surface_key"`
	CreatedTrustedSurfaceLabel   *string     `json:"created_trusted_surface_label,omitempty"`
	UpdatedByUserID              uuid.UUID   `json:"updated_by_user_id"`
	UpdatedBySessionID           uuid.UUID   `json:"updated_by_session_id"`
	UpdatedByRole                string      `json:"updated_by_role"`
	UpdatedByCapability          string      `json:"updated_by_capability"`
	UpdatedTrustedSurfaceKey     string      `json:"updated_trusted_surface_key"`
	UpdatedTrustedSurfaceLabel   *string     `json:"updated_trusted_surface_label,omitempty"`
	CreatedAt                    time.Time   `json:"created_at"`
	UpdatedAt                    time.Time   `json:"updated_at"`
	CancelledAt                  *time.Time  `json:"cancelled_at,omitempty"`
	CancelledByUserID            *uuid.UUID  `json:"cancelled_by_user_id,omitempty"`
	CancelledBySessionID         *uuid.UUID  `json:"cancelled_by_session_id,omitempty"`
	CancelledByRole              *string     `json:"cancelled_by_role,omitempty"`
	CancelledByCapability        *string     `json:"cancelled_by_capability,omitempty"`
	CancelledTrustedSurfaceKey   *string     `json:"cancelled_trusted_surface_key,omitempty"`
	CancelledTrustedSurfaceLabel *string     `json:"cancelled_trusted_surface_label,omitempty"`
	Conflicts                    []Conflict  `json:"conflicts,omitempty"`
	Exceptions                   []string    `json:"exceptions,omitempty"`
}

type Occurrence struct {
	BlockID         uuid.UUID  `json:"block_id"`
	FacilityKey     string     `json:"facility_key"`
	ZoneKey         *string    `json:"zone_key,omitempty"`
	ResourceKey     *string    `json:"resource_key,omitempty"`
	Scope           string     `json:"scope"`
	Kind            string     `json:"kind"`
	Effect          string     `json:"effect"`
	Visibility      string     `json:"visibility"`
	Status          string     `json:"status"`
	StartsAt        time.Time  `json:"starts_at"`
	EndsAt          time.Time  `json:"ends_at"`
	OccurrenceDate  string     `json:"occurrence_date"`
	Conflicts       []Conflict `json:"conflicts,omitempty"`
}
