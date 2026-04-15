package ops

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/athena"
	"github.com/ixxet/apollo/internal/schedule"
)

type stubScheduleReader struct {
	occurrences []schedule.Occurrence
	err         error
	called      bool
	facilityKey string
	window      schedule.CalendarWindow
}

func (s *stubScheduleReader) GetCalendar(_ context.Context, facilityKey string, window schedule.CalendarWindow) ([]schedule.Occurrence, error) {
	s.called = true
	s.facilityKey = facilityKey
	s.window = window
	if s.err != nil {
		return nil, s.err
	}
	return s.occurrences, nil
}

type stubOccupancyReader struct {
	snapshot        athena.OccupancySnapshot
	analytics       athena.AnalyticsReport
	occupancyErr    error
	analyticsErr    error
	analyticsFilter athena.AnalyticsFilter
}

func (s *stubOccupancyReader) CurrentOccupancy(context.Context, string) (athena.OccupancySnapshot, error) {
	if s.occupancyErr != nil {
		return athena.OccupancySnapshot{}, s.occupancyErr
	}
	return s.snapshot, nil
}

func (s *stubOccupancyReader) OccupancyAnalytics(_ context.Context, filter athena.AnalyticsFilter) (athena.AnalyticsReport, error) {
	s.analyticsFilter = filter
	if s.analyticsErr != nil {
		return athena.AnalyticsReport{}, s.analyticsErr
	}
	return s.analytics, nil
}

func TestFacilityOverviewComposesScheduleAndSanitizedAthenaTruth(t *testing.T) {
	from := time.Date(2026, 4, 15, 13, 0, 0, 0, time.UTC)
	until := time.Date(2026, 4, 15, 15, 0, 0, 0, time.UTC)
	zoneKey := "gym-floor"
	resourceKey := "full-court"
	scheduleReader := &stubScheduleReader{
		occurrences: []schedule.Occurrence{
			{
				BlockID:        uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
				FacilityKey:    "ashtonbee",
				ZoneKey:        &zoneKey,
				ResourceKey:    &resourceKey,
				Scope:          schedule.ScopeResource,
				Kind:           schedule.KindEvent,
				Effect:         schedule.EffectHardReserve,
				Visibility:     schedule.VisibilityInternal,
				Status:         schedule.StatusScheduled,
				StartsAt:       from.Add(30 * time.Minute),
				EndsAt:         from.Add(90 * time.Minute),
				OccurrenceDate: "2026-04-15",
			},
			{
				BlockID:        uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
				FacilityKey:    "ashtonbee",
				Scope:          schedule.ScopeFacility,
				Kind:           schedule.KindOperatingHours,
				Effect:         schedule.EffectInformational,
				Visibility:     schedule.VisibilityPublicLabeled,
				Status:         schedule.StatusScheduled,
				StartsAt:       from,
				EndsAt:         until,
				OccurrenceDate: "2026-04-15",
			},
		},
	}
	occupancyReader := &stubOccupancyReader{
		snapshot: athena.OccupancySnapshot{
			FacilityID:   "ashtonbee",
			CurrentCount: 12,
			ObservedAt:   until,
		},
		analytics: athena.AnalyticsReport{
			FacilityID:    "ashtonbee",
			Since:         from,
			Until:         until,
			BucketMinutes: 15,
			ObservationSummary: athena.ObservationSummary{
				Total:         5,
				Pass:          4,
				Fail:          1,
				CommittedPass: 4,
			},
			SessionSummary: athena.SessionSummary{
				OpenCount:      2,
				ClosedCount:    1,
				UniqueVisitors: 3,
				OccupancyAtEnd: 12,
			},
			FlowBuckets: []athena.FlowBucket{{
				StartedAt:    from,
				EndedAt:      from.Add(15 * time.Minute),
				PassIn:       2,
				OccupancyEnd: 11,
			}},
			NodeBreakdown: []athena.NodeBreakdown{{
				NodeID:        "entry-node",
				Total:         5,
				Pass:          4,
				Fail:          1,
				CommittedPass: 4,
			}},
			Sessions: []athena.SessionFact{{
				SessionID:    "session-001",
				State:        "open",
				EntryEventID: "edge-event-001",
			}},
		},
	}

	service := NewService(scheduleReader, occupancyReader, 7*24*time.Hour)
	overview, err := service.GetFacilityOverview(context.Background(), FacilityOverviewInput{
		FacilityKey:   " ashtonbee ",
		From:          from,
		Until:         until,
		BucketMinutes: 15,
	})
	if err != nil {
		t.Fatalf("GetFacilityOverview() error = %v", err)
	}

	if !scheduleReader.called {
		t.Fatal("schedule reader was not called")
	}
	if scheduleReader.facilityKey != "ashtonbee" {
		t.Fatalf("schedule facility = %q, want ashtonbee", scheduleReader.facilityKey)
	}
	if !scheduleReader.window.From.Equal(from) || !scheduleReader.window.Until.Equal(until) {
		t.Fatalf("schedule window = %+v, want %s-%s", scheduleReader.window, from, until)
	}
	if occupancyReader.analyticsFilter.SessionLimit != 1 {
		t.Fatalf("analytics session limit = %d, want 1", occupancyReader.analyticsFilter.SessionLimit)
	}
	if overview.CurrentOccupancy.CurrentCount != 12 {
		t.Fatalf("current count = %d, want 12", overview.CurrentOccupancy.CurrentCount)
	}
	if overview.ScheduleSummary.OccurrenceCount != 2 {
		t.Fatalf("schedule occurrence count = %d, want 2", overview.ScheduleSummary.OccurrenceCount)
	}
	if overview.ScheduleSummary.ByKind[schedule.KindEvent] != 1 {
		t.Fatalf("event count = %d, want 1", overview.ScheduleSummary.ByKind[schedule.KindEvent])
	}
	if len(overview.OccupancyAnalytics.FlowBuckets) != 1 {
		t.Fatalf("flow bucket count = %d, want 1", len(overview.OccupancyAnalytics.FlowBuckets))
	}

	body, err := json.Marshal(overview)
	if err != nil {
		t.Fatalf("json.Marshal(overview) error = %v", err)
	}
	for _, leaked := range []string{"session-001", "edge-event-001", "external_identity_hash", "tag_hash", "account_raw"} {
		if strings.Contains(string(body), leaked) {
			t.Fatalf("overview leaked %q: %s", leaked, body)
		}
	}
}

