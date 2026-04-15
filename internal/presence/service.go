package presence

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ixxet/apollo/internal/schedule"
	"github.com/ixxet/apollo/internal/store"
	"github.com/ixxet/apollo/internal/visits"
)

const recentVisitLimit = 10
const memberFacilityWindowDays = 14

const (
	StatusPresent    = "present"
	StatusNotPresent = "not_present"

	StreakStatusActive     = "active"
	StreakStatusInactive   = "inactive"
	StreakStatusNotStarted = "not_started"

	TapLinkStatusLinked = "linked"

	ClaimStatusActive   = "active"
	ClaimStatusInactive = "inactive"
)

var (
	ErrClaimTagHashRequired        = errors.New("presence claim tag_hash is required")
	ErrClaimTagHashInvalid         = errors.New("presence claim tag_hash is invalid")
	ErrClaimAlreadyActive          = errors.New("presence claim already exists")
	ErrClaimInactive               = errors.New("presence claim is inactive and cannot be reused")
	ErrClaimOwnedByAnotherMember   = errors.New("presence claim already belongs to another member")
	ErrMemberFacilitiesUnavailable = errors.New("member facility composition is unavailable")
	ErrMemberFacilityKeyRequired   = errors.New("facility_key is required")
	ErrMemberFacilityNotFound      = errors.New("member facility is unavailable")
	ErrMemberCalendarWindowInvalid = errors.New("member facility calendar window is invalid")
	ErrMemberCalendarWindowTooWide = errors.New("member facility calendar window exceeds the maximum range")
)

type VisitRecorder interface {
	RecordArrival(ctx context.Context, input visits.ArrivalInput) (visits.Result, error)
	RecordDeparture(ctx context.Context, input visits.DepartureInput) (visits.Result, error)
}

