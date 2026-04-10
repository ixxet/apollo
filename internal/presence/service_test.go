package presence

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/store"
	"github.com/ixxet/apollo/internal/visits"
)

type stubStore struct {
	linkedVisits   []LinkedVisit
	streaks        []store.ApolloMemberPresenceStreak
	latestEvents   []store.ApolloMemberPresenceStreakEvent
	ensureCalls    []ensureLinkedVisitCall
	ensureErr      error
	linkedVisitErr error
	streakErr      error
	eventErr       error
}

type ensureLinkedVisitCall struct {
	visit   store.ApolloVisit
	tagHash string
	now     time.Time
}

func (s *stubStore) ListLinkedVisitsByUserID(context.Context, uuid.UUID) ([]LinkedVisit, error) {
	return s.linkedVisits, s.linkedVisitErr
}

func (s *stubStore) ListFacilityStreaksByUserID(context.Context, uuid.UUID) ([]store.ApolloMemberPresenceStreak, error) {
	return s.streaks, s.streakErr
}

func (s *stubStore) ListLatestFacilityStreakEventsByUserID(context.Context, uuid.UUID) ([]store.ApolloMemberPresenceStreakEvent, error) {
	return s.latestEvents, s.eventErr
}

func (s *stubStore) EnsureLinkedVisitAndCredit(_ context.Context, visit store.ApolloVisit, tagHash string, now time.Time) error {
	s.ensureCalls = append(s.ensureCalls, ensureLinkedVisitCall{
		visit:   visit,
		tagHash: tagHash,
		now:     now,
	})
	return s.ensureErr
}

type stubVisitRecorder struct {
	result visits.Result
	err    error
}

func (s *stubVisitRecorder) RecordArrival(context.Context, visits.ArrivalInput) (visits.Result, error) {
	return s.result, s.err
}

func (s *stubVisitRecorder) RecordDeparture(context.Context, visits.DepartureInput) (visits.Result, error) {
	return s.result, s.err
}

