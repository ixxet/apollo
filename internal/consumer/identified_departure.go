package consumer

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ixxet/apollo/internal/visits"
	protoevents "github.com/ixxet/ashton-proto/events"
)

type DepartureRecorder interface {
	RecordDeparture(ctx context.Context, input visits.DepartureInput) (visits.Result, error)
}

type IdentifiedDepartureHandler struct {
	service DepartureRecorder
}

func NewIdentifiedDepartureHandler(service DepartureRecorder) *IdentifiedDepartureHandler {
	return &IdentifiedDepartureHandler{service: service}
}

func (h *IdentifiedDepartureHandler) HandleMessage(ctx context.Context, payload []byte) (visits.Result, error) {
	event, err := protoevents.ParseIdentifiedPresenceDeparted(payload)
	if err != nil {
		slog.Warn("identified departure rejected", "error", err)
		return visits.Result{}, fmt.Errorf("parse identified departure: %w", err)
	}

	identityHash := strings.TrimSpace(event.Data.GetExternalIdentityHash())
	if identityHash == "" {
		slog.Info("identified departure ignored", "event_id", event.ID, "outcome", visits.OutcomeIgnoredAnonymous, "reason", "anonymous")
		return visits.Result{Outcome: visits.OutcomeIgnoredAnonymous}, nil
	}

	result, err := h.service.RecordDeparture(ctx, visits.DepartureInput{
		SourceEventID:        event.ID,
		FacilityKey:          strings.TrimSpace(event.Data.GetFacilityId()),
		ExternalIdentityHash: identityHash,
		DepartedAt:           event.Data.GetRecordedAt().AsTime().UTC(),
	})
	if err != nil {
		slog.Error("identified departure visit close failed", "event_id", event.ID, "error", err)
		return visits.Result{}, fmt.Errorf("record identified departure %q: %w", event.ID, err)
	}

	slog.Info("identified departure handled", "event_id", event.ID, "outcome", result.Outcome)
	return result, nil
}
