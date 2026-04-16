package booking

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ixxet/apollo/internal/schedule"
	"github.com/ixxet/apollo/internal/store"
)

var (
	ErrRequestNotFound            = errors.New("booking request not found")
	ErrFacilityRequired           = errors.New("facility_key is required")
	ErrWindowInvalid              = errors.New("booking request window is invalid")
	ErrContactNameRequired        = errors.New("contact_name is required")
	ErrContactChannelRequired     = errors.New("contact_email or contact_phone is required")
	ErrContactEmailInvalid        = errors.New("contact_email is invalid")
	ErrAttendeeCountInvalid       = errors.New("attendee_count must be positive")
	ErrExpectedVersionRequired    = errors.New("expected_version must be positive")
	ErrRequestVersionStale        = errors.New("booking request version is stale")
	ErrRequestTransitionInvalid   = errors.New("booking request transition is invalid")
	ErrRequestActorRequired       = errors.New("booking actor attribution is required")
	ErrRequestTrustedSurface      = errors.New("booking actor trusted surface is required")
	ErrScheduleServiceUnavailable = errors.New("schedule service is unavailable for booking requests")
	ErrLinkedScheduleBlockDrift   = errors.New("linked schedule block drifted from booking request")
)

type Service struct {
	repository *Repository
	schedule   *schedule.Service
	now        func() time.Time
}

func NewService(repository *Repository, scheduleService *schedule.Service) *Service {
	return &Service{
		repository: repository,
		schedule:   scheduleService,
		now:        time.Now,
	}
}

func (s *Service) ListRequests(ctx context.Context, facilityKey string) ([]Request, error) {
	facilityKey = strings.TrimSpace(facilityKey)
	rows, err := s.repository.List(ctx, facilityKey)
	if err != nil {
		return nil, err
	}

	result := make([]Request, 0, len(rows))
	for _, row := range rows {
		request, err := s.requestFromStore(ctx, store.New(s.repository.db), row)
		if err != nil {
			return nil, err
		}
		result = append(result, request)
	}
	return result, nil
}

func (s *Service) GetRequest(ctx context.Context, requestID uuid.UUID) (Request, error) {
	row, err := s.repository.Get(ctx, requestID)
	if err != nil {
		return Request{}, err
	}
	if row == nil {
		return Request{}, ErrRequestNotFound
	}
	return s.requestFromStore(ctx, store.New(s.repository.db), *row)
}

func (s *Service) CreateRequest(ctx context.Context, actor StaffActor, input RequestInput) (Request, error) {
	if err := validateStaffActor(actor); err != nil {
		return Request{}, err
	}

	resolved, err := validateRequestInput(input)
	if err != nil {
		return Request{}, err
	}
	blockInput := scheduleBlockInputForRequest(resolved)

	return withBookingQueriesTx(ctx, s.repository.db, func(queries *store.Queries) (Request, error) {
		if s.schedule == nil {
			return Request{}, ErrScheduleServiceUnavailable
		}
		if _, err := s.schedule.PreviewBlockWithQueries(ctx, queries, blockInput); err != nil {
			return Request{}, err
		}

		now := s.now().UTC()
		attendeeCount := int32PtrFromIntPtr(resolved.AttendeeCount)
		row, err := queries.CreateBookingRequest(ctx, store.CreateBookingRequestParams{
			FacilityKey:                resolved.FacilityKey,
			ZoneKey:                    resolved.ZoneKey,
			ResourceKey:                resolved.ResourceKey,
			Scope:                      requestScope(resolved),
			RequestedStartAt:           pgTimestamptzFromTime(resolved.RequestedStartAt),
			RequestedEndAt:             pgTimestamptzFromTime(resolved.RequestedEndAt),
			ContactName:                resolved.ContactName,
			ContactEmail:               resolved.ContactEmail,
			ContactPhone:               resolved.ContactPhone,
			Organization:               resolved.Organization,
			Purpose:                    resolved.Purpose,
			AttendeeCount:              attendeeCount,
			InternalNotes:              resolved.InternalNotes,
			CreatedByUserID:            actor.UserID,
			CreatedBySessionID:         actor.SessionID,
			CreatedByRole:              string(actor.Role),
			CreatedByCapability:        string(actor.Capability),
			CreatedTrustedSurfaceKey:   actor.TrustedSurfaceKey,
			CreatedTrustedSurfaceLabel: nullableStringPtr(actor.TrustedSurfaceLabel),
			CreatedAt:                  pgTimestamptzFromTime(now),
		})
		if err != nil {
			return Request{}, err
		}
		return s.requestFromStore(ctx, queries, row)
	})
}

