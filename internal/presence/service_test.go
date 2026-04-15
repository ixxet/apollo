package presence

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/schedule"
	"github.com/ixxet/apollo/internal/store"
	"github.com/ixxet/apollo/internal/visits"
)

type stubStore struct {
	linkedVisits      []LinkedVisit
	streaks           []store.ApolloMemberPresenceStreak
	latestEvents      []store.ApolloMemberPresenceStreakEvent
	claimedTags       []store.ApolloClaimedTag
	claimedTag        *store.ApolloClaimedTag
	facilityRefs      []string
	facilitySports    []FacilitySport
	ensureCalls       []ensureLinkedVisitCall
	ensureErr         error
	linkedVisitErr    error
	streakErr         error
	eventErr          error
	claimListErr      error
	claimLookupErr    error
	claimCreateErr    error
	facilityRefsErr   error
	facilitySportsErr error
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

func (s *stubStore) ListClaimedTagsByUserID(context.Context, uuid.UUID) ([]store.ApolloClaimedTag, error) {
	return s.claimedTags, s.claimListErr
}

func (s *stubStore) GetClaimedTagByHash(context.Context, string) (*store.ApolloClaimedTag, error) {
	return s.claimedTag, s.claimLookupErr
}

func (s *stubStore) CreateClaimedTag(_ context.Context, userID uuid.UUID, tagHash string, label *string) (store.ApolloClaimedTag, error) {
	if s.claimCreateErr != nil {
		return store.ApolloClaimedTag{}, s.claimCreateErr
	}
	return store.ApolloClaimedTag{
		UserID:    userID,
		TagHash:   tagHash,
		Label:     label,
		IsActive:  true,
		ClaimedAt: pgTimestamp(time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)),
	}, nil
}

func (s *stubStore) ListFacilityCatalogRefs(context.Context) ([]string, error) {
	return s.facilityRefs, s.facilityRefsErr
}

func (s *stubStore) ListFacilitySports(context.Context) ([]FacilitySport, error) {
	return s.facilitySports, s.facilitySportsErr
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

type stubFacilityCalendar struct {
	occurrencesByFacility map[string][]schedule.Occurrence
	err                   error
	calls                 []calendarCall
}

type calendarCall struct {
	facilityKey string
	window      schedule.CalendarWindow
}

func (s *stubFacilityCalendar) GetCalendar(_ context.Context, facilityKey string, window schedule.CalendarWindow) ([]schedule.Occurrence, error) {
	s.calls = append(s.calls, calendarCall{
		facilityKey: facilityKey,
		window:      window,
	})
	if s.err != nil {
		return nil, s.err
	}
	return append([]schedule.Occurrence(nil), s.occurrencesByFacility[facilityKey]...), nil
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

func TestGetMemberFacilityCalendarProjectsOnlyMemberSafeFacilityWindows(t *testing.T) {
	userID := uuid.MustParse("12121212-3434-5656-7878-909090909090")
	window := schedule.CalendarWindow{
		From:  time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
	}

	calendar := &stubFacilityCalendar{
		occurrencesByFacility: map[string][]schedule.Occurrence{
			"ashtonbee": {
				{
					FacilityKey:    "ashtonbee",
					Scope:          schedule.ScopeFacility,
					Kind:           schedule.KindOperatingHours,
					Visibility:     schedule.VisibilityPublicLabeled,
					Status:         schedule.StatusScheduled,
					StartsAt:       time.Date(2026, 4, 16, 13, 0, 0, 0, time.UTC),
					EndsAt:         time.Date(2026, 4, 16, 21, 0, 0, 0, time.UTC),
					OccurrenceDate: "2026-04-16",
				},
				{
					FacilityKey:    "ashtonbee",
					Scope:          schedule.ScopeFacility,
					Kind:           schedule.KindClosure,
					Visibility:     schedule.VisibilityPublicBusy,
					Status:         schedule.StatusScheduled,
					StartsAt:       time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC),
					EndsAt:         time.Date(2026, 4, 18, 16, 0, 0, 0, time.UTC),
					OccurrenceDate: "2026-04-18",
				},
				{
					FacilityKey:    "ashtonbee",
					Scope:          schedule.ScopeFacility,
					Kind:           schedule.KindEvent,
					Visibility:     schedule.VisibilityPublicBusy,
					Status:         schedule.StatusScheduled,
					StartsAt:       time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC),
					EndsAt:         time.Date(2026, 4, 19, 13, 0, 0, 0, time.UTC),
					OccurrenceDate: "2026-04-19",
				},
				{
					FacilityKey:    "ashtonbee",
					Scope:          schedule.ScopeFacility,
					Kind:           schedule.KindOperatingHours,
					Visibility:     schedule.VisibilityInternal,
					Status:         schedule.StatusScheduled,
					StartsAt:       time.Date(2026, 4, 20, 13, 0, 0, 0, time.UTC),
					EndsAt:         time.Date(2026, 4, 20, 21, 0, 0, 0, time.UTC),
					OccurrenceDate: "2026-04-20",
				},
				{
					FacilityKey:    "ashtonbee",
					Scope:          schedule.ScopeZone,
					Kind:           schedule.KindOperatingHours,
					Visibility:     schedule.VisibilityPublicLabeled,
					Status:         schedule.StatusScheduled,
					StartsAt:       time.Date(2026, 4, 21, 13, 0, 0, 0, time.UTC),
					EndsAt:         time.Date(2026, 4, 21, 15, 0, 0, 0, time.UTC),
					OccurrenceDate: "2026-04-21",
				},
				{
					FacilityKey:    "ashtonbee",
					Scope:          schedule.ScopeResource,
					Kind:           schedule.KindClosure,
					Visibility:     schedule.VisibilityPublicBusy,
					Status:         schedule.StatusScheduled,
					StartsAt:       time.Date(2026, 4, 22, 9, 0, 0, 0, time.UTC),
					EndsAt:         time.Date(2026, 4, 22, 10, 0, 0, 0, time.UTC),
					OccurrenceDate: "2026-04-22",
				},
				{
					FacilityKey:    "ashtonbee",
					Scope:          schedule.ScopeFacility,
					Kind:           schedule.KindOperatingHours,
					Visibility:     schedule.VisibilityPublicLabeled,
					Status:         schedule.StatusCancelled,
					StartsAt:       time.Date(2026, 4, 23, 13, 0, 0, 0, time.UTC),
					EndsAt:         time.Date(2026, 4, 23, 21, 0, 0, 0, time.UTC),
					OccurrenceDate: "2026-04-23",
				},
			},
		},
	}

	service := NewService(&stubStore{
		facilityRefs: []string{"annex", "ashtonbee"},
	}, &stubVisitRecorder{}, WithFacilityCalendar(calendar))

	result, err := service.GetMemberFacilityCalendar(context.Background(), userID, "ashtonbee", window)
	if err != nil {
		t.Fatalf("GetMemberFacilityCalendar() error = %v", err)
	}
	if result.FacilityKey != "ashtonbee" {
		t.Fatalf("result.FacilityKey = %q, want ashtonbee", result.FacilityKey)
	}
	if !result.From.Equal(window.From) {
		t.Fatalf("result.From = %s, want %s", result.From.Format(time.RFC3339), window.From.Format(time.RFC3339))
	}
	if !result.Until.Equal(window.Until) {
		t.Fatalf("result.Until = %s, want %s", result.Until.Format(time.RFC3339), window.Until.Format(time.RFC3339))
	}
	if got, want := len(result.Hours), 1; got != want {
		t.Fatalf("len(result.Hours) = %d, want %d", got, want)
	}
	if got, want := len(result.Closures), 1; got != want {
		t.Fatalf("len(result.Closures) = %d, want %d", got, want)
	}
	if result.Hours[0].OccurrenceDate != "2026-04-16" {
		t.Fatalf("result.Hours[0].OccurrenceDate = %q, want 2026-04-16", result.Hours[0].OccurrenceDate)
	}
	if result.Closures[0].OccurrenceDate != "2026-04-18" {
		t.Fatalf("result.Closures[0].OccurrenceDate = %q, want 2026-04-18", result.Closures[0].OccurrenceDate)
	}
	if got, want := len(calendar.calls), 1; got != want {
		t.Fatalf("len(calendar.calls) = %d, want %d", got, want)
	}
	if calendar.calls[0].facilityKey != "ashtonbee" {
		t.Fatalf("calendar.calls[0].facilityKey = %q, want ashtonbee", calendar.calls[0].facilityKey)
	}
}

