package visits

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ixxet/apollo/internal/store"
)

type stubFinder struct {
	userByTag      *store.ApolloUser
	visitBySource  *store.ApolloVisit
	openVisit      *store.ApolloVisit
	createdVisit   *store.ApolloVisit
	createVisitErr error
	createParams   *store.CreateVisitParams
}

func (s *stubFinder) FindActiveUserByTagHash(context.Context, string) (*store.ApolloUser, error) {
	return s.userByTag, nil
}

func (s *stubFinder) GetVisitBySourceEventID(context.Context, string) (*store.ApolloVisit, error) {
	return s.visitBySource, nil
}

func (s *stubFinder) GetOpenVisitByUserAndFacility(context.Context, uuid.UUID, string) (*store.ApolloVisit, error) {
	return s.openVisit, nil
}

func (s *stubFinder) CreateVisit(_ context.Context, params store.CreateVisitParams) (*store.ApolloVisit, error) {
	s.createParams = &params
	return s.createdVisit, s.createVisitErr
}

func TestRecordArrivalReturnsUnknownTag(t *testing.T) {
	service := NewService(&stubFinder{})

	result, err := service.RecordArrival(context.Background(), ArrivalInput{
		SourceEventID:        "mock-in-001",
		FacilityKey:          "ashtonbee",
		ExternalIdentityHash: "unknown",
		ArrivedAt:            time.Date(2026, 4, 1, 12, 30, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("RecordArrival() error = %v", err)
	}
	if result.Outcome != OutcomeUnknownTag {
		t.Fatalf("result.Outcome = %q, want %q", result.Outcome, OutcomeUnknownTag)
	}
}

func TestRecordArrivalIgnoresAnonymous(t *testing.T) {
	service := NewService(&stubFinder{})

	result, err := service.RecordArrival(context.Background(), ArrivalInput{
		SourceEventID:        "mock-in-000",
		FacilityKey:          "ashtonbee",
		ExternalIdentityHash: "",
		ArrivedAt:            time.Date(2026, 4, 1, 12, 30, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("RecordArrival() error = %v", err)
	}
	if result.Outcome != OutcomeIgnoredAnonymous {
		t.Fatalf("result.Outcome = %q, want %q", result.Outcome, OutcomeIgnoredAnonymous)
	}
}

func TestRecordArrivalReturnsDuplicateForExistingSourceEventID(t *testing.T) {
	existingVisit := &store.ApolloVisit{FacilityKey: "ashtonbee"}
	service := NewService(&stubFinder{
		userByTag:     &store.ApolloUser{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111")},
		visitBySource: existingVisit,
	})

	result, err := service.RecordArrival(context.Background(), ArrivalInput{
		SourceEventID:        "mock-in-001",
		FacilityKey:          "ashtonbee",
		ExternalIdentityHash: "tag_tracer2_001",
		ArrivedAt:            time.Date(2026, 4, 1, 12, 30, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("RecordArrival() error = %v", err)
	}
	if result.Outcome != OutcomeDuplicate {
		t.Fatalf("result.Outcome = %q, want %q", result.Outcome, OutcomeDuplicate)
	}
	if result.Visit != existingVisit {
		t.Fatal("result.Visit did not return the duplicate visit")
	}
}

func TestRecordArrivalReturnsAlreadyOpenForOpenFacilityVisit(t *testing.T) {
	openVisit := &store.ApolloVisit{FacilityKey: "ashtonbee"}
	service := NewService(&stubFinder{
		userByTag: &store.ApolloUser{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111")},
		openVisit: openVisit,
	})

	result, err := service.RecordArrival(context.Background(), ArrivalInput{
		SourceEventID:        "mock-in-002",
		FacilityKey:          "ashtonbee",
		ExternalIdentityHash: "tag_tracer2_001",
		ArrivedAt:            time.Date(2026, 4, 1, 12, 30, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("RecordArrival() error = %v", err)
	}
	if result.Outcome != OutcomeAlreadyOpen {
		t.Fatalf("result.Outcome = %q, want %q", result.Outcome, OutcomeAlreadyOpen)
	}
	if result.Visit != openVisit {
		t.Fatal("result.Visit did not return the open visit")
	}
}

func TestRecordArrivalCreatesVisit(t *testing.T) {
	zoneKey := "weight-room"
	createdVisit := &store.ApolloVisit{FacilityKey: "ashtonbee"}
	repository := &stubFinder{
		userByTag:    &store.ApolloUser{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111")},
		createdVisit: createdVisit,
	}
	service := NewService(repository)
	arrivedAt := time.Date(2026, 4, 1, 12, 30, 0, 0, time.UTC)

	result, err := service.RecordArrival(context.Background(), ArrivalInput{
		SourceEventID:        "mock-in-003",
		FacilityKey:          "ashtonbee",
		ZoneKey:              &zoneKey,
		ExternalIdentityHash: "tag_tracer2_001",
		ArrivedAt:            arrivedAt,
	})
	if err != nil {
		t.Fatalf("RecordArrival() error = %v", err)
	}
	if result.Outcome != OutcomeCreated {
		t.Fatalf("result.Outcome = %q, want %q", result.Outcome, OutcomeCreated)
	}
	if repository.createParams == nil {
		t.Fatal("CreateVisit() was not called")
	}
	if repository.createParams.FacilityKey != "ashtonbee" {
		t.Fatalf("CreateVisit().FacilityKey = %q, want ashtonbee", repository.createParams.FacilityKey)
	}
	if repository.createParams.ZoneKey == nil || *repository.createParams.ZoneKey != zoneKey {
		t.Fatalf("CreateVisit().ZoneKey = %#v, want %q", repository.createParams.ZoneKey, zoneKey)
	}
	if repository.createParams.SourceEventID == nil || *repository.createParams.SourceEventID != "mock-in-003" {
		t.Fatalf("CreateVisit().SourceEventID = %#v, want mock-in-003", repository.createParams.SourceEventID)
	}
	if repository.createParams.ArrivedAt != (pgtype.Timestamptz{Time: arrivedAt, Valid: true}) {
		t.Fatalf("CreateVisit().ArrivedAt = %#v, want %#v", repository.createParams.ArrivedAt, pgtype.Timestamptz{Time: arrivedAt, Valid: true})
	}
}