func (s *Service) StartReview(ctx context.Context, actor StaffActor, requestID uuid.UUID, input TransitionInput) (Request, error) {
	return s.transition(ctx, actor, requestID, input, StatusUnderReview, map[string]struct{}{
		StatusRequested:    {},
		StatusNeedsChanges: {},
	})
}

func (s *Service) NeedsChanges(ctx context.Context, actor StaffActor, requestID uuid.UUID, input TransitionInput) (Request, error) {
	return s.transition(ctx, actor, requestID, input, StatusNeedsChanges, map[string]struct{}{
		StatusRequested:   {},
		StatusUnderReview: {},
	})
}

func (s *Service) Reject(ctx context.Context, actor StaffActor, requestID uuid.UUID, input TransitionInput) (Request, error) {
	return s.transition(ctx, actor, requestID, input, StatusRejected, map[string]struct{}{
		StatusRequested:    {},
		StatusUnderReview:  {},
		StatusNeedsChanges: {},
	})
}

func (s *Service) Cancel(ctx context.Context, actor StaffActor, requestID uuid.UUID, input TransitionInput) (Request, error) {
	if err := validateStaffActor(actor); err != nil {
		return Request{}, err
	}
	if input.ExpectedVersion <= 0 {
		return Request{}, ErrExpectedVersionRequired
	}

	return withBookingQueriesTx(ctx, s.repository.db, func(queries *store.Queries) (Request, error) {
		current, err := s.requireCurrentForTransition(ctx, queries, requestID, input.ExpectedVersion)
		if err != nil {
			return Request{}, err
		}

		switch current.Status {
		case StatusRequested, StatusUnderReview, StatusNeedsChanges:
			row, err := s.updateStatus(ctx, queries, actor, current, StatusCancelled, nil, input.InternalNotes)
			if err != nil {
				return Request{}, err
			}
			return s.requestFromStore(ctx, queries, row)
		case StatusApproved:
			if s.schedule == nil {
				return Request{}, ErrScheduleServiceUnavailable
			}
			blockID := uuidPtrFromPgUUID(current.ScheduleBlockID)
			if blockID == nil {
				return Request{}, ErrLinkedScheduleBlockDrift
			}

			cancelledBlock, err := s.schedule.CancelLinkedReservationWithQueries(
				ctx,
				queries,
				scheduleActorFromBookingActor(actor),
				*blockID,
				reservationExpectationForStore(current),
			)
			if isLinkedScheduleDrift(err) {
				return Request{}, ErrLinkedScheduleBlockDrift
			}
			if err != nil {
				return Request{}, err
			}

			row, err := s.updateStatus(ctx, queries, actor, current, StatusCancelled, &cancelledBlock.ID, input.InternalNotes)
			if err != nil {
				return Request{}, err
			}
			return s.requestFromStore(ctx, queries, row)
		default:
			return Request{}, ErrRequestTransitionInvalid
		}
	})
}