type FacilityCalendarReader interface {
	GetCalendar(ctx context.Context, facilityKey string, window schedule.CalendarWindow) ([]schedule.Occurrence, error)
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

type Claim struct {
	TagHash   string    `json:"tag_hash"`
	Label     *string   `json:"label,omitempty"`
	Status    string    `json:"status"`
	ClaimedAt time.Time `json:"claimed_at"`
}

type ClaimInput struct {
	TagHash string  `json:"tag_hash"`
	Label   *string `json:"label,omitempty"`
}

type MemberFacility struct {
	FacilityKey     string           `json:"facility_key"`
	SupportedSports []FacilitySport  `json:"supported_sports"`
	Hours           []FacilityWindow `json:"hours"`
	Closures        []FacilityWindow `json:"closures,omitempty"`
}

type MemberFacilityCalendar struct {
	FacilityKey string           `json:"facility_key"`
	From        time.Time        `json:"from"`
	Until       time.Time        `json:"until"`
	Hours       []FacilityWindow `json:"hours"`
	Closures    []FacilityWindow `json:"closures,omitempty"`
}

type FacilitySport struct {
	FacilityKey string `json:"-"`
	SportKey    string `json:"sport_key"`
	DisplayName string `json:"display_name"`
}

type FacilityWindow struct {
	StartsAt       time.Time `json:"starts_at"`
	EndsAt         time.Time `json:"ends_at"`
	OccurrenceDate string    `json:"occurrence_date"`
}

type Clock func() time.Time

type Store interface {
	ListLinkedVisitsByUserID(ctx context.Context, userID uuid.UUID) ([]LinkedVisit, error)
	ListFacilityStreaksByUserID(ctx context.Context, userID uuid.UUID) ([]store.ApolloMemberPresenceStreak, error)
	ListLatestFacilityStreakEventsByUserID(ctx context.Context, userID uuid.UUID) ([]store.ApolloMemberPresenceStreakEvent, error)
	ListClaimedTagsByUserID(ctx context.Context, userID uuid.UUID) ([]store.ApolloClaimedTag, error)
	GetClaimedTagByHash(ctx context.Context, tagHash string) (*store.ApolloClaimedTag, error)
	CreateClaimedTag(ctx context.Context, userID uuid.UUID, tagHash string, label *string) (store.ApolloClaimedTag, error)
	ListFacilityCatalogRefs(ctx context.Context) ([]string, error)
	ListFacilitySports(ctx context.Context) ([]FacilitySport, error)
	EnsureLinkedVisitAndCredit(ctx context.Context, visit store.ApolloVisit, tagHash string, now time.Time) error
}

type Option func(*Service)

type Service struct {
	repository Store
	visits     VisitRecorder
	calendar   FacilityCalendarReader
	now        Clock
}

func NewService(repository Store, visitRecorder VisitRecorder, options ...Option) *Service {
	service := &Service{
		repository: repository,
		visits:     visitRecorder,
		now:        time.Now,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func WithFacilityCalendar(calendar FacilityCalendarReader) Option {
	return func(service *Service) {
		service.calendar = calendar
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

func (s *Service) ListClaims(ctx context.Context, userID uuid.UUID) ([]Claim, error) {
	rows, err := s.repository.ListClaimedTagsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	claims := make([]Claim, 0, len(rows))
	for _, row := range rows {
		claims = append(claims, Claim{
			TagHash:   row.TagHash,
			Label:     row.Label,
			Status:    claimStatus(row.IsActive),
			ClaimedAt: row.ClaimedAt.Time.UTC(),
		})
	}

	return claims, nil
}

func (s *Service) Claim(ctx context.Context, userID uuid.UUID, input ClaimInput) (Claim, error) {
	tagHash, err := normalizeClaimTagHash(input.TagHash)
	if err != nil {
		return Claim{}, err
	}
	label := normalizeClaimLabel(input.Label)

	existing, err := s.repository.GetClaimedTagByHash(ctx, tagHash)
	if err != nil {
		return Claim{}, err
	}
	if existing != nil {
		return Claim{}, classifyExistingClaim(*existing, userID)
	}

	created, err := s.repository.CreateClaimedTag(ctx, userID, tagHash, label)
	if err != nil {
		if isUniqueViolation(err) {
			existing, lookupErr := s.repository.GetClaimedTagByHash(ctx, tagHash)
			if lookupErr != nil {
				return Claim{}, lookupErr
			}
			if existing != nil {
				return Claim{}, classifyExistingClaim(*existing, userID)
			}
		}
		return Claim{}, err
	}

	return Claim{
		TagHash:   created.TagHash,
		Label:     created.Label,
		Status:    claimStatus(created.IsActive),
		ClaimedAt: created.ClaimedAt.Time.UTC(),
	}, nil
}

func (s *Service) ListMemberFacilities(ctx context.Context, userID uuid.UUID) ([]MemberFacility, error) {
	if s.calendar == nil {
		return nil, ErrMemberFacilitiesUnavailable
	}

	orderedFacilities, err := s.listMemberFacilityKeys(ctx, userID)
	if err != nil {
		return nil, err
	}
	facilitySports, err := s.repository.ListFacilitySports(ctx)
	if err != nil {
		return nil, err
	}

	sportsByFacility := make(map[string][]FacilitySport)
	for _, facilitySport := range facilitySports {
		sportsByFacility[facilitySport.FacilityKey] = append(sportsByFacility[facilitySport.FacilityKey], facilitySport)
	}
	for facilityKey := range sportsByFacility {
		sort.Slice(sportsByFacility[facilityKey], func(i, j int) bool {
			left := sportsByFacility[facilityKey][i]
			right := sportsByFacility[facilityKey][j]
			if left.DisplayName != right.DisplayName {
				return left.DisplayName < right.DisplayName
			}
			return left.SportKey < right.SportKey
		})
	}

	now := s.now().UTC()
	window := schedule.CalendarWindow{
		From:  now,
		Until: now.AddDate(0, 0, memberFacilityWindowDays),
	}

	facilities := make([]MemberFacility, 0, len(orderedFacilities))
	for _, facilityKey := range orderedFacilities {
		occurrences, err := s.calendar.GetCalendar(ctx, facilityKey, window)
		if err != nil {
			return nil, err
		}

		memberFacility := MemberFacility{
			FacilityKey:     facilityKey,
			SupportedSports: cloneFacilitySports(sportsByFacility[facilityKey]),
			Hours:           []FacilityWindow{},
			Closures:        []FacilityWindow{},
		}

		memberFacility.Hours, memberFacility.Closures = projectMemberFacilityWindows(occurrences)

		facilities = append(facilities, memberFacility)
	}

	return facilities, nil
}

func (s *Service) GetMemberFacilityCalendar(ctx context.Context, userID uuid.UUID, facilityKey string, window schedule.CalendarWindow) (MemberFacilityCalendar, error) {
	if s.calendar == nil {
		return MemberFacilityCalendar{}, ErrMemberFacilitiesUnavailable
	}

	facilityKey = strings.TrimSpace(facilityKey)
	if facilityKey == "" {
		return MemberFacilityCalendar{}, ErrMemberFacilityKeyRequired
	}

	window, err := normalizeMemberCalendarWindow(window)
	if err != nil {
		return MemberFacilityCalendar{}, err
	}

	availableFacilityKeys, err := s.listMemberFacilityKeys(ctx, userID)
	if err != nil {
		return MemberFacilityCalendar{}, err
	}
	if !containsFacilityKey(availableFacilityKeys, facilityKey) {
		return MemberFacilityCalendar{}, ErrMemberFacilityNotFound
	}

	occurrences, err := s.calendar.GetCalendar(ctx, facilityKey, window)
	if err != nil {
		return MemberFacilityCalendar{}, err
	}

	hours, closures := projectMemberFacilityWindows(occurrences)
	return MemberFacilityCalendar{
		FacilityKey: facilityKey,
		From:        window.From,
		Until:       window.Until,
		Hours:       hours,
		Closures:    closures,
	}, nil
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

func claimStatus(active bool) string {
	if active {
		return ClaimStatusActive
	}
	return ClaimStatusInactive
}

func normalizeClaimTagHash(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", ErrClaimTagHashRequired
	}
	if len(trimmed) > 128 {
		return "", ErrClaimTagHashInvalid
	}
	for _, character := range trimmed {
		switch {
		case character >= 'a' && character <= 'z':
		case character >= 'A' && character <= 'Z':
		case character >= '0' && character <= '9':
		case character == '-', character == '_', character == ':':
		default:
			return "", ErrClaimTagHashInvalid
		}
	}
	return trimmed, nil
}

func normalizeClaimLabel(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func classifyExistingClaim(existing store.ApolloClaimedTag, userID uuid.UUID) error {
	if existing.UserID != userID {
		return ErrClaimOwnedByAnotherMember
	}
	if !existing.IsActive {
		return ErrClaimInactive
	}
	return ErrClaimAlreadyActive
}

func cloneFacilitySports(input []FacilitySport) []FacilitySport {
	if len(input) == 0 {
		return []FacilitySport{}
	}
	output := make([]FacilitySport, len(input))
	copy(output, input)
	return output
}

func (s *Service) listMemberFacilityKeys(ctx context.Context, userID uuid.UUID) ([]string, error) {
	catalogKeys, err := s.repository.ListFacilityCatalogRefs(ctx)
	if err != nil {
		return nil, err
	}
	linkedVisits, err := s.repository.ListLinkedVisitsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	facilityKeys := make(map[string]struct{}, len(catalogKeys)+len(linkedVisits))
	for _, facilityKey := range catalogKeys {
		facilityKeys[facilityKey] = struct{}{}
	}
	for _, visit := range linkedVisits {
		facilityKeys[visit.Visit.FacilityKey] = struct{}{}
	}

	orderedFacilities := make([]string, 0, len(facilityKeys))
	for facilityKey := range facilityKeys {
		orderedFacilities = append(orderedFacilities, facilityKey)
	}
	sort.Strings(orderedFacilities)

	return orderedFacilities, nil
}

func projectMemberFacilityWindows(occurrences []schedule.Occurrence) ([]FacilityWindow, []FacilityWindow) {
	hours := make([]FacilityWindow, 0, len(occurrences))
	closures := make([]FacilityWindow, 0, len(occurrences))

	for _, occurrence := range occurrences {
		if occurrence.Scope != schedule.ScopeFacility {
			continue
		}
		if occurrence.Status != schedule.StatusScheduled {
			continue
		}
		if occurrence.Visibility == schedule.VisibilityInternal {
			continue
		}

		window := FacilityWindow{
			StartsAt:       occurrence.StartsAt.UTC(),
			EndsAt:         occurrence.EndsAt.UTC(),
			OccurrenceDate: occurrence.OccurrenceDate,
		}

		switch occurrence.Kind {
		case schedule.KindOperatingHours:
			hours = append(hours, window)
		case schedule.KindClosure:
			closures = append(closures, window)
		}
	}

	return hours, closures
}

func normalizeMemberCalendarWindow(window schedule.CalendarWindow) (schedule.CalendarWindow, error) {
	normalized := schedule.CalendarWindow{
		From:  window.From.UTC(),
		Until: window.Until.UTC(),
	}
	if normalized.From.IsZero() || normalized.Until.IsZero() || !normalized.From.Before(normalized.Until) {
		return schedule.CalendarWindow{}, ErrMemberCalendarWindowInvalid
	}
	if normalized.Until.Sub(normalized.From) > time.Duration(memberFacilityWindowDays)*24*time.Hour {
		return schedule.CalendarWindow{}, ErrMemberCalendarWindowTooWide
	}
	return normalized, nil
}

func containsFacilityKey(facilityKeys []string, facilityKey string) bool {
	for _, candidate := range facilityKeys {
		if candidate == facilityKey {
			return true
		}
	}
	return false
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505"
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}
