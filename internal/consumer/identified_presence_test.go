package consumer

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ixxet/apollo/internal/visits"
	protoevents "github.com/ixxet/ashton-proto/events"
)

type stubArrivalRecorder struct {
	result visits.Result
	err    error
	input  *visits.ArrivalInput
}

func (s *stubArrivalRecorder) RecordArrival(_ context.Context, input visits.ArrivalInput) (visits.Result, error) {
	s.input = &input
	return s.result, s.err
}

func TestHandleMessageMapsValidPayloadToArrivalInput(t *testing.T) {
	recorder := &stubArrivalRecorder{
		result: visits.Result{Outcome: visits.OutcomeCreated},
	}
	handler := NewIdentifiedPresenceHandler(recorder)

	result, err := handler.HandleMessage(context.Background(), mutateValidPayload(t, func(event map[string]any) {
		event["id"] = "evt-001"
	}))
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}
	if result.Outcome != visits.OutcomeCreated {
		t.Fatalf("result.Outcome = %q, want %q", result.Outcome, visits.OutcomeCreated)
	}
	if recorder.input == nil {
		t.Fatal("RecordArrival() was not called")
	}
	if recorder.input.SourceEventID != "evt-001" {
		t.Fatalf("input.SourceEventID = %q, want evt-001", recorder.input.SourceEventID)
	}
	if recorder.input.FacilityKey != "ashtonbee" {
		t.Fatalf("input.FacilityKey = %q, want ashtonbee", recorder.input.FacilityKey)
	}
	if recorder.input.ZoneKey == nil || *recorder.input.ZoneKey != "weight-room" {
		t.Fatalf("input.ZoneKey = %#v, want weight-room", recorder.input.ZoneKey)
	}
	if !recorder.input.ArrivedAt.Equal(time.Date(2026, 4, 1, 12, 30, 0, 0, time.UTC)) {
		t.Fatalf("input.ArrivedAt = %s, want 2026-04-01T12:30:00Z", recorder.input.ArrivedAt)
	}
}

func TestHandleMessageReturnsUnknownTagOutcome(t *testing.T) {
	handler := NewIdentifiedPresenceHandler(&stubArrivalRecorder{
		result: visits.Result{Outcome: visits.OutcomeUnknownTag},
	})

	result, err := handler.HandleMessage(context.Background(), mutateValidPayload(t, func(event map[string]any) {
		event["id"] = "evt-002"
		event["data"].(map[string]any)["external_identity_hash"] = "unknown"
	}))
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}
	if result.Outcome != visits.OutcomeUnknownTag {
		t.Fatalf("result.Outcome = %q, want %q", result.Outcome, visits.OutcomeUnknownTag)
	}
}

func TestHandleMessageRejectsAnonymousContractViolation(t *testing.T) {
	handler := NewIdentifiedPresenceHandler(&stubArrivalRecorder{})

	if _, err := handler.HandleMessage(context.Background(), mutateValidPayload(t, func(event map[string]any) {
		event["id"] = "evt-003"
		event["data"].(map[string]any)["external_identity_hash"] = ""
	})); err == nil {
		t.Fatal("HandleMessage() error = nil, want anonymous contract failure")
	}
}

func TestHandleMessageRejectsMalformedJSON(t *testing.T) {
	handler := NewIdentifiedPresenceHandler(&stubArrivalRecorder{})

	if _, err := handler.HandleMessage(context.Background(), []byte(`{`)); err == nil {
		t.Fatal("HandleMessage() error = nil, want malformed JSON error")
	}
}

func TestHandleMessageRejectsMissingFacilityID(t *testing.T) {
	handler := NewIdentifiedPresenceHandler(&stubArrivalRecorder{})

	if _, err := handler.HandleMessage(context.Background(), protoevents.MissingFacilityIDIdentifiedPresenceArrivedFixture()); err == nil {
		t.Fatal("HandleMessage() error = nil, want missing facility_id error")
	}
}

func TestHandleMessageRejectsMissingTimestamp(t *testing.T) {
	handler := NewIdentifiedPresenceHandler(&stubArrivalRecorder{})

	if _, err := handler.HandleMessage(context.Background(), mutateValidPayload(t, func(event map[string]any) {
		delete(event, "timestamp")
	})); err == nil {
		t.Fatal("HandleMessage() error = nil, want missing timestamp error")
	}
}

func TestHandleMessageRejectsMissingID(t *testing.T) {
	handler := NewIdentifiedPresenceHandler(&stubArrivalRecorder{})

	if _, err := handler.HandleMessage(context.Background(), mutateValidPayload(t, func(event map[string]any) {
		delete(event, "id")
	})); err == nil {
		t.Fatal("HandleMessage() error = nil, want missing id error")
	}
}

func TestHandleMessageRejectsWrongEnvelopeSource(t *testing.T) {
	handler := NewIdentifiedPresenceHandler(&stubArrivalRecorder{})

	if _, err := handler.HandleMessage(context.Background(), mutateValidPayload(t, func(event map[string]any) {
		event["source"] = "apollo"
	})); err == nil {
		t.Fatal("HandleMessage() error = nil, want wrong source error")
	}
}

func TestHandleMessageRejectsWrongEnvelopeType(t *testing.T) {
	handler := NewIdentifiedPresenceHandler(&stubArrivalRecorder{})

	if _, err := handler.HandleMessage(context.Background(), mutateValidPayload(t, func(event map[string]any) {
		event["type"] = "athena.identified_presence.departed"
	})); err == nil {
		t.Fatal("HandleMessage() error = nil, want wrong type error")
	}
}

func TestHandleMessageRejectsInvalidSourceValue(t *testing.T) {
	handler := NewIdentifiedPresenceHandler(&stubArrivalRecorder{})

	if _, err := handler.HandleMessage(context.Background(), protoevents.InvalidSourceIdentifiedPresenceArrivedFixture()); err == nil {
		t.Fatal("HandleMessage() error = nil, want invalid source error")
	}
}

func TestHandleMessageRejectsInvalidRecordedAt(t *testing.T) {
	handler := NewIdentifiedPresenceHandler(&stubArrivalRecorder{})

	if _, err := handler.HandleMessage(context.Background(), protoevents.InvalidRecordedAtIdentifiedPresenceArrivedFixture()); err == nil {
		t.Fatal("HandleMessage() error = nil, want invalid recorded_at error")
	}
}

func mutateValidPayload(t *testing.T, mutate func(map[string]any)) []byte {
	t.Helper()

	var event map[string]any
	if err := json.Unmarshal(protoevents.ValidIdentifiedPresenceArrivedFixture(), &event); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	mutate(event)

	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	return payload
}

func TestHandleMessageErrorMentionsContractFailure(t *testing.T) {
	handler := NewIdentifiedPresenceHandler(&stubArrivalRecorder{})

	_, err := handler.HandleMessage(context.Background(), protoevents.InvalidSourceIdentifiedPresenceArrivedFixture())
	if err == nil {
		t.Fatal("HandleMessage() error = nil, want contract error")
	}
	if !strings.Contains(err.Error(), "source") {
		t.Fatalf("HandleMessage() error = %v, want source context", err)
	}
}
