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

type stubDepartureRecorder struct {
	result visits.Result
	err    error
	input  *visits.DepartureInput
}

func (s *stubDepartureRecorder) RecordDeparture(_ context.Context, input visits.DepartureInput) (visits.Result, error) {
	s.input = &input
	return s.result, s.err
}

func TestHandleDepartureMessageMapsValidPayloadToDepartureInput(t *testing.T) {
	recorder := &stubDepartureRecorder{
		result: visits.Result{Outcome: visits.OutcomeClosed},
	}
	handler := NewIdentifiedDepartureHandler(recorder)

	result, err := handler.HandleMessage(context.Background(), mutateValidDeparturePayload(t, func(event map[string]any) {
		event["id"] = "evt-depart-001"
	}))
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}
	if result.Outcome != visits.OutcomeClosed {
		t.Fatalf("result.Outcome = %q, want %q", result.Outcome, visits.OutcomeClosed)
	}
	if recorder.input == nil {
		t.Fatal("RecordDeparture() was not called")
	}
	if recorder.input.SourceEventID != "evt-depart-001" {
		t.Fatalf("input.SourceEventID = %q, want evt-depart-001", recorder.input.SourceEventID)
	}
	if recorder.input.FacilityKey != "ashtonbee" {
		t.Fatalf("input.FacilityKey = %q, want ashtonbee", recorder.input.FacilityKey)
	}
	if !recorder.input.DepartedAt.Equal(time.Date(2026, 4, 1, 12, 45, 0, 0, time.UTC)) {
		t.Fatalf("input.DepartedAt = %s, want 2026-04-01T12:45:00Z", recorder.input.DepartedAt)
	}
}

func TestHandleDepartureMessageReturnsNoOpenVisitOutcome(t *testing.T) {
	handler := NewIdentifiedDepartureHandler(&stubDepartureRecorder{
		result: visits.Result{Outcome: visits.OutcomeNoOpenVisit},
	})

	result, err := handler.HandleMessage(context.Background(), mutateValidDeparturePayload(t, func(event map[string]any) {
		event["id"] = "evt-depart-002"
	}))
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}
	if result.Outcome != visits.OutcomeNoOpenVisit {
		t.Fatalf("result.Outcome = %q, want %q", result.Outcome, visits.OutcomeNoOpenVisit)
	}
}

func TestHandleDepartureMessageIgnoresAnonymousPayload(t *testing.T) {
	recorder := &stubDepartureRecorder{}
	handler := NewIdentifiedDepartureHandler(recorder)

	result, err := handler.HandleMessage(context.Background(), mutateValidDeparturePayload(t, func(event map[string]any) {
		event["id"] = "evt-depart-003"
		event["data"].(map[string]any)["external_identity_hash"] = ""
	}))
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}
	if result.Outcome != visits.OutcomeIgnoredAnonymous {
		t.Fatalf("result.Outcome = %q, want %q", result.Outcome, visits.OutcomeIgnoredAnonymous)
	}
	if recorder.input != nil {
		t.Fatal("RecordDeparture() should not be called for anonymous events")
	}
}

func TestHandleDepartureMessageRejectsMissingFacilityID(t *testing.T) {
	handler := NewIdentifiedDepartureHandler(&stubDepartureRecorder{})

	if _, err := handler.HandleMessage(context.Background(), protoevents.MissingFacilityIDIdentifiedPresenceDepartedFixture()); err == nil {
		t.Fatal("HandleMessage() error = nil, want missing facility_id error")
	}
}

func TestHandleDepartureMessageRejectsInvalidSourceValue(t *testing.T) {
	handler := NewIdentifiedDepartureHandler(&stubDepartureRecorder{})

	if _, err := handler.HandleMessage(context.Background(), protoevents.InvalidSourceIdentifiedPresenceDepartedFixture()); err == nil {
		t.Fatal("HandleMessage() error = nil, want invalid source error")
	}
}

func TestHandleDepartureMessageRejectsInvalidRecordedAt(t *testing.T) {
	handler := NewIdentifiedDepartureHandler(&stubDepartureRecorder{})

	if _, err := handler.HandleMessage(context.Background(), protoevents.InvalidRecordedAtIdentifiedPresenceDepartedFixture()); err == nil {
		t.Fatal("HandleMessage() error = nil, want invalid recorded_at error")
	}
}

func TestHandleDepartureMessageRejectsWrongEnvelopeType(t *testing.T) {
	handler := NewIdentifiedDepartureHandler(&stubDepartureRecorder{})

	if _, err := handler.HandleMessage(context.Background(), mutateValidDeparturePayload(t, func(event map[string]any) {
		event["type"] = protoevents.SubjectIdentifiedPresenceArrived
	})); err == nil {
		t.Fatal("HandleMessage() error = nil, want wrong type error")
	}
}

func TestHandleDepartureMessageErrorMentionsContractFailure(t *testing.T) {
	handler := NewIdentifiedDepartureHandler(&stubDepartureRecorder{})

	_, err := handler.HandleMessage(context.Background(), protoevents.InvalidSourceIdentifiedPresenceDepartedFixture())
	if err == nil {
		t.Fatal("HandleMessage() error = nil, want contract error")
	}
	if !strings.Contains(err.Error(), "source") {
		t.Fatalf("HandleMessage() error = %v, want source context", err)
	}
}

func mutateValidDeparturePayload(t *testing.T, mutate func(map[string]any)) []byte {
	t.Helper()

	var event map[string]any
	if err := json.Unmarshal(protoevents.ValidIdentifiedPresenceDepartedFixture(), &event); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	mutate(event)

	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	return payload
}