func (s *Service) Approve(ctx context.Context, actor StaffActor, requestID uuid.UUID, input TransitionInput) (Request, error) {
	if err := validateStaffActor(actor); err != nil {
		return Request{}, err
	}
	if input.ExpectedVersion <= 0 {
		return Request{}, ErrExpectedVersionRequired
	}

	return withBookingQueriesTx(ctx, s.repository.db, func(queries *store.Queries) (Request, error) {
		current, err := s.requireCurrentForTransition(ctx, queries, requestID, input.ExpectedVersion)
		if err != nil {
			return Request{}, err
		}
		if current.Status != StatusRequested && current.Status != StatusUnderReview {
			return Request{}, ErrRequestTransitionInvalid
		}
		if s.schedule == nil {
			return Request{}, ErrScheduleServiceUnavailable
		}

		block, err := s.schedule.CreateBlockWithQueries(ctx, queries, schedule.StaffActor{
			UserID:              actor.UserID,
			Role:                actor.Role,
			SessionID:           actor.SessionID,
			Capability:          actor.Capability,
			TrustedSurfaceKey:   actor.TrustedSurfaceKey,
			TrustedSurfaceLabel: actor.TrustedSurfaceLabel,
		}, scheduleBlockInputForStore(current))
		if err != nil {
			return Request{}, err
		}

		row, err := s.updateStatus(ctx, queries, actor, current, StatusApproved, &block.ID, input.InternalNotes)
		if err != nil {
			return Request{}, err
		}
		return s.requestFromStore(ctx, queries, row)
	})
}

func (s *Service) transition(ctx context.Context, actor StaffActor, requestID uuid.UUID, input TransitionInput, nextStatus string, allowed map[string]struct{}) (Request, error) {
	if err := validateStaffActor(actor); err != nil {
		return Request{}, err
	}
	if input.ExpectedVersion <= 0 {
		return Request{}, ErrExpectedVersionRequired
	}

	return withBookingQueriesTx(ctx, s.repository.db, func(queries *store.Queries) (Request, error) {
		current, err := s.requireCurrentForTransition(ctx, queries, requestID, input.ExpectedVersion)
		if err != nil {
			return Request{}, err
		}
		if _, ok := allowed[current.Status]; !ok {
			return Request{}, ErrRequestTransitionInvalid
		}

		row, err := s.updateStatus(ctx, queries, actor, current, nextStatus, nil, input.InternalNotes)
		if err != nil {
			return Request{}, err
		}
		return s.requestFromStore(ctx, queries, row)
	})
}

func (s *Service) requireCurrentForTransition(ctx context.Context, queries *store.Queries, requestID uuid.UUID, expectedVersion int) (store.ApolloBookingRequest, error) {
	current, err := queries.GetBookingRequestByIDForUpdate(ctx, requestID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ApolloBookingRequest{}, ErrRequestNotFound
	}
	if err != nil {
		return store.ApolloBookingRequest{}, err
	}
	if current.Version != int32(expectedVersion) {
		return store.ApolloBookingRequest{}, ErrRequestVersionStale
	}
	return current, nil
}

