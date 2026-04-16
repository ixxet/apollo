package booking

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/mail"
	"strconv"
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
	ErrContactPhoneInvalid        = errors.New("contact_phone is invalid")
	ErrAttendeeCountInvalid       = errors.New("attendee_count must be positive")
	ErrPublicOptionRequired       = errors.New("booking option_id is required")
	ErrPublicOptionNotFound       = errors.New("booking option is not available")
	ErrPublicFieldTooLong         = errors.New("booking request field is too long")
	ErrPublicWindowPast           = errors.New("booking request window must be in the future")
	ErrPublicWindowTooFar         = errors.New("booking request window is too far in the future")
	ErrPublicDurationInvalid      = errors.New("booking request duration is invalid")
	ErrIdempotencyKeyRequired     = errors.New("idempotency key is required")
	ErrIdempotencyConflict        = errors.New("idempotency key has already been used for a different request")
	ErrExpectedVersionRequired    = errors.New("expected_version must be positive")
	ErrRequestVersionStale        = errors.New("booking request version is stale")
	ErrRequestTransitionInvalid   = errors.New("booking request transition is invalid")
	ErrRequestActorRequired       = errors.New("booking actor attribution is required")
	ErrRequestTrustedSurface      = errors.New("booking actor trusted surface is required")
	ErrScheduleServiceUnavailable = errors.New("schedule service is unavailable for booking requests")
	ErrLinkedScheduleBlockDrift   = errors.New("linked schedule block drifted from booking request")
)

