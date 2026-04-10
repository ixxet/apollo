package presence

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/store"
	"github.com/ixxet/apollo/internal/visits"
)

const recentVisitLimit = 10

const (
	StatusPresent    = "present"
	StatusNotPresent = "not_present"

	StreakStatusActive     = "active"
	StreakStatusInactive   = "inactive"
	StreakStatusNotStarted = "not_started"

	TapLinkStatusLinked = "linked"
)

type VisitRecorder interface {
	RecordArrival(ctx context.Context, input visits.ArrivalInput) (visits.Result, error)
	RecordDeparture(ctx context.Context, input visits.DepartureInput) (visits.Result, error)
}

type Summary struct {
	Facilities []FacilitySummary `json:"facilities"`
}

type FacilitySummary struct {
	FacilityKey  string         `json:"facility_key"`
	Status       string         `json:"status"`
	Current      *Visit         `json:"current,omitempty"`
	RecentVisits []Visit        `json:"recent_visits"`
	Streak       FacilityStreak `json:"streak"`
}

type Visit struct {
	ID          uuid.UUID  `json:"id"`
	FacilityKey string     `json:"facility_key"`
	ZoneKey     *string    `json:"zone_key,omitempty"`
	ArrivedAt   time.Time  `json:"arrived_at"`
	DepartedAt  *time.Time `json:"departed_at,omitempty"`
	TapLink     TapLink    `json:"tap_link"`
}

type TapLink struct {
	Status   string    `json:"status"`
	LinkedAt time.Time `json:"linked_at"`
}

type FacilityStreak struct {
	Status          string       `json:"status"`
	CurrentCount    int          `json:"current_count"`
	CurrentStartDay *string      `json:"current_start_day,omitempty"`
	LastCreditedDay *string      `json:"last_credited_day,omitempty"`
	LatestEvent     *StreakEvent `json:"latest_event,omitempty"`
}

type StreakEvent struct {
	Kind       string    `json:"kind"`
	CountAfter int       `json:"count_after"`
	EventDay   string    `json:"event_day"`
	VisitID    uuid.UUID `json:"visit_id"`
}

type Clock func() time.Time

type Store interface {
	ListLinkedVisitsByUserID(ctx context.Context, userID uuid.UUID) ([]LinkedVisit, error)
	ListFacilityStreaksByUserID(ctx context.Context, userID uuid.UUID) ([]store.ApolloMemberPresenceStreak, error)
	ListLatestFacilityStreakEventsByUserID(ctx context.Context, userID uuid.UUID) ([]store.ApolloMemberPresenceStreakEvent, error)
	EnsureLinkedVisitAndCredit(ctx context.Context, visit store.ApolloVisit, tagHash string, now time.Time) error
}

type Service struct {
	repository Store
	visits     VisitRecorder
	now        Clock
}

func NewService(repository Store, visitRecorder VisitRecorder) *Service {
	return &Service{
		repository: repository,
		visits:     visitRecorder,
		now:        time.Now,
	}
}

func (s *Service) RecordArrival(ctx context.Context, input visits.ArrivalInput) (visits.Result, error) {
	result, err := s.visits.RecordArrival(ctx, input)
	if err != nil {
		return result, err
	}
	if result.Visit == nil {
		return result, nil
	}

	switch result.Outcome {
	case visits.OutcomeCreated, visits.OutcomeDuplicate, visits.OutcomeAlreadyOpen:
		if err := s.repository.EnsureLinkedVisitAndCredit(ctx, *result.Visit, input.ExternalIdentityHash, s.now().UTC()); err != nil {
			return result, err
		}
	}

	return result, nil
}

func (s *Service) RecordDeparture(ctx context.Context, input visits.DepartureInput) (visits.Result, error) {
	return s.visits.RecordDeparture(ctx, input)
}

