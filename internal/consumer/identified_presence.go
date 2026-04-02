package consumer

import (
	"context"
	"encoding/json"
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
	if anonymous, err := isAnonymousIdentifiedPresence(payload); err != nil {
		return visits.Result{}, err
	} else if anonymous {
		return visits.Result{Outcome: visits.OutcomeIgnoredAnonymous}, nil
	}

	event, err := protoevents.ParseIdentifiedPresenceArrived(payload)
	if err != nil {
		return visits.Result{}, err
	}

	identityHash := strings.TrimSpace(event.Data.GetExternalIdentityHash())
	if identityHash == "" {
		return visits.Result{Outcome: visits.OutcomeIgnoredAnonymous}, nil
	}

	var zoneKey *string
	if trimmed := strings.TrimSpace(event.Data.GetZoneId()); trimmed != "" {
		zoneKey = &trimmed
	}

	return h.service.RecordArrival(ctx, visits.ArrivalInput{
		SourceEventID:        event.ID,
		FacilityKey:          strings.TrimSpace(event.Data.GetFacilityId()),
		ZoneKey:              zoneKey,
		ExternalIdentityHash: identityHash,
		ArrivedAt:            event.Data.GetRecordedAt().AsTime().UTC(),
	})
}

func isAnonymousIdentifiedPresence(payload []byte) (bool, error) {
	var envelope map[string]any
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return false, err
	}

	if strings.TrimSpace(stringValue(envelope["source"])) != protoevents.ServiceAthena {
		return false, nil
	}
	if strings.TrimSpace(stringValue(envelope["type"])) != protoevents.SubjectIdentifiedPresenceArrived {
		return false, nil
	}

	data, ok := envelope["data"].(map[string]any)
	if !ok {
		return false, nil
	}

	identity, ok := data["external_identity_hash"]
	if !ok {
		return true, nil
	}

	return strings.TrimSpace(stringValue(identity)) == "", nil
}

func stringValue(value any) string {
	stringValue, ok := value.(string)
	if !ok {
		return ""
	}

	return stringValue
}