func TestGetSummaryBuildsFacilityScopedPresenceTapLinksAndStreakEvents(t *testing.T) {
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	visitOneID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	visitTwoID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	visitThreeID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)

	repository := &stubStore{
		linkedVisits: []LinkedVisit{
			{
				Visit: store.ApolloVisit{
					ID:          visitOneID,
					UserID:      userID,
					FacilityKey: "annex",
					ArrivedAt:   pgTimestamp(time.Date(2026, 4, 7, 18, 0, 0, 0, time.UTC)),
					DepartedAt:  pgTimestamp(time.Date(2026, 4, 7, 19, 0, 0, 0, time.UTC)),
				},
				TapLink: store.ApolloVisitTapLink{
					VisitID:  visitOneID,
					LinkedAt: pgTimestamp(time.Date(2026, 4, 7, 18, 1, 0, 0, time.UTC)),
				},
			},
			{
				Visit: store.ApolloVisit{
					ID:          visitTwoID,
					UserID:      userID,
					FacilityKey: "ashtonbee",
					ZoneKey:     stringPtr("gym-floor"),
					ArrivedAt:   pgTimestamp(time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)),
				},
				TapLink: store.ApolloVisitTapLink{
					VisitID:  visitTwoID,
					LinkedAt: pgTimestamp(time.Date(2026, 4, 10, 9, 0, 5, 0, time.UTC)),
				},
			},
			{
				Visit: store.ApolloVisit{
					ID:          visitThreeID,
					UserID:      userID,
					FacilityKey: "ashtonbee",
					ArrivedAt:   pgTimestamp(time.Date(2026, 4, 9, 8, 30, 0, 0, time.UTC)),
					DepartedAt:  pgTimestamp(time.Date(2026, 4, 9, 10, 0, 0, 0, time.UTC)),
				},
				TapLink: store.ApolloVisitTapLink{
					VisitID:  visitThreeID,
					LinkedAt: pgTimestamp(time.Date(2026, 4, 9, 8, 30, 5, 0, time.UTC)),
				},
			},
		},
		streaks: []store.ApolloMemberPresenceStreak{
			{
				UserID:          userID,
				FacilityKey:     "annex",
				CurrentCount:    2,
				CurrentStartDay: pgDate(time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)),
				LastCreditedDay: pgDate(time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC)),
			},
			{
				UserID:            userID,
				FacilityKey:       "ashtonbee",
				CurrentCount:      4,
				CurrentStartDay:   pgDate(time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC)),
				LastCreditedDay:   pgDate(time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)),
				LastLinkedVisitID: visitTwoID,
			},
		},
		latestEvents: []store.ApolloMemberPresenceStreakEvent{
			{
				ID:          uuid.MustParse("55555555-5555-5555-5555-555555555555"),
				UserID:      userID,
				FacilityKey: "annex",
				EventKind:   "continued",
				CountAfter:  2,
				StreakDay:   pgDate(time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC)),
				VisitID:     visitOneID,
			},
			{
				ID:          uuid.MustParse("66666666-6666-6666-6666-666666666666"),
				UserID:      userID,
				FacilityKey: "ashtonbee",
				EventKind:   "continued",
				CountAfter:  4,
				StreakDay:   pgDate(time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)),
				VisitID:     visitTwoID,
			},
		},
	}
	service := NewService(repository, &stubVisitRecorder{})
	service.now = func() time.Time { return now }

	summary, err := service.GetSummary(context.Background(), userID)
	if err != nil {
		t.Fatalf("GetSummary() error = %v", err)
	}
	if len(summary.Facilities) != 2 {
		t.Fatalf("len(summary.Facilities) = %d, want 2", len(summary.Facilities))
	}

	annex := summary.Facilities[0]
	if annex.FacilityKey != "annex" {
		t.Fatalf("annex.FacilityKey = %q, want annex", annex.FacilityKey)
	}
	if annex.Status != StatusNotPresent {
		t.Fatalf("annex.Status = %q, want %q", annex.Status, StatusNotPresent)
	}
	if annex.Current != nil {
		t.Fatalf("annex.Current = %#v, want nil", annex.Current)
	}
	if len(annex.RecentVisits) != 1 {
		t.Fatalf("len(annex.RecentVisits) = %d, want 1", len(annex.RecentVisits))
	}
	if annex.RecentVisits[0].TapLink.Status != TapLinkStatusLinked {
		t.Fatalf("annex tap-link status = %q, want %q", annex.RecentVisits[0].TapLink.Status, TapLinkStatusLinked)
	}
	if annex.Streak.Status != StreakStatusInactive {
		t.Fatalf("annex streak status = %q, want %q", annex.Streak.Status, StreakStatusInactive)
	}
	if annex.Streak.LatestEvent == nil || annex.Streak.LatestEvent.VisitID != visitOneID {
		t.Fatalf("annex latest event = %#v, want visit %s", annex.Streak.LatestEvent, visitOneID)
	}

	ashtonbee := summary.Facilities[1]
	if ashtonbee.FacilityKey != "ashtonbee" {
		t.Fatalf("ashtonbee.FacilityKey = %q, want ashtonbee", ashtonbee.FacilityKey)
	}
	if ashtonbee.Status != StatusPresent {
		t.Fatalf("ashtonbee.Status = %q, want %q", ashtonbee.Status, StatusPresent)
	}
	if ashtonbee.Current == nil || ashtonbee.Current.ID != visitTwoID {
		t.Fatalf("ashtonbee.Current = %#v, want visit %s", ashtonbee.Current, visitTwoID)
	}
	if ashtonbee.Current.ZoneKey == nil || *ashtonbee.Current.ZoneKey != "gym-floor" {
		t.Fatalf("ashtonbee.Current.ZoneKey = %#v, want gym-floor", ashtonbee.Current.ZoneKey)
	}
	if len(ashtonbee.RecentVisits) != 2 {
		t.Fatalf("len(ashtonbee.RecentVisits) = %d, want 2", len(ashtonbee.RecentVisits))
	}
	if ashtonbee.RecentVisits[0].ID != visitTwoID {
		t.Fatalf("ashtonbee.RecentVisits[0].ID = %s, want %s", ashtonbee.RecentVisits[0].ID, visitTwoID)
	}
	if ashtonbee.Streak.Status != StreakStatusActive {
		t.Fatalf("ashtonbee streak status = %q, want %q", ashtonbee.Streak.Status, StreakStatusActive)
	}
	if ashtonbee.Streak.CurrentCount != 4 {
		t.Fatalf("ashtonbee current count = %d, want 4", ashtonbee.Streak.CurrentCount)
	}
	if ashtonbee.Streak.LatestEvent == nil || ashtonbee.Streak.LatestEvent.EventDay != "2026-04-10" {
		t.Fatalf("ashtonbee latest event = %#v, want 2026-04-10", ashtonbee.Streak.LatestEvent)
	}
}

