package consumer

import (
	"context"
	"testing"
	"time"

	"github.com/ixxet/apollo/internal/visits"
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

	result, err := handler.HandleMessage(context.Background(), []byte(`{
		"id":"evt-001",
		"source":"athena",
		"type":"athena.identified_presence.arrived",
		"timestamp":"2026-04-01T12:30:00Z",
		"data":{
			"facility_id":"ashtonbee",
			"zone_id":"weight-room",
			"external_identity_hash":"tag_tracer2_001",
			"source":"mock",
			"recorded_at":"2026-04-01T12:30:00Z"
		}
	}`))
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

	result, err := handler.HandleMessage(context.Background(), []byte(`{
		"id":"evt-002",
		"source":"athena",
		"type":"athena.identified_presence.arrived",
		"timestamp":"2026-04-01T12:31:00Z",
		"data":{
			"facility_id":"ashtonbee",
			"external_identity_hash":"unknown",
			"source":"mock",
			"recorded_at":"2026-04-01T12:31:00Z"
		}
	}`))
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}
	if result.Outcome != visits.OutcomeUnknownTag {
		t.Fatalf("result.Outcome = %q, want %q", result.Outcome, visits.OutcomeUnknownTag)
	}
}

func TestHandleMessageIgnoresAnonymousPayload(t *testing.T) {
	recorder := &stubArrivalRecorder{}
	handler := NewIdentifiedPresenceHandler(recorder)

	result, err := handler.HandleMessage(context.Background(), []byte(`{
		"id":"evt-003",
		"source":"athena",
		"type":"athena.identified_presence.arrived",
		"timestamp":"2026-04-01T12:32:00Z",
		"data":{
			"facility_id":"ashtonbee",
			"external_identity_hash":"",
			"source":"mock",
			"recorded_at":"2026-04-01T12:32:00Z"
		}
	}`))
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}
	if result.Outcome != visits.OutcomeIgnoredAnonymous {
		t.Fatalf("result.Outcome = %q, want %q", result.Outcome, visits.OutcomeIgnoredAnonymous)
	}
	if recorder.input != nil {
		t.Fatal("RecordArrival() should not be called for anonymous events")
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

	if _, err := handler.HandleMessage(context.Background(), []byte(`{
		"id":"evt-004",
		"source":"athena",
		"type":"athena.identified_presence.arrived",
		"timestamp":"2026-04-01T12:33:00Z",
		"data":{
			"external_identity_hash":"tag_tracer2_001",
			"source":"mock",
			"recorded_at":"2026-04-01T12:33:00Z"
		}
	}`)); err == nil {
		t.Fatal("HandleMessage() error = nil, want missing facility_id error")
	}
}

func TestHandleMessageRejectsMissingTimestamp(t *testing.T) {
	handler := NewIdentifiedPresenceHandler(&stubArrivalRecorder{})

	if _, err := handler.HandleMessage(context.Background(), []byte(`{
		"id":"evt-005",
		"source":"athena",
		"type":"athena.identified_presence.arrived",
		"data":{
			"facility_id":"ashtonbee",
			"external_identity_hash":"tag_tracer2_001",
			"source":"mock",
			"recorded_at":"2026-04-01T12:34:00Z"
		}
	}`)); err == nil {
		t.Fatal("HandleMessage() error = nil, want missing timestamp error")
	}
}