func TestGetMemberFacilityCalendarRejectsMissingFacilityInvalidWindowsAndUnknownFacilities(t *testing.T) {
	userID := uuid.MustParse("98989898-7878-5656-3434-121212121212")
	service := NewService(&stubStore{
		facilityRefs: []string{"ashtonbee"},
	}, &stubVisitRecorder{}, WithFacilityCalendar(&stubFacilityCalendar{}))

	tests := []struct {
		name        string
		facilityKey string
		window      schedule.CalendarWindow
		wantErr     error
	}{
		{
			name:        "missing facility key",
			facilityKey: " ",
			window: schedule.CalendarWindow{
				From:  time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
				Until: time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC),
			},
			wantErr: ErrMemberFacilityKeyRequired,
		},
		{
			name:        "missing from boundary",
			facilityKey: "ashtonbee",
			window: schedule.CalendarWindow{
				Until: time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC),
			},
			wantErr: ErrMemberCalendarWindowInvalid,
		},
		{
			name:        "equal boundaries",
			facilityKey: "ashtonbee",
			window: schedule.CalendarWindow{
				From:  time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC),
				Until: time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC),
			},
			wantErr: ErrMemberCalendarWindowInvalid,
		},
		{
			name:        "reversed boundaries",
			facilityKey: "ashtonbee",
			window: schedule.CalendarWindow{
				From:  time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
				Until: time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC),
			},
			wantErr: ErrMemberCalendarWindowInvalid,
		},
		{
			name:        "too wide",
			facilityKey: "ashtonbee",
			window: schedule.CalendarWindow{
				From:  time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
				Until: time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC),
			},
			wantErr: ErrMemberCalendarWindowTooWide,
		},
		{
			name:        "unknown facility",
			facilityKey: "annex",
			window: schedule.CalendarWindow{
				From:  time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
				Until: time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC),
			},
			wantErr: ErrMemberFacilityNotFound,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := service.GetMemberFacilityCalendar(context.Background(), userID, testCase.facilityKey, testCase.window)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("GetMemberFacilityCalendar() error = %v, want %v", err, testCase.wantErr)
			}
		})
	}
}
