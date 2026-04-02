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
	userByTag        *store.ApolloUser
	visitBySource    *store.ApolloVisit
	visitByDeparture *store.ApolloVisit
	openVisit        *store.ApolloVisit
	createdVisit     *store.ApolloVisit
	closedVisit      *store.ApolloVisit
	createVisitErr   error
	closeVisitErr    error
	createParams     *store.CreateVisitParams
	closeParams      *store.CloseVisitParams
}

func (s *stubFinder) FindActiveUserByTagHash(context.Context, string) (*store.ApolloUser, error) {
	return s.userByTag, nil
}

func (s *stubFinder) GetVisitBySourceEventID(context.Context, string) (*store.ApolloVisit, error) {
	return s.visitBySource, nil
}

func (s *stubFinder) GetVisitByDepartureSourceEventID(context.Context, string) (*store.ApolloVisit, error) {
	return s.visitByDeparture, nil
}

func (s *stubFinder) GetOpenVisitByUserAndFacility(context.Context, uuid.UUID, string) (*store.ApolloVisit, error) {
	return s.openVisit, nil
}

func (s *stubFinder) CreateVisit(_ context.Context, params store.CreateVisitParams) (*store.ApolloVisit, error) {
	s.createParams = &params
	return s.createdVisit, s.createVisitErr
}

func (s *stubFinder) CloseVisit(_ context.Context, params store.CloseVisitParams) (*store.ApolloVisit, error) {
	s.closeParams = &params
	return s.closedVisit, s.closeVisitErr
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

func TestRecordDepartureEvaluatesLifecycleOutcomes(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	arrivedAt := time.Date(2026, 4, 1, 12, 30, 0, 0, time.UTC)
	departedAt := time.Date(2026, 4, 1, 13, 10, 0, 0, time.UTC)
	openVisit := &store.ApolloVisit{
		ID:          uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		UserID:      userID,
		FacilityKey: "ashtonbee",
		ArrivedAt:   pgtype.Timestamptz{Time: arrivedAt, Valid: true},
	}
	closedVisit := &store.ApolloVisit{
		ID:                     openVisit.ID,
		UserID:                 userID,
		FacilityKey:            "ashtonbee",
		ArrivedAt:              pgtype.Timestamptz{Time: arrivedAt, Valid: true},
		DepartedAt:             pgtype.Timestamptz{Time: departedAt, Valid: true},
		DepartureSourceEventID: stringPtr("mock-out-001"),
	}

	tests := []struct {
		name            string
		repository      *stubFinder
		input           DepartureInput
		wantOutcome     Outcome
		wantCloseCalled bool
		wantVisit       *store.ApolloVisit
	}{
		{
			name:       "anonymous departure is ignored",
			repository: &stubFinder{},
			input: DepartureInput{
				SourceEventID: "mock-out-000",
				FacilityKey:   "ashtonbee",
				DepartedAt:    departedAt,
			},
			wantOutcome: OutcomeIgnoredAnonymous,
		},
		{
			name:       "unknown tag returns deterministic outcome",
			repository: &stubFinder{},
			input: DepartureInput{
				SourceEventID:        "mock-out-unknown",
				FacilityKey:          "ashtonbee",
				ExternalIdentityHash: "unknown",
				DepartedAt:           departedAt,
			},
			wantOutcome: OutcomeUnknownTag,
		},
		{
			name: "duplicate departure returns existing closed visit",
			repository: &stubFinder{
				userByTag:        &store.ApolloUser{ID: userID},
				visitByDeparture: closedVisit,
			},
			input: DepartureInput{
				SourceEventID:        "mock-out-001",
				FacilityKey:          "ashtonbee",
				ExternalIdentityHash: "tag_tracer5_001",
				DepartedAt:           departedAt,
			},
			wantOutcome: OutcomeDuplicate,
			wantVisit:   closedVisit,
		},
		{
			name: "same user open in another facility does not close",
			repository: &stubFinder{
				userByTag: &store.ApolloUser{ID: userID},
			},
			input: DepartureInput{
				SourceEventID:        "mock-out-002",
				FacilityKey:          "annex",
				ExternalIdentityHash: "tag_tracer5_001",
				DepartedAt:           departedAt,
			},
			wantOutcome: OutcomeNoOpenVisit,
		},
		{
			name: "out of order departure leaves open visit unchanged",
			repository: &stubFinder{
				userByTag: &store.ApolloUser{ID: userID},
				openVisit: openVisit,
			},
			input: DepartureInput{
				SourceEventID:        "mock-out-003",
				FacilityKey:          "ashtonbee",
				ExternalIdentityHash: "tag_tracer5_001",
				DepartedAt:           arrivedAt.Add(-time.Minute),
			},
			wantOutcome: OutcomeOutOfOrder,
			wantVisit:   openVisit,
		},
		{
			name: "matching open visit closes deterministically",
			repository: &stubFinder{
				userByTag:   &store.ApolloUser{ID: userID},
				openVisit:   openVisit,
				closedVisit: closedVisit,
			},
			input: DepartureInput{
				SourceEventID:        "mock-out-001",
				FacilityKey:          "ashtonbee",
				ExternalIdentityHash: "tag_tracer5_001",
				DepartedAt:           departedAt,
			},
			wantOutcome:     OutcomeClosed,
			wantCloseCalled: true,
			wantVisit:       closedVisit,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			service := NewService(testCase.repository)

			result, err := service.RecordDeparture(context.Background(), testCase.input)
			if err != nil {
				t.Fatalf("RecordDeparture() error = %v", err)
			}
			if result.Outcome != testCase.wantOutcome {
				t.Fatalf("result.Outcome = %q, want %q", result.Outcome, testCase.wantOutcome)
			}
			if result.Visit != testCase.wantVisit {
				t.Fatalf("result.Visit = %#v, want %#v", result.Visit, testCase.wantVisit)
			}
			if testCase.wantCloseCalled && testCase.repository.closeParams == nil {
				t.Fatal("CloseVisit() was not called")
			}
			if !testCase.wantCloseCalled && testCase.repository.closeParams != nil {
				t.Fatalf("CloseVisit() was called with %#v, want no close", testCase.repository.closeParams)
			}
		})
	}
}