func (s *Service) updateStatus(ctx context.Context, queries *store.Queries, actor StaffActor, current store.ApolloBookingRequest, nextStatus string, scheduleBlockID *uuid.UUID, internalNotes *string) (store.ApolloBookingRequest, error) {
	row, err := queries.UpdateBookingRequestStatus(ctx, store.UpdateBookingRequestStatusParams{
		ID:                         current.ID,
		Status:                     nextStatus,
		ScheduleBlockID:            pgUUIDFromPtr(scheduleBlockID),
		InternalNotes:              trimOptionalString(internalNotes),
		UpdatedByUserID:            actor.UserID,
		UpdatedBySessionID:         actor.SessionID,
		UpdatedByRole:              string(actor.Role),
		UpdatedByCapability:        string(actor.Capability),
		UpdatedTrustedSurfaceKey:   actor.TrustedSurfaceKey,
		UpdatedTrustedSurfaceLabel: nullableStringPtr(actor.TrustedSurfaceLabel),
		UpdatedAt:                  pgTimestamptzFromTime(s.now().UTC()),
		Version:                    current.Version,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ApolloBookingRequest{}, ErrRequestVersionStale
	}
	if err != nil {
		return store.ApolloBookingRequest{}, err
	}
	return row, nil
}

func (s *Service) requestFromStore(ctx context.Context, queries *store.Queries, row store.ApolloBookingRequest) (Request, error) {
	availability, err := s.availabilityForRequest(ctx, queries, row)
	if err != nil {
		return Request{}, err
	}
	return requestFromStore(row, availability), nil
}

func (s *Service) availabilityForRequest(ctx context.Context, queries *store.Queries, row store.ApolloBookingRequest) (AvailabilityDecision, error) {
	switch row.Status {
	case StatusApproved:
		return AvailabilityDecision{Status: AvailabilityReserved, Available: false}, nil
	case StatusRejected, StatusCancelled:
		return AvailabilityDecision{Status: AvailabilityNotApplicable, Available: false}, nil
	}
	if s.schedule == nil {
		return AvailabilityDecision{}, ErrScheduleServiceUnavailable
	}

	decision, err := s.schedule.PreviewBlockWithQueries(ctx, queries, scheduleBlockInputForStore(row))
	if err != nil {
		return AvailabilityDecision{}, err
	}
	status := AvailabilityAvailable
	if !decision.Available {
		status = AvailabilityConflict
	}
	return AvailabilityDecision{
		Status:    status,
		Available: decision.Available,
		Conflicts: decision.Conflicts,
	}, nil
}

func validateStaffActor(actor StaffActor) error {
	if actor.UserID == uuid.Nil || actor.SessionID == uuid.Nil {
		return ErrRequestActorRequired
	}
	if strings.TrimSpace(actor.TrustedSurfaceKey) == "" {
		return ErrRequestTrustedSurface
	}
	return nil
}

func validateRequestInput(input RequestInput) (RequestInput, error) {
	input.FacilityKey = strings.TrimSpace(input.FacilityKey)
	input.ContactName = strings.TrimSpace(input.ContactName)
	input.ZoneKey = trimOptionalString(input.ZoneKey)
	input.ResourceKey = trimOptionalString(input.ResourceKey)
	input.ContactEmail = trimOptionalString(input.ContactEmail)
	input.ContactPhone = trimOptionalString(input.ContactPhone)
	input.Organization = trimOptionalString(input.Organization)
	input.Purpose = trimOptionalString(input.Purpose)
	input.InternalNotes = trimOptionalString(input.InternalNotes)

	switch {
	case input.FacilityKey == "":
		return RequestInput{}, ErrFacilityRequired
	case input.RequestedStartAt.IsZero() || input.RequestedEndAt.IsZero() || !input.RequestedStartAt.Before(input.RequestedEndAt):
		return RequestInput{}, ErrWindowInvalid
	case input.ContactName == "":
		return RequestInput{}, ErrContactNameRequired
	case input.ContactEmail == nil && input.ContactPhone == nil:
		return RequestInput{}, ErrContactChannelRequired
	case input.AttendeeCount != nil && *input.AttendeeCount <= 0:
		return RequestInput{}, ErrAttendeeCountInvalid
	}
	if input.ContactEmail != nil {
		if _, err := mail.ParseAddress(*input.ContactEmail); err != nil {
			return RequestInput{}, ErrContactEmailInvalid
		}
	}
	return input, nil
}

func requestScope(input RequestInput) string {
	if input.ResourceKey != nil {
		return schedule.ScopeResource
	}
	if input.ZoneKey != nil {
		return schedule.ScopeZone
	}
	return schedule.ScopeFacility
}

func scheduleBlockInputForRequest(input RequestInput) schedule.BlockInput {
	return schedule.BlockInput{
		FacilityKey: input.FacilityKey,
		ZoneKey:     input.ZoneKey,
		ResourceKey: input.ResourceKey,
		Scope:       requestScope(input),
		Kind:        schedule.KindReservation,
		Effect:      schedule.EffectHardReserve,
		Visibility:  schedule.VisibilityInternal,
		OneOff: &schedule.OneOffInput{
			StartsAt: input.RequestedStartAt.UTC(),
			EndsAt:   input.RequestedEndAt.UTC(),
		},
	}
}

func scheduleBlockInputForStore(row store.ApolloBookingRequest) schedule.BlockInput {
	return schedule.BlockInput{
		FacilityKey: row.FacilityKey,
		ZoneKey:     row.ZoneKey,
		ResourceKey: row.ResourceKey,
		Scope:       row.Scope,
		Kind:        schedule.KindReservation,
		Effect:      schedule.EffectHardReserve,
		Visibility:  schedule.VisibilityInternal,
		OneOff: &schedule.OneOffInput{
			StartsAt: row.RequestedStartAt.Time.UTC(),
			EndsAt:   row.RequestedEndAt.Time.UTC(),
		},
	}
}

func reservationExpectationForStore(row store.ApolloBookingRequest) schedule.ReservationCancellationExpectation {
	return schedule.ReservationCancellationExpectation{
		FacilityKey: row.FacilityKey,
		ZoneKey:     row.ZoneKey,
		ResourceKey: row.ResourceKey,
		Scope:       row.Scope,
		StartsAt:    row.RequestedStartAt.Time.UTC(),
		EndsAt:      row.RequestedEndAt.Time.UTC(),
	}
}

func scheduleActorFromBookingActor(actor StaffActor) schedule.StaffActor {
	return schedule.StaffActor{
		UserID:              actor.UserID,
		Role:                actor.Role,
		SessionID:           actor.SessionID,
		Capability:          actor.Capability,
		TrustedSurfaceKey:   actor.TrustedSurfaceKey,
		TrustedSurfaceLabel: actor.TrustedSurfaceLabel,
	}
}

func isLinkedScheduleDrift(err error) bool {
	return errors.Is(err, schedule.ErrBlockNotFound) ||
		errors.Is(err, schedule.ErrBlockCancelled) ||
		errors.Is(err, schedule.ErrBlockReservationMismatch) ||
		errors.Is(err, schedule.ErrBlockVersionStale)
}

func requestFromStore(row store.ApolloBookingRequest, availability AvailabilityDecision) Request {
	return Request{
		ID:                         row.ID,
		FacilityKey:                row.FacilityKey,
		ZoneKey:                    row.ZoneKey,
		ResourceKey:                row.ResourceKey,
		Scope:                      row.Scope,
		RequestedStartAt:           row.RequestedStartAt.Time.UTC(),
		RequestedEndAt:             row.RequestedEndAt.Time.UTC(),
		ContactName:                row.ContactName,
		ContactEmail:               row.ContactEmail,
		ContactPhone:               row.ContactPhone,
		Organization:               row.Organization,
		Purpose:                    row.Purpose,
		AttendeeCount:              intPtrFromInt32Ptr(row.AttendeeCount),
		InternalNotes:              row.InternalNotes,
		Status:                     row.Status,
		Version:                    int(row.Version),
		ScheduleBlockID:            uuidPtrFromPgUUID(row.ScheduleBlockID),
		Availability:               availability,
		CreatedByUserID:            row.CreatedByUserID,
		CreatedBySessionID:         row.CreatedBySessionID,
		CreatedByRole:              row.CreatedByRole,
		CreatedByCapability:        row.CreatedByCapability,
		CreatedTrustedSurfaceKey:   row.CreatedTrustedSurfaceKey,
		CreatedTrustedSurfaceLabel: row.CreatedTrustedSurfaceLabel,
		UpdatedByUserID:            row.UpdatedByUserID,
		UpdatedBySessionID:         row.UpdatedBySessionID,
		UpdatedByRole:              row.UpdatedByRole,
		UpdatedByCapability:        row.UpdatedByCapability,
		UpdatedTrustedSurfaceKey:   row.UpdatedTrustedSurfaceKey,
		UpdatedTrustedSurfaceLabel: row.UpdatedTrustedSurfaceLabel,
		CreatedAt:                  row.CreatedAt.Time.UTC(),
		UpdatedAt:                  row.UpdatedAt.Time.UTC(),
	}
}

func trimOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func nullableStringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func int32PtrFromIntPtr(value *int) *int32 {
	if value == nil {
		return nil
	}
	converted := int32(*value)
	return &converted
}

func intPtrFromInt32Ptr(value *int32) *int {
	if value == nil {
		return nil
	}
	converted := int(*value)
	return &converted
}

func pgTimestamptzFromTime(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

func pgUUIDFromPtr(value *uuid.UUID) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *value, Valid: true}
}

func uuidPtrFromPgUUID(value pgtype.UUID) *uuid.UUID {
	if !value.Valid {
		return nil
	}
	converted := uuid.UUID(value.Bytes)
	return &converted
}
