package ops

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/ixxet/apollo/internal/athena"
	"github.com/ixxet/apollo/internal/schedule"
)

const (
	DefaultAnalyticsMaxWindow = 7 * 24 * time.Hour
	maxScheduleOccurrences    = 50
)

var (
	ErrFacilityRequired = errors.New("facility_key is required")
	ErrWindowRequired   = errors.New("from and until are required")
	ErrWindowInvalid    = errors.New("ops overview window is invalid")
	ErrWindowTooLarge   = errors.New("ops overview window exceeds maximum range")
	ErrBucketInvalid    = errors.New("bucket_minutes must be greater than zero")
)

type ScheduleReader interface {
	GetCalendar(ctx context.Context, facilityKey string, window schedule.CalendarWindow) ([]schedule.Occurrence, error)
}

type OccupancyReader interface {
	CurrentOccupancy(ctx context.Context, facilityID string) (athena.OccupancySnapshot, error)
	OccupancyAnalytics(ctx context.Context, filter athena.AnalyticsFilter) (athena.AnalyticsReport, error)
}

type Service struct {
	scheduleReader     ScheduleReader
	occupancyReader    OccupancyReader
	analyticsMaxWindow time.Duration
}

type FacilityOverviewInput struct {
	FacilityKey   string
	From          time.Time
	Until         time.Time
	BucketMinutes int
}

type FacilityOverview struct {
	FacilityKey        string             `json:"facility_key"`
	Status             string             `json:"status"`
	Window             OverviewWindow     `json:"window"`
	CurrentOccupancy   CurrentOccupancy   `json:"current_occupancy"`
	OccupancyAnalytics OccupancyAnalytics `json:"occupancy_analytics"`
	ScheduleSummary    ScheduleSummary    `json:"schedule_summary"`
	SourceServices     SourceServices     `json:"source_services"`
}

type OverviewWindow struct {
	From  string `json:"from"`
	Until string `json:"until"`
}

type SourceServices struct {
	Schedule  string `json:"schedule"`
	Occupancy string `json:"occupancy"`
	Analytics string `json:"analytics"`
}

type CurrentOccupancy struct {
	FacilityID    string `json:"facility_id"`
	ZoneID        string `json:"zone_id,omitempty"`
	CurrentCount  int    `json:"current_count"`
	ObservedAt    string `json:"observed_at"`
	SourceService string `json:"source_service"`
}

type OccupancyAnalytics struct {
	FacilityID         string                    `json:"facility_id"`
	ZoneID             string                    `json:"zone_id,omitempty"`
	NodeID             string                    `json:"node_id,omitempty"`
	Since              string                    `json:"since"`
	Until              string                    `json:"until"`
	BucketMinutes      int                       `json:"bucket_minutes"`
	ObservationSummary athena.ObservationSummary `json:"observation_summary"`
	SessionSummary     athena.SessionSummary     `json:"session_summary"`
	FlowBuckets        []FlowBucket              `json:"flow_buckets"`
	NodeBreakdown      []athena.NodeBreakdown    `json:"node_breakdown"`
	SourceService      string                    `json:"source_service"`
}

type FlowBucket struct {
	StartedAt    string `json:"started_at"`
	EndedAt      string `json:"ended_at"`
	PassIn       int    `json:"pass_in"`
	PassOut      int    `json:"pass_out"`
	FailIn       int    `json:"fail_in"`
	FailOut      int    `json:"fail_out"`
	OccupancyEnd int    `json:"occupancy_end"`
}

type ScheduleSummary struct {
	OccurrenceCount         int                  `json:"occurrence_count"`
	ReturnedOccurrenceCount int                  `json:"returned_occurrence_count"`
	ByKind                  map[string]int       `json:"by_kind"`
	ByEffect                map[string]int       `json:"by_effect"`
	ByVisibility            map[string]int       `json:"by_visibility"`
	Occurrences             []ScheduleOccurrence `json:"occurrences"`
	SourceService           string               `json:"source_service"`
}

type ScheduleOccurrence struct {
	Scope          string  `json:"scope"`
	Kind           string  `json:"kind"`
	Effect         string  `json:"effect"`
	Visibility     string  `json:"visibility"`
	ZoneKey        *string `json:"zone_key,omitempty"`
	ResourceKey    *string `json:"resource_key,omitempty"`
	StartsAt       string  `json:"starts_at"`
	EndsAt         string  `json:"ends_at"`
	OccurrenceDate string  `json:"occurrence_date"`
}

func NewService(scheduleReader ScheduleReader, occupancyReader OccupancyReader, analyticsMaxWindow time.Duration) *Service {
	if analyticsMaxWindow <= 0 {
		analyticsMaxWindow = DefaultAnalyticsMaxWindow
	}

	return &Service{
		scheduleReader:     scheduleReader,
		occupancyReader:    occupancyReader,
		analyticsMaxWindow: analyticsMaxWindow,
	}
}