func TestFacilityOverviewRejectsInvalidInputs(t *testing.T) {
	from := time.Date(2026, 4, 15, 13, 0, 0, 0, time.UTC)
	testCases := []struct {
		name  string
		input FacilityOverviewInput
		want  error
	}{
		{
			name:  "missing facility",
			input: FacilityOverviewInput{From: from, Until: from.Add(time.Hour)},
			want:  ErrFacilityRequired,
		},
		{
			name:  "missing window",
			input: FacilityOverviewInput{FacilityKey: "ashtonbee"},
			want:  ErrWindowRequired,
		},
		{
			name:  "reversed window",
			input: FacilityOverviewInput{FacilityKey: "ashtonbee", From: from.Add(time.Hour), Until: from},
			want:  ErrWindowInvalid,
		},
		{
			name:  "oversized window",
			input: FacilityOverviewInput{FacilityKey: "ashtonbee", From: from, Until: from.Add(25 * time.Hour)},
			want:  ErrWindowTooLarge,
		},
		{
			name:  "negative bucket",
			input: FacilityOverviewInput{FacilityKey: "ashtonbee", From: from, Until: from.Add(time.Hour), BucketMinutes: -1},
			want:  ErrBucketInvalid,
		},
	}

	service := NewService(&stubScheduleReader{}, &stubOccupancyReader{}, 24*time.Hour)
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := service.GetFacilityOverview(context.Background(), testCase.input)
			if !errors.Is(err, testCase.want) {
				t.Fatalf("GetFacilityOverview() error = %v, want %v", err, testCase.want)
			}
		})
	}
}

func TestFacilityOverviewPropagatesScheduleAndAthenaBoundaries(t *testing.T) {
	from := time.Date(2026, 4, 15, 13, 0, 0, 0, time.UTC)
	testCases := []struct {
		name         string
		scheduleErr  error
		occupancyErr error
		analyticsErr error
		want         error
	}{
		{
			name:        "schedule",
			scheduleErr: schedule.ErrResourceFacilityInvalid,
			want:        schedule.ErrResourceFacilityInvalid,
		},
		{
			name:         "occupancy",
			occupancyErr: athena.ErrRequestTimeout,
			want:         athena.ErrRequestTimeout,
		},
		{
			name:         "analytics",
			analyticsErr: athena.ErrAnalyticsMalformed,
			want:         athena.ErrAnalyticsMalformed,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := NewService(
				&stubScheduleReader{err: testCase.scheduleErr},
				&stubOccupancyReader{
					snapshot:     athena.OccupancySnapshot{FacilityID: "ashtonbee", ObservedAt: from, CurrentCount: 1},
					analytics:    athena.AnalyticsReport{FacilityID: "ashtonbee", Since: from, Until: from.Add(time.Hour), BucketMinutes: 15},
					occupancyErr: testCase.occupancyErr,
					analyticsErr: testCase.analyticsErr,
				},
				24*time.Hour,
			)

			_, err := service.GetFacilityOverview(context.Background(), FacilityOverviewInput{
				FacilityKey: "ashtonbee",
				From:        from,
				Until:       from.Add(time.Hour),
			})
			if !errors.Is(err, testCase.want) {
				t.Fatalf("GetFacilityOverview() error = %v, want %v", err, testCase.want)
			}
		})
	}
}