func TestGetSummaryRejectsMultipleOpenLinkedVisitsInSameFacility(t *testing.T) {
	userID := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	repository := &stubStore{
		linkedVisits: []LinkedVisit{
			{
				Visit: store.ApolloVisit{
					ID:          uuid.MustParse("88888888-8888-8888-8888-888888888888"),
					UserID:      userID,
					FacilityKey: "ashtonbee",
					ArrivedAt:   pgTimestamp(time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)),
				},
				TapLink: store.ApolloVisitTapLink{
					VisitID:  uuid.MustParse("88888888-8888-8888-8888-888888888888"),
					LinkedAt: pgTimestamp(time.Date(2026, 4, 10, 9, 0, 5, 0, time.UTC)),
				},
			},
			{
				Visit: store.ApolloVisit{
					ID:          uuid.MustParse("99999999-9999-9999-9999-999999999999"),
					UserID:      userID,
					FacilityKey: "ashtonbee",
					ArrivedAt:   pgTimestamp(time.Date(2026, 4, 10, 9, 5, 0, 0, time.UTC)),
				},
				TapLink: store.ApolloVisitTapLink{
					VisitID:  uuid.MustParse("99999999-9999-9999-9999-999999999999"),
					LinkedAt: pgTimestamp(time.Date(2026, 4, 10, 9, 5, 5, 0, time.UTC)),
				},
			},
		},
	}
	service := NewService(repository, &stubVisitRecorder{})

	if _, err := service.GetSummary(context.Background(), userID); err == nil {
		t.Fatal("GetSummary() error = nil, want open-visit integrity error")
	}
}

func TestRecordArrivalEnsuresLinkAndCreditForVisitBackedOutcomes(t *testing.T) {
	visit := &store.ApolloVisit{
		ID:          uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		UserID:      uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
		FacilityKey: "ashtonbee",
		ArrivedAt:   pgTimestamp(time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)),
	}

	tests := []struct {
		name        string
		outcome     visits.Outcome
		wantEnsured bool
	}{
		{name: "created", outcome: visits.OutcomeCreated, wantEnsured: true},
		{name: "duplicate", outcome: visits.OutcomeDuplicate, wantEnsured: true},
		{name: "already open", outcome: visits.OutcomeAlreadyOpen, wantEnsured: true},
		{name: "unknown tag", outcome: visits.OutcomeUnknownTag, wantEnsured: false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			repository := &stubStore{}
			service := NewService(repository, &stubVisitRecorder{
				result: visits.Result{
					Outcome: testCase.outcome,
					Visit:   visit,
				},
			})
			service.now = func() time.Time { return time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC) }

			_, err := service.RecordArrival(context.Background(), visits.ArrivalInput{
				SourceEventID:        "presence-arrival-001",
				FacilityKey:          "ashtonbee",
				ExternalIdentityHash: "tag-presence-001",
				ArrivedAt:            time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC),
			})
			if err != nil {
				t.Fatalf("RecordArrival() error = %v", err)
			}

			if got := len(repository.ensureCalls); testCase.wantEnsured && got != 1 {
				t.Fatalf("len(ensureCalls) = %d, want 1", got)
			} else if !testCase.wantEnsured && got != 0 {
				t.Fatalf("len(ensureCalls) = %d, want 0", got)
			}
		})
	}
}
