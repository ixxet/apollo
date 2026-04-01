package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ixxet/apollo/internal/visits"
)

const SubjectIdentifiedPresenceArrived = "athena.identified_presence.arrived"

type ArrivalRecorder interface {
	RecordArrival(ctx context.Context, input visits.ArrivalInput) (visits.Result, error)
}

type IdentifiedPresenceHandler struct {
	service ArrivalRecorder
}

type envelope struct {
	ID        string          `json:"id"`
	Source    string          `json:"source"`
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

type arrivalData struct {
	FacilityID           *string `json:"facility_id"`
	ZoneID               *string `json:"zone_id"`
	ExternalIdentityHash *string `json:"external_identity_hash"`
	Source               *string `json:"source"`
	RecordedAt           *string `json:"recorded_at"`
}

func NewIdentifiedPresenceHandler(service ArrivalRecorder) *IdentifiedPresenceHandler {
	return &IdentifiedPresenceHandler{service: service}
}

func (h *IdentifiedPresenceHandler) HandleMessage(ctx context.Context, payload []byte) (visits.Result, error) {
	var event envelope
	if err := json.Unmarshal(payload, &event); err != nil {
		return visits.Result{}, fmt.Errorf("unmarshal envelope: %w", err)
	}

	if strings.TrimSpace(event.ID) == "" {
		return visits.Result{}, fmt.Errorf("identified presence event missing id")
	}
	if event.Source != "athena" {
		return visits.Result{}, fmt.Errorf("identified presence event has unexpected source %q", event.Source)
	}
	if event.Type != SubjectIdentifiedPresenceArrived {
		return visits.Result{}, fmt.Errorf("identified presence event has unexpected type %q", event.Type)
	}
	if strings.TrimSpace(event.Timestamp) == "" {
		return visits.Result{}, fmt.Errorf("identified presence event missing timestamp")
	}
	if _, err := parseTimestamp(event.Timestamp); err != nil {
		return visits.Result{}, fmt.Errorf("identified presence event timestamp: %w", err)
	}
	if len(event.Data) == 0 {
		return visits.Result{}, fmt.Errorf("identified presence event missing data")
	}

	var data arrivalData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return visits.Result{}, fmt.Errorf("unmarshal identified presence data: %w", err)
	}

	if data.FacilityID == nil || strings.TrimSpace(*data.FacilityID) == "" {
		return visits.Result{}, fmt.Errorf("identified presence event missing facility_id")
	}
	if data.RecordedAt == nil || strings.TrimSpace(*data.RecordedAt) == "" {
		return visits.Result{}, fmt.Errorf("identified presence event missing recorded_at")
	}
	recordedAt, err := parseTimestamp(*data.RecordedAt)
	if err != nil {
		return visits.Result{}, fmt.Errorf("identified presence event recorded_at: %w", err)
	}
	if data.Source == nil || strings.TrimSpace(*data.Source) == "" {
		return visits.Result{}, fmt.Errorf("identified presence event missing source")
	}

	identityHash := ""
	if data.ExternalIdentityHash != nil {
		identityHash = strings.TrimSpace(*data.ExternalIdentityHash)
	}
	if identityHash == "" {
		return visits.Result{Outcome: visits.OutcomeIgnoredAnonymous}, nil
	}

	var zoneKey *string
	if data.ZoneID != nil && strings.TrimSpace(*data.ZoneID) != "" {
		trimmed := strings.TrimSpace(*data.ZoneID)
		zoneKey = &trimmed
	}

	return h.service.RecordArrival(ctx, visits.ArrivalInput{
		SourceEventID:        event.ID,
		FacilityKey:          strings.TrimSpace(*data.FacilityID),
		ZoneKey:              zoneKey,
		ExternalIdentityHash: identityHash,
		ArrivedAt:            recordedAt,
	})
}

func parseTimestamp(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}
