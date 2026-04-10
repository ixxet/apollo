package consumer

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ixxet/apollo/internal/visits"
	protoevents "github.com/ixxet/ashton-proto/events"
)

type ArrivalRecorder interface {
	RecordArrival(ctx context.Context, input visits.ArrivalInput) (visits.Result, error)
}

type IdentifiedPresenceHandler struct {
	service ArrivalRecorder
}

func NewIdentifiedPresenceHandler(service ArrivalRecorder) *IdentifiedPresenceHandler {
	return &IdentifiedPresenceHandler{service: service}
}

func (h *IdentifiedPresenceHandler) HandleMessage(ctx context.Context, payload []byte) (visits.Result, error) {
	event, err := protoevents.ParseIdentifiedPresenceArrived(payload)
	if err != nil {
		slog.Warn("identified presence rejected", "error", err)
		return visits.Result{}, fmt.Errorf("parse identified arrival: %w", err)
	}

	identityHash := strings.TrimSpace(event.Data.GetExternalIdentityHash())
	if identityHash == "" {
		slog.Info("identified presence ignored", "event_id", event.ID, "outcome", visits.OutcomeIgnoredAnonymous, "reason", "anonymous")
		return visits.Result{Outcome: visits.OutcomeIgnoredAnonymous}, nil
	}

	var zoneKey *string
	if trimmed := strings.TrimSpace(event.Data.GetZoneId()); trimmed != "" {
		zoneKey = &trimmed
	}

	result, err := h.service.RecordArrival(ctx, visits.ArrivalInput{
		SourceEventID:        event.ID,
		FacilityKey:          strings.TrimSpace(event.Data.GetFacilityId()),
		ZoneKey:              zoneKey,
		ExternalIdentityHash: identityHash,
		ArrivedAt:            event.Data.GetRecordedAt().AsTime().UTC(),
	})
	if err != nil {
		slog.Error("identified presence visit record failed", "event_id", event.ID, "error", err)
		return visits.Result{}, fmt.Errorf("record identified arrival %q: %w", event.ID, err)
	}

	slog.Info("identified presence handled", "event_id", event.ID, "outcome", result.Outcome)
	return result, nil
}