const (
	publicMaxContactNameLength  = 120
	publicMaxContactEmailLength = 254
	publicMaxContactPhoneLength = 32
	publicMaxOrganizationLength = 120
	publicMaxPurposeLength      = 1000
	publicMaxIdempotencyLength  = 200
	publicMaxAttendeeCount      = 500

	publicMinDuration = 30 * time.Minute
	publicMaxDuration = 8 * time.Hour
	publicMaxHorizon  = 180 * 24 * time.Hour
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

func (s *Service) ListPublicOptions(ctx context.Context) ([]PublicOption, error) {
	rows, err := store.New(s.repository.db).ListPublicBookingOptions(ctx)
	if err != nil {
		return nil, err
	}

	options := make([]PublicOption, 0, len(rows))
	for _, row := range rows {
		if row.PublicLabel == nil {
			continue
		}
		options = append(options, PublicOption{
			OptionID: row.PublicOptionID,
			Label:    *row.PublicLabel,
		})
	}
	return options, nil
}

func (s *Service) CreatePublicRequest(ctx context.Context, channel string, idempotencyKey string, input PublicRequestInput) (PublicReceipt, error) {
	channel = normalizePublicIntakeChannel(channel)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" || len(idempotencyKey) > publicMaxIdempotencyLength {
		return PublicReceipt{}, ErrIdempotencyKeyRequired
	}

	normalized, err := validatePublicRequestInput(input, s.now().UTC())
	if err != nil {
		return PublicReceipt{}, err
	}

	keyHash := sha256Hex(idempotencyKey)
	payloadHash := publicPayloadHash(normalized)
	receipt := PublicReceipt{Status: "received"}

	return withBookingQueriesTx(ctx, s.repository.db, func(queries *store.Queries) (PublicReceipt, error) {
		existing, err := queries.GetBookingRequestIdempotencyByKeyHashForUpdate(ctx, keyHash)
		if err == nil {
			if existing.PayloadHash == payloadHash {
				return receipt, nil
			}
			return PublicReceipt{}, ErrIdempotencyConflict
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return PublicReceipt{}, err
		}

		resource, err := queries.GetPublicBookingResourceByOptionID(ctx, normalized.OptionID)
		if errors.Is(err, pgx.ErrNoRows) {
			return PublicReceipt{}, ErrPublicOptionNotFound
		}
		if err != nil {
			return PublicReceipt{}, err
		}
		if resource.PublicLabel == nil {
			return PublicReceipt{}, ErrPublicOptionNotFound
		}

		resolved := RequestInput{
			FacilityKey:      resource.FacilityKey,
			ZoneKey:          resource.ZoneKey,
			ResourceKey:      &resource.ResourceKey,
			RequestedStartAt: normalized.RequestedStartAt,
			RequestedEndAt:   normalized.RequestedEndAt,
			ContactName:      normalized.ContactName,
			ContactEmail:     normalized.ContactEmail,
			ContactPhone:     normalized.ContactPhone,
			Organization:     normalized.Organization,
			Purpose:          normalized.Purpose,
			AttendeeCount:    normalized.AttendeeCount,
		}
		resolved, err = validateRequestInput(resolved)
		if err != nil {
			return PublicReceipt{}, err
		}
		if s.schedule == nil {
			return PublicReceipt{}, ErrScheduleServiceUnavailable
		}
		if _, err := s.schedule.PreviewBlockWithQueries(ctx, queries, scheduleBlockInputForRequest(resolved)); err != nil {
			return PublicReceipt{}, err
		}

		now := s.now().UTC()
		requestID := uuid.New()
		row, err := queries.CreatePublicBookingRequest(ctx, store.CreatePublicBookingRequestParams{
			ID:               requestID,
			FacilityKey:      resolved.FacilityKey,
			ZoneKey:          resolved.ZoneKey,
			ResourceKey:      resolved.ResourceKey,
			Scope:            requestScope(resolved),
			RequestedStartAt: pgTimestamptzFromTime(resolved.RequestedStartAt),
			RequestedEndAt:   pgTimestamptzFromTime(resolved.RequestedEndAt),
			ContactName:      resolved.ContactName,
			ContactEmail:     resolved.ContactEmail,
			ContactPhone:     resolved.ContactPhone,
			Organization:     resolved.Organization,
			Purpose:          resolved.Purpose,
			AttendeeCount:    int32PtrFromIntPtr(resolved.AttendeeCount),
			IntakeChannel:    channel,
			CreatedAt:        pgTimestamptzFromTime(now),
		})
		if err != nil {
			return PublicReceipt{}, err
		}

		if _, err := queries.CreateBookingRequestIdempotencyKey(ctx, store.CreateBookingRequestIdempotencyKeyParams{
			KeyHash:          keyHash,
			PayloadHash:      payloadHash,
			BookingRequestID: row.ID,
			CreatedAt:        pgTimestamptzFromTime(now),
		}); err != nil {
			return PublicReceipt{}, err
		}

		return receipt, nil
	})
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
			CreatedByUserID:            pgUUIDFromUUID(actor.UserID),
			CreatedBySessionID:         pgUUIDFromUUID(actor.SessionID),
			CreatedByRole:              requiredStringPtr(string(actor.Role)),
			CreatedByCapability:        requiredStringPtr(string(actor.Capability)),
			CreatedTrustedSurfaceKey:   requiredStringPtr(actor.TrustedSurfaceKey),
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
		UpdatedByUserID:            pgUUIDFromUUID(actor.UserID),
		UpdatedBySessionID:         pgUUIDFromUUID(actor.SessionID),
		UpdatedByRole:              requiredStringPtr(string(actor.Role)),
		UpdatedByCapability:        requiredStringPtr(string(actor.Capability)),
		UpdatedTrustedSurfaceKey:   requiredStringPtr(actor.TrustedSurfaceKey),
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

func validatePublicRequestInput(input PublicRequestInput, now time.Time) (PublicRequestInput, error) {
	input.ContactName = strings.TrimSpace(input.ContactName)
	input.ContactEmail = trimOptionalString(input.ContactEmail)
	input.ContactPhone = trimOptionalString(input.ContactPhone)
	input.Organization = trimOptionalString(input.Organization)
	input.Purpose = trimOptionalString(input.Purpose)
	input.RequestedStartAt = input.RequestedStartAt.UTC()
	input.RequestedEndAt = input.RequestedEndAt.UTC()

	switch {
	case input.OptionID == uuid.Nil:
		return PublicRequestInput{}, ErrPublicOptionRequired
	case input.RequestedStartAt.IsZero() || input.RequestedEndAt.IsZero() || !input.RequestedStartAt.Before(input.RequestedEndAt):
		return PublicRequestInput{}, ErrWindowInvalid
	case !input.RequestedStartAt.After(now):
		return PublicRequestInput{}, ErrPublicWindowPast
	case input.RequestedStartAt.After(now.Add(publicMaxHorizon)):
		return PublicRequestInput{}, ErrPublicWindowTooFar
	case input.RequestedEndAt.Sub(input.RequestedStartAt) < publicMinDuration || input.RequestedEndAt.Sub(input.RequestedStartAt) > publicMaxDuration:
		return PublicRequestInput{}, ErrPublicDurationInvalid
	case input.ContactName == "":
		return PublicRequestInput{}, ErrContactNameRequired
	case len(input.ContactName) > publicMaxContactNameLength:
		return PublicRequestInput{}, ErrPublicFieldTooLong
	case input.ContactEmail == nil && input.ContactPhone == nil:
		return PublicRequestInput{}, ErrContactChannelRequired
	case input.AttendeeCount != nil && (*input.AttendeeCount <= 0 || *input.AttendeeCount > publicMaxAttendeeCount):
		return PublicRequestInput{}, ErrAttendeeCountInvalid
	}

	if input.ContactEmail != nil {
		if len(*input.ContactEmail) > publicMaxContactEmailLength {
			return PublicRequestInput{}, ErrPublicFieldTooLong
		}
		if _, err := mail.ParseAddress(*input.ContactEmail); err != nil {
			return PublicRequestInput{}, ErrContactEmailInvalid
		}
	}
	if input.ContactPhone != nil {
		if len(*input.ContactPhone) > publicMaxContactPhoneLength {
			return PublicRequestInput{}, ErrPublicFieldTooLong
		}
		if !validPublicPhone(*input.ContactPhone) {
			return PublicRequestInput{}, ErrContactPhoneInvalid
		}
	}
	if input.Organization != nil && len(*input.Organization) > publicMaxOrganizationLength {
		return PublicRequestInput{}, ErrPublicFieldTooLong
	}
	if input.Purpose != nil && len(*input.Purpose) > publicMaxPurposeLength {
		return PublicRequestInput{}, ErrPublicFieldTooLong
	}

	return input, nil
}

func validPublicPhone(value string) bool {
	digits := 0
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '+' || r == '-' || r == '(' || r == ')' || r == ' ' || r == '.':
		default:
			return false
		}
	}
	return digits >= 7
}

