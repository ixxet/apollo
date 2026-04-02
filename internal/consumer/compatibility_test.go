package consumer

import (
	"context"
	"testing"
	"time"

	"github.com/ixxet/apollo/internal/visits"
	protoevents "github.com/ixxet/ashton-proto/events"
	athenav1 "github.com/ixxet/ashton-proto/gen/go/ashton/athena/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestHandleMessageAcceptsSharedMarshalPayload(t *testing.T) {
	recorder := &stubArrivalRecorder{
		result: visits.Result{Outcome: visits.OutcomeCreated},
	}
	handler := NewIdentifiedPresenceHandler(recorder)

	payload, err := protoevents.MarshalIdentifiedPresenceArrived(protoevents.IdentifiedPresenceArrivedEvent{
		ID:        "evt-compat-001",
		Timestamp: time.Date(2026, 4, 1, 12, 30, 0, 0, time.UTC),
		Data: &athenav1.IdentifiedPresenceArrived{
			FacilityId:           "ashtonbee",
			ZoneId:               "weight-room",
			ExternalIdentityHash: "tag_tracer2_001",
			Source:               athenav1.PresenceSource_PRESENCE_SOURCE_MOCK,
			RecordedAt:           timestamppb.New(time.Date(2026, 4, 1, 12, 30, 0, 0, time.UTC)),
		},
	})
	if err != nil {
		t.Fatalf("MarshalIdentifiedPresenceArrived() error = %v", err)
	}

	result, err := handler.HandleMessage(context.Background(), payload)
	if err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}
	if result.Outcome != visits.OutcomeCreated {
		t.Fatalf("result.Outcome = %q, want %q", result.Outcome, visits.OutcomeCreated)
	}
	if recorder.input == nil {
		t.Fatal("RecordArrival() was not called")
	}
	if recorder.input.SourceEventID != "evt-compat-001" {
		t.Fatalf("input.SourceEventID = %q, want evt-compat-001", recorder.input.SourceEventID)
	}
}