func TestRecordDeparturePassesDepartureTrackingToRepository(t *testing.T) {
	repository := &stubFinder{
		userByTag: &store.ApolloUser{ID: uuid.MustParse("11111111-1111-1111-1111-111111111111")},
		openVisit: &store.ApolloVisit{
			ID:        uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			ArrivedAt: pgtype.Timestamptz{Time: time.Date(2026, 4, 1, 12, 30, 0, 0, time.UTC), Valid: true},
		},
		closedVisit: &store.ApolloVisit{
			ID:                     uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			DepartedAt:             pgtype.Timestamptz{Time: time.Date(2026, 4, 1, 13, 10, 0, 0, time.UTC), Valid: true},
			DepartureSourceEventID: stringPtr("mock-out-010"),
		},
	}
	service := NewService(repository)
	departedAt := time.Date(2026, 4, 1, 13, 10, 0, 0, time.UTC)

	result, err := service.RecordDeparture(context.Background(), DepartureInput{
		SourceEventID:        "mock-out-010",
		FacilityKey:          "ashtonbee",
		ExternalIdentityHash: "tag_tracer5_010",
		DepartedAt:           departedAt,
	})
	if err != nil {
		t.Fatalf("RecordDeparture() error = %v", err)
	}
	if result.Outcome != OutcomeClosed {
		t.Fatalf("result.Outcome = %q, want %q", result.Outcome, OutcomeClosed)
	}
	if repository.closeParams == nil {
		t.Fatal("CloseVisit() was not called")
	}
	if repository.closeParams.DepartureSourceEventID == nil || *repository.closeParams.DepartureSourceEventID != "mock-out-010" {
		t.Fatalf("CloseVisit().DepartureSourceEventID = %#v, want mock-out-010", repository.closeParams.DepartureSourceEventID)
	}
	if repository.closeParams.DepartedAt != (pgtype.Timestamptz{Time: departedAt, Valid: true}) {
		t.Fatalf("CloseVisit().DepartedAt = %#v, want %#v", repository.closeParams.DepartedAt, pgtype.Timestamptz{Time: departedAt, Valid: true})
	}
}