func (s *Service) GetSummary(ctx context.Context, userID uuid.UUID) (Summary, error) {
	linkedVisits, err := s.repository.ListLinkedVisitsByUserID(ctx, userID)
	if err != nil {
		return Summary{}, err
	}
	streaks, err := s.repository.ListFacilityStreaksByUserID(ctx, userID)
	if err != nil {
		return Summary{}, err
	}
	latestEvents, err := s.repository.ListLatestFacilityStreakEventsByUserID(ctx, userID)
	if err != nil {
		return Summary{}, err
	}

	visitsByFacility := make(map[string][]LinkedVisit)
	facilityKeys := make(map[string]struct{})
	for _, visit := range linkedVisits {
		visitsByFacility[visit.Visit.FacilityKey] = append(visitsByFacility[visit.Visit.FacilityKey], visit)
		facilityKeys[visit.Visit.FacilityKey] = struct{}{}
	}

	streaksByFacility := make(map[string]store.ApolloMemberPresenceStreak, len(streaks))
	for _, streak := range streaks {
		streaksByFacility[streak.FacilityKey] = streak
		facilityKeys[streak.FacilityKey] = struct{}{}
	}
	latestEventsByFacility := make(map[string]store.ApolloMemberPresenceStreakEvent, len(latestEvents))
	for _, event := range latestEvents {
		latestEventsByFacility[event.FacilityKey] = event
		facilityKeys[event.FacilityKey] = struct{}{}
	}

	orderedFacilities := make([]string, 0, len(facilityKeys))
	for facilityKey := range facilityKeys {
		orderedFacilities = append(orderedFacilities, facilityKey)
	}
	sort.Strings(orderedFacilities)

	summary := Summary{
		Facilities: make([]FacilitySummary, 0, len(orderedFacilities)),
	}
	for _, facilityKey := range orderedFacilities {
		facilityVisits := visitsByFacility[facilityKey]
		current, status, err := currentFacilityVisit(facilityVisits)
		if err != nil {
			return Summary{}, err
		}

		recentVisits := make([]Visit, 0, minInt(len(facilityVisits), recentVisitLimit))
		for index, visit := range facilityVisits {
			if index >= recentVisitLimit {
				break
			}
			recentVisits = append(recentVisits, buildVisit(visit))
		}

		facilitySummary := FacilitySummary{
			FacilityKey:  facilityKey,
			Status:       status,
			Current:      current,
			RecentVisits: recentVisits,
			Streak:       buildFacilityStreak(streaksByFacility[facilityKey], latestEventsByFacility[facilityKey], s.now().UTC()),
		}
		summary.Facilities = append(summary.Facilities, facilitySummary)
	}

	return summary, nil
}

func currentFacilityVisit(visitsInFacility []LinkedVisit) (*Visit, string, error) {
	openVisits := make([]LinkedVisit, 0, 1)
	for _, visit := range visitsInFacility {
		if !visit.Visit.DepartedAt.Valid {
			openVisits = append(openVisits, visit)
		}
	}
	if len(openVisits) == 0 {
		return nil, StatusNotPresent, nil
	}
	if len(openVisits) > 1 {
		return nil, "", fmt.Errorf("facility %q has %d open linked visits", visitsInFacility[0].Visit.FacilityKey, len(openVisits))
	}

	current := buildVisit(openVisits[0])
	return &current, StatusPresent, nil
}

func buildVisit(visit LinkedVisit) Visit {
	var departedAt *time.Time
	if visit.Visit.DepartedAt.Valid {
		value := visit.Visit.DepartedAt.Time.UTC()
		departedAt = &value
	}

	return Visit{
		ID:          visit.Visit.ID,
		FacilityKey: visit.Visit.FacilityKey,
		ZoneKey:     visit.Visit.ZoneKey,
		ArrivedAt:   visit.Visit.ArrivedAt.Time.UTC(),
		DepartedAt:  departedAt,
		TapLink: TapLink{
			Status:   TapLinkStatusLinked,
			LinkedAt: visit.TapLink.LinkedAt.Time.UTC(),
		},
	}
}

func buildFacilityStreak(streak store.ApolloMemberPresenceStreak, latestEvent store.ApolloMemberPresenceStreakEvent, now time.Time) FacilityStreak {
	if streak.UserID == uuid.Nil {
		return FacilityStreak{
			Status:       StreakStatusNotStarted,
			CurrentCount: 0,
		}
	}

	status := StreakStatusInactive
	if streak.LastCreditedDay.Valid {
		lastCreditedDay := normalizeUTCDay(streak.LastCreditedDay.Time)
		if !normalizeUTCDay(now).After(lastCreditedDay.AddDate(0, 0, 1)) {
			status = StreakStatusActive
		}
	}

	return FacilityStreak{
		Status:          status,
		CurrentCount:    int(streak.CurrentCount),
		CurrentStartDay: formatDatePointer(streak.CurrentStartDay.Time, streak.CurrentStartDay.Valid),
		LastCreditedDay: formatDatePointer(streak.LastCreditedDay.Time, streak.LastCreditedDay.Valid),
		LatestEvent:     buildLatestEvent(latestEvent),
	}
}

func buildLatestEvent(event store.ApolloMemberPresenceStreakEvent) *StreakEvent {
	if event.ID == uuid.Nil {
		return nil
	}

	return &StreakEvent{
		Kind:       event.EventKind,
		CountAfter: int(event.CountAfter),
		EventDay:   event.StreakDay.Time.UTC().Format("2006-01-02"),
		VisitID:    event.VisitID,
	}
}

func formatDatePointer(value time.Time, valid bool) *string {
	if !valid {
		return nil
	}
	formatted := value.UTC().Format("2006-01-02")
	return &formatted
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}