func (s *Service) GetFacilityOverview(ctx context.Context, input FacilityOverviewInput) (FacilityOverview, error) {
	resolved, err := s.validateInput(input)
	if err != nil {
		return FacilityOverview{}, err
	}

	window := schedule.CalendarWindow{From: resolved.From, Until: resolved.Until}
	occurrences, err := s.scheduleReader.GetCalendar(ctx, resolved.FacilityKey, window)
	if err != nil {
		return FacilityOverview{}, err
	}

	occupancy, err := s.occupancyReader.CurrentOccupancy(ctx, resolved.FacilityKey)
	if err != nil {
		return FacilityOverview{}, err
	}

	analytics, err := s.occupancyReader.OccupancyAnalytics(ctx, athena.AnalyticsFilter{
		FacilityID:    resolved.FacilityKey,
		Since:         resolved.From,
		Until:         resolved.Until,
		BucketMinutes: resolved.BucketMinutes,
		SessionLimit:  1,
	})
	if err != nil {
		return FacilityOverview{}, err
	}

	return FacilityOverview{
		FacilityKey: resolved.FacilityKey,
		Status:      "complete",
		Window: OverviewWindow{
			From:  resolved.From.UTC().Format(time.RFC3339),
			Until: resolved.Until.UTC().Format(time.RFC3339),
		},
		CurrentOccupancy:   currentOccupancyFromAthena(occupancy),
		OccupancyAnalytics: occupancyAnalyticsFromAthena(analytics),
		ScheduleSummary:    scheduleSummaryFromOccurrences(occurrences),
		SourceServices: SourceServices{
			Schedule:  "apollo",
			Occupancy: "athena",
			Analytics: "athena",
		},
	}, nil
}

func (s *Service) validateInput(input FacilityOverviewInput) (FacilityOverviewInput, error) {
	input.FacilityKey = strings.TrimSpace(input.FacilityKey)
	if input.FacilityKey == "" {
		return FacilityOverviewInput{}, ErrFacilityRequired
	}
	if input.From.IsZero() || input.Until.IsZero() {
		return FacilityOverviewInput{}, ErrWindowRequired
	}
	if !input.From.Before(input.Until) {
		return FacilityOverviewInput{}, ErrWindowInvalid
	}
	if input.Until.Sub(input.From) > s.analyticsMaxWindow {
		return FacilityOverviewInput{}, ErrWindowTooLarge
	}
	if input.BucketMinutes < 0 {
		return FacilityOverviewInput{}, ErrBucketInvalid
	}

	input.From = input.From.UTC()
	input.Until = input.Until.UTC()
	return input, nil
}

func currentOccupancyFromAthena(snapshot athena.OccupancySnapshot) CurrentOccupancy {
	return CurrentOccupancy{
		FacilityID:    snapshot.FacilityID,
		ZoneID:        snapshot.ZoneID,
		CurrentCount:  snapshot.CurrentCount,
		ObservedAt:    snapshot.ObservedAt.UTC().Format(time.RFC3339),
		SourceService: "athena",
	}
}

func occupancyAnalyticsFromAthena(report athena.AnalyticsReport) OccupancyAnalytics {
	buckets := make([]FlowBucket, 0, len(report.FlowBuckets))
	for _, bucket := range report.FlowBuckets {
		buckets = append(buckets, FlowBucket{
			StartedAt:    bucket.StartedAt.UTC().Format(time.RFC3339),
			EndedAt:      bucket.EndedAt.UTC().Format(time.RFC3339),
			PassIn:       bucket.PassIn,
			PassOut:      bucket.PassOut,
			FailIn:       bucket.FailIn,
			FailOut:      bucket.FailOut,
			OccupancyEnd: bucket.OccupancyEnd,
		})
	}

	return OccupancyAnalytics{
		FacilityID:         report.FacilityID,
		ZoneID:             report.ZoneID,
		NodeID:             report.NodeID,
		Since:              report.Since.UTC().Format(time.RFC3339),
		Until:              report.Until.UTC().Format(time.RFC3339),
		BucketMinutes:      report.BucketMinutes,
		ObservationSummary: report.ObservationSummary,
		SessionSummary:     report.SessionSummary,
		FlowBuckets:        buckets,
		NodeBreakdown:      append([]athena.NodeBreakdown(nil), report.NodeBreakdown...),
		SourceService:      "athena",
	}
}

func scheduleSummaryFromOccurrences(occurrences []schedule.Occurrence) ScheduleSummary {
	sorted := append([]schedule.Occurrence(nil), occurrences...)
	sort.Slice(sorted, func(i, j int) bool {
		if !sorted[i].StartsAt.Equal(sorted[j].StartsAt) {
			return sorted[i].StartsAt.Before(sorted[j].StartsAt)
		}
		if sorted[i].Kind != sorted[j].Kind {
			return sorted[i].Kind < sorted[j].Kind
		}
		return sorted[i].Scope < sorted[j].Scope
	})

	summary := ScheduleSummary{
		OccurrenceCount: len(sorted),
		ByKind:          make(map[string]int),
		ByEffect:        make(map[string]int),
		ByVisibility:    make(map[string]int),
		SourceService:   "apollo",
	}

	for index, occurrence := range sorted {
		summary.ByKind[occurrence.Kind]++
		summary.ByEffect[occurrence.Effect]++
		summary.ByVisibility[occurrence.Visibility]++

		if index >= maxScheduleOccurrences {
			continue
		}
		summary.Occurrences = append(summary.Occurrences, ScheduleOccurrence{
			Scope:          occurrence.Scope,
			Kind:           occurrence.Kind,
			Effect:         occurrence.Effect,
			Visibility:     occurrence.Visibility,
			ZoneKey:        occurrence.ZoneKey,
			ResourceKey:    occurrence.ResourceKey,
			StartsAt:       occurrence.StartsAt.UTC().Format(time.RFC3339),
			EndsAt:         occurrence.EndsAt.UTC().Format(time.RFC3339),
			OccurrenceDate: occurrence.OccurrenceDate,
		})
	}

	summary.ReturnedOccurrenceCount = len(summary.Occurrences)
	return summary
}