func normalizePublicIntakeChannel(channel string) string {
	switch strings.TrimSpace(channel) {
	case IntakeChannelPublicWeb:
		return IntakeChannelPublicWeb
	case IntakeChannelPublicAPI:
		return IntakeChannelPublicAPI
	default:
		return IntakeChannelPublicAPI
	}
}

func publicPayloadHash(input PublicRequestInput) string {
	parts := []string{
		input.OptionID.String(),
		input.RequestedStartAt.UTC().Format(time.RFC3339Nano),
		input.RequestedEndAt.UTC().Format(time.RFC3339Nano),
		input.ContactName,
		optionalStringValue(input.ContactEmail),
		optionalStringValue(input.ContactPhone),
		optionalStringValue(input.Organization),
		optionalStringValue(input.Purpose),
		optionalIntValue(input.AttendeeCount),
	}
	return sha256Hex(strings.Join(parts, "\n"))
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
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
		RequestSource:              row.RequestSource,
		IntakeChannel:              row.IntakeChannel,
		Status:                     row.Status,
		Version:                    int(row.Version),
		ScheduleBlockID:            uuidPtrFromPgUUID(row.ScheduleBlockID),
		Availability:               availability,
		CreatedByUserID:            uuidPtrFromPgUUID(row.CreatedByUserID),
		CreatedBySessionID:         uuidPtrFromPgUUID(row.CreatedBySessionID),
		CreatedByRole:              row.CreatedByRole,
		CreatedByCapability:        row.CreatedByCapability,
		CreatedTrustedSurfaceKey:   row.CreatedTrustedSurfaceKey,
		CreatedTrustedSurfaceLabel: row.CreatedTrustedSurfaceLabel,
		UpdatedByUserID:            uuidPtrFromPgUUID(row.UpdatedByUserID),
		UpdatedBySessionID:         uuidPtrFromPgUUID(row.UpdatedBySessionID),
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

func requiredStringPtr(value string) *string {
	return &value
}

func optionalStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalIntValue(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
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

func pgUUIDFromUUID(value uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: value, Valid: true}
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
