package schedule

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ixxet/apollo/internal/store"
)

var (
	ErrResourceNotFound            = errors.New("schedule resource not found")
	ErrResourceKeyRequired         = errors.New("resource_key is required")
	ErrResourceTypeRequired        = errors.New("resource_type is required")
	ErrResourceDisplayNameRequired = errors.New("display_name is required")
	ErrResourceFacilityRequired    = errors.New("facility_key is required")
	ErrResourceFacilityInvalid     = errors.New("facility_key is invalid")
	ErrResourceZoneInvalid         = errors.New("zone_key is invalid for facility")
	ErrResourceEdgeInvalid         = errors.New("resource edge is invalid")
	ErrResourceEdgeSelfReference   = errors.New("resource edge cannot point to itself")
	ErrResourceEdgeCycle           = errors.New("resource edge would create a cycle")
	ErrBlockNotFound               = errors.New("schedule block not found")
	ErrBlockVersionStale           = errors.New("schedule block version is stale")
	ErrBlockScopeInvalid           = errors.New("schedule block scope is invalid")
	ErrBlockShapeInvalid           = errors.New("schedule block shape is invalid")
	ErrBlockKindInvalid            = errors.New("schedule block kind is invalid")
	ErrBlockEffectInvalid          = errors.New("schedule block effect is invalid")
	ErrBlockVisibilityInvalid      = errors.New("schedule block visibility is invalid")
	ErrBlockTimezoneRequired       = errors.New("weekly recurrence requires timezone")
	ErrBlockRecurrenceInvalid      = errors.New("weekly recurrence is invalid")
	ErrBlockDateWindowInvalid      = errors.New("schedule window is invalid")
	ErrBlockDateWindowTooLarge     = errors.New("schedule window exceeds the maximum range")
	ErrBlockConflictRejected       = errors.New("schedule block conflicts with effective inventory")
	ErrBlockResourceNotClaimable   = errors.New("resource is not active and bookable for inventory claims")
	ErrBlockClaimableScopeEmpty    = errors.New("schedule block requires at least one active and bookable resource in scope")
	ErrBlockOperatingHoursOverlap  = errors.New("operating hours overlap on the same scope")
	ErrBlockCancelled              = errors.New("schedule block is cancelled")
	ErrBlockReservationMismatch    = errors.New("schedule reservation does not match expected booking request")
	ErrExceptionDateRequired       = errors.New("exception_date is required")
	ErrExceptionNotAllowed         = errors.New("date exception is only supported for weekly recurring blocks")
	ErrExceptionWindowInvalid      = errors.New("exception_date must fall within the block recurrence window")
	ErrActorAttributionRequired    = errors.New("schedule actor attribution is required")
	ErrActorTrustedSurfaceMissing  = errors.New("schedule actor trusted surface is required")
)

type Service struct {
	repository *Repository
	now        func() time.Time
}

func NewService(repository *Repository) *Service {
	return &Service{
		repository: repository,
		now:        time.Now,
	}
}

func (s *Service) ListResources(ctx context.Context, facilityKey string) ([]Resource, error) {
	facilityKey = strings.TrimSpace(facilityKey)
	if facilityKey == "" {
		return nil, ErrResourceFacilityRequired
	}

	rows, err := s.repository.ListResourcesByFacilityKey(ctx, facilityKey)
	if err != nil {
		return nil, err
	}

	result := make([]Resource, 0, len(rows))
	for _, row := range rows {
		result = append(result, resourceFromStore(row))
	}
	return result, nil
}

func (s *Service) GetResource(ctx context.Context, resourceKey string) (Resource, error) {
	resourceKey = strings.TrimSpace(resourceKey)
	if resourceKey == "" {
		return Resource{}, ErrResourceKeyRequired
	}

	row, err := s.repository.GetResourceByKey(ctx, resourceKey)
	if err != nil {
		return Resource{}, err
	}
	if row == nil {
		return Resource{}, ErrResourceNotFound
	}

	resource := resourceFromStore(*row)
	edges, err := s.repository.ListResourceEdgesByFacilityKey(ctx, resource.FacilityKey)
	if err != nil {
		return Resource{}, err
	}
	for _, edge := range edges {
		if edge.ResourceKey == resource.ResourceKey || edge.RelatedResourceKey == resource.ResourceKey {
			resource.Edges = append(resource.Edges, edgeFromStore(edge))
		}
	}
	return resource, nil
}

func (s *Service) UpsertResource(ctx context.Context, actor StaffActor, input ResourceInput) (Resource, error) {
	if err := validateStaffActor(actor); err != nil {
		return Resource{}, err
	}

	resolved, err := validateResourceInput(input)
	if err != nil {
		return Resource{}, err
	}

	row, err := withScheduleQueriesTx(ctx, s.repository.db, func(queries *store.Queries) (store.ApolloScheduleResource, error) {
		if err := s.validateFacilityAndZone(ctx, queries, resolved.FacilityKey, resolved.ZoneKey); err != nil {
			return store.ApolloScheduleResource{}, err
		}

		return queries.UpsertScheduleResource(ctx, store.UpsertScheduleResourceParams{
			ResourceKey:  resolved.ResourceKey,
			FacilityKey:  resolved.FacilityKey,
			ZoneKey:      resolved.ZoneKey,
			ResourceType: resolved.ResourceType,
			DisplayName:  resolved.DisplayName,
			PublicLabel:  resolved.PublicLabel,
			Bookable:     resolved.Bookable,
			Active:       resolved.Active,
			UpdatedAt:    pgTimestamptzFromTime(s.now().UTC()),
		})
	})
	if err != nil {
		return Resource{}, err
	}

	return resourceFromStore(row), nil
}

func (s *Service) ListResourceEdges(ctx context.Context, facilityKey string) ([]ResourceEdge, error) {
	facilityKey = strings.TrimSpace(facilityKey)
	if facilityKey == "" {
		return nil, ErrResourceFacilityRequired
	}

	rows, err := s.repository.ListResourceEdgesByFacilityKey(ctx, facilityKey)
	if err != nil {
		return nil, err
	}

	result := make([]ResourceEdge, 0, len(rows))
	for _, row := range rows {
		result = append(result, edgeFromStore(row))
	}
	return result, nil
}

func (s *Service) UpsertResourceEdge(ctx context.Context, actor StaffActor, input ResourceEdgeInput) (ResourceEdge, error) {
	if err := validateStaffActor(actor); err != nil {
		return ResourceEdge{}, err
	}

	resolved, err := validateResourceEdgeInput(input)
	if err != nil {
		return ResourceEdge{}, err
	}

	row, err := withScheduleQueriesTx(ctx, s.repository.db, func(queries *store.Queries) (store.ApolloScheduleResourceEdge, error) {
		source, err := s.requireResource(ctx, queries, resolved.ResourceKey)
		if err != nil {
			return store.ApolloScheduleResourceEdge{}, err
		}
		related, err := s.requireResource(ctx, queries, resolved.RelatedResourceKey)
		if err != nil {
			return store.ApolloScheduleResourceEdge{}, err
		}
		if source.FacilityKey != related.FacilityKey {
			return store.ApolloScheduleResourceEdge{}, ErrResourceEdgeInvalid
		}

		sourceKey := resolved.ResourceKey
		relatedKey := resolved.RelatedResourceKey
		if resolved.EdgeType == EdgeExclusiveWith && relatedKey < sourceKey {
			sourceKey, relatedKey = relatedKey, sourceKey
		}

		snapshot, err := s.loadFacilitySnapshot(ctx, queries, source.FacilityKey)
		if err != nil {
			return store.ApolloScheduleResourceEdge{}, err
		}
		if resolved.EdgeType != EdgeExclusiveWith && snapshot.resourceHasPath(relatedKey, sourceKey) {
			return store.ApolloScheduleResourceEdge{}, ErrResourceEdgeCycle
		}

		return queries.UpsertScheduleResourceEdge(ctx, store.UpsertScheduleResourceEdgeParams{
			ResourceKey:        sourceKey,
			RelatedResourceKey: relatedKey,
			EdgeType:           resolved.EdgeType,
			UpdatedAt:          pgTimestamptzFromTime(s.now().UTC()),
		})
	})
	if err != nil {
		return ResourceEdge{}, err
	}

	return edgeFromStore(row), nil
}

func (s *Service) ListBlocks(ctx context.Context, facilityKey string) ([]Block, error) {
	facilityKey = strings.TrimSpace(facilityKey)
	if facilityKey == "" {
		return nil, ErrResourceFacilityRequired
	}

	window := CalendarWindow{
		From:  s.now().UTC(),
		Until: s.now().UTC().AddDate(0, 0, DefaultCalendarWindowDays),
	}
	snapshot, err := s.loadFacilitySnapshotFromRepository(ctx, facilityKey)
	if err != nil {
		return nil, err
	}

	return s.decorateBlocks(snapshot.blocks, snapshot.blocks, snapshot, window)
}

func (s *Service) GetCalendar(ctx context.Context, facilityKey string, window CalendarWindow) ([]Occurrence, error) {
	facilityKey = strings.TrimSpace(facilityKey)
	if facilityKey == "" {
		return nil, ErrResourceFacilityRequired
	}
	if err := validateCalendarWindow(window); err != nil {
		return nil, err
	}

	snapshot, err := s.loadFacilitySnapshotFromRepository(ctx, facilityKey)
	if err != nil {
		return nil, err
	}

	activeBlocks := make([]Block, 0, len(snapshot.blocks))
	for _, block := range snapshot.blocks {
		if block.Status != StatusCancelled {
			activeBlocks = append(activeBlocks, block)
		}
	}

	occurrenceIndex, occurrences, err := s.expandOccurrences(activeBlocks, snapshot, window)
	if err != nil {
		return nil, err
	}

	result := make([]Occurrence, 0, len(occurrences))
	for _, occurrence := range occurrences {
		occurrence.Conflicts = dedupeConflicts(occurrenceConflicts(occurrence, occurrences, snapshot))
		result = append(result, occurrence.Occurrence)
	}

	_ = occurrenceIndex
	return result, nil
}

func (s *Service) CreateBlock(ctx context.Context, actor StaffActor, input BlockInput) (Block, error) {
	return withScheduleQueriesTx(ctx, s.repository.db, func(queries *store.Queries) (Block, error) {
		return s.CreateBlockWithQueries(ctx, queries, actor, input)
	})
}

func (s *Service) CreateBlockWithQueries(ctx context.Context, queries *store.Queries, actor StaffActor, input BlockInput) (Block, error) {
	if err := validateStaffActor(actor); err != nil {
		return Block{}, err
	}

	resolved, err := validateBlockInput(input)
	if err != nil {
		return Block{}, err
	}

	if err := s.validateBlockReferences(ctx, queries, resolved); err != nil {
		return Block{}, err
	}
	snapshot, err := s.loadFacilitySnapshot(ctx, queries, resolved.FacilityKey)
	if err != nil {
		return Block{}, err
	}

	writeParams, err := s.prepareCandidateBlock(ctx, queries, actor, resolved, snapshot, s.now().UTC())
	if err != nil {
		return Block{}, err
	}

	if err := s.validateBlockAgainstSnapshot(writeParams.candidate, snapshot); err != nil {
		return Block{}, err
	}

	row, err := queries.CreateScheduleBlock(ctx, writeParams.params)
	if err != nil {
		return Block{}, err
	}

	block := blockFromStore(row)
	return s.decorateBlock(block, append(snapshot.blocks, block), snapshot, writeParams.window)
}

func (s *Service) PreviewBlock(ctx context.Context, input BlockInput) (AvailabilityDecision, error) {
	queries := store.New(s.repository.db)
	return s.PreviewBlockWithQueries(ctx, queries, input)
}

func (s *Service) PreviewBlockWithQueries(ctx context.Context, queries *store.Queries, input BlockInput) (AvailabilityDecision, error) {
	resolved, err := validateBlockInput(input)
	if err != nil {
		return AvailabilityDecision{}, err
	}
	if err := s.validateBlockReferences(ctx, queries, resolved); err != nil {
		return AvailabilityDecision{}, err
	}

	snapshot, err := s.loadFacilitySnapshot(ctx, queries, resolved.FacilityKey)
	if err != nil {
		return AvailabilityDecision{}, err
	}

	candidate, window, err := s.previewCandidateBlock(ctx, queries, resolved, snapshot, s.now().UTC())
	if err != nil {
		return AvailabilityDecision{}, err
	}

	if err := s.validateBlockAgainstSnapshot(candidate, snapshot); err != nil {
		if errors.Is(err, ErrBlockConflictRejected) {
			decorated, decorateErr := s.decorateBlock(candidate, append(snapshot.blocks, candidate), snapshot, window)
			if decorateErr != nil {
				return AvailabilityDecision{}, decorateErr
			}
			return AvailabilityDecision{
				Status:    AvailabilityConflict,
				Available: false,
				Conflicts: decorated.Conflicts,
			}, nil
		}
		return AvailabilityDecision{}, err
	}

	decorated, err := s.decorateBlock(candidate, append(snapshot.blocks, candidate), snapshot, window)
	if err != nil {
		return AvailabilityDecision{}, err
	}
	status := AvailabilityAvailable
	available := true
	if len(decorated.Conflicts) > 0 {
		status = AvailabilityConflict
		available = false
	}
	return AvailabilityDecision{
		Status:    status,
		Available: available,
		Conflicts: decorated.Conflicts,
	}, nil
}

func (s *Service) validateBlockReferences(ctx context.Context, queries *store.Queries, input BlockInput) error {
	if err := s.validateFacilityAndZone(ctx, queries, input.FacilityKey, input.ZoneKey); err != nil {
		return err
	}
	if input.ResourceKey == nil {
		return nil
	}

	resource, err := s.requireResource(ctx, queries, *input.ResourceKey)
	if err != nil {
		return err
	}
	if resource.FacilityKey != input.FacilityKey {
		return ErrResourceFacilityInvalid
	}
	if input.ZoneKey != nil {
		if resource.ZoneKey == nil || *input.ZoneKey != *resource.ZoneKey {
			return ErrResourceZoneInvalid
		}
	}
	return nil
}

func (s *Service) AddException(ctx context.Context, actor StaffActor, blockID uuid.UUID, expectedVersion int, input BlockExceptionInput) (Block, error) {
	if err := validateStaffActor(actor); err != nil {
		return Block{}, err
	}

	exceptionDate, err := pgDateFromString(strings.TrimSpace(input.ExceptionDate))
	if err != nil || !exceptionDate.Valid {
		return Block{}, ErrExceptionDateRequired
	}

	return withScheduleQueriesTx(ctx, s.repository.db, func(queries *store.Queries) (Block, error) {
		current, err := s.requireBlock(ctx, queries, blockID)
		if err != nil {
			return Block{}, err
		}
		if current.Version != int32(expectedVersion) {
			return Block{}, ErrBlockVersionStale
		}
		if current.Status == StatusCancelled {
			return Block{}, ErrBlockCancelled
		}
		if current.ScheduleType != ScheduleTypeWeekly {
			return Block{}, ErrExceptionNotAllowed
		}
		if !exceptionWithinBlock(current, exceptionDate) {
			return Block{}, ErrExceptionWindowInvalid
		}

		if _, err := queries.InsertScheduleBlockException(ctx, store.InsertScheduleBlockExceptionParams{
			ScheduleBlockID:            blockID,
			ExceptionDate:              exceptionDate,
			CreatedByUserID:            actor.UserID,
			CreatedBySessionID:         actor.SessionID,
			CreatedByRole:              string(actor.Role),
			CreatedByCapability:        string(actor.Capability),
			CreatedTrustedSurfaceKey:   actor.TrustedSurfaceKey,
			CreatedTrustedSurfaceLabel: nullableStringPtr(actor.TrustedSurfaceLabel),
			CreatedAt:                  pgTimestamptzFromTime(s.now().UTC()),
		}); err != nil {
			if isUniqueViolation(err) {
				return Block{}, ErrBlockVersionStale
			}
			return Block{}, err
		}

		updated, err := queries.BumpScheduleBlockVersion(ctx, store.BumpScheduleBlockVersionParams{
			ID:                         blockID,
			Version:                    current.Version,
			UpdatedByUserID:            actor.UserID,
			UpdatedBySessionID:         actor.SessionID,
			UpdatedByRole:              string(actor.Role),
			UpdatedByCapability:        string(actor.Capability),
			UpdatedTrustedSurfaceKey:   actor.TrustedSurfaceKey,
			UpdatedTrustedSurfaceLabel: nullableStringPtr(actor.TrustedSurfaceLabel),
			UpdatedAt:                  pgTimestamptzFromTime(s.now().UTC()),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Block{}, ErrBlockVersionStale
			}
			return Block{}, err
		}

		snapshot, err := s.loadFacilitySnapshot(ctx, queries, updated.FacilityKey)
		if err != nil {
			return Block{}, err
		}
		block := blockFromStore(updated)
		return s.decorateBlock(block, snapshot.blocks, snapshot, s.windowForBlock(block))
	})
}

func (s *Service) CancelBlock(ctx context.Context, actor StaffActor, blockID uuid.UUID, expectedVersion int) (Block, error) {
	if err := validateStaffActor(actor); err != nil {
		return Block{}, err
	}

	return withScheduleQueriesTx(ctx, s.repository.db, func(queries *store.Queries) (Block, error) {
		return s.CancelBlockWithQueries(ctx, queries, actor, blockID, expectedVersion)
	})
}

func (s *Service) CancelBlockWithQueries(ctx context.Context, queries *store.Queries, actor StaffActor, blockID uuid.UUID, expectedVersion int) (Block, error) {
	if err := validateStaffActor(actor); err != nil {
		return Block{}, err
	}

	current, err := s.requireBlockForUpdate(ctx, queries, blockID)
	if err != nil {
		return Block{}, err
	}
	if current.Version != int32(expectedVersion) {
		return Block{}, ErrBlockVersionStale
	}
	if current.Status == StatusCancelled {
		return Block{}, ErrBlockCancelled
	}

	return s.cancelLockedBlock(ctx, queries, actor, current)
}

func (s *Service) CancelLinkedReservationWithQueries(ctx context.Context, queries *store.Queries, actor StaffActor, blockID uuid.UUID, expected ReservationCancellationExpectation) (Block, error) {
	if err := validateStaffActor(actor); err != nil {
		return Block{}, err
	}

	current, err := s.requireBlockForUpdate(ctx, queries, blockID)
	if err != nil {
		return Block{}, err
	}
	if current.Status == StatusCancelled {
		return Block{}, ErrBlockCancelled
	}
	if !scheduleBlockMatchesReservationExpectation(current, expected) {
		return Block{}, ErrBlockReservationMismatch
	}

	return s.cancelLockedBlock(ctx, queries, actor, current)
}

func (s *Service) cancelLockedBlock(ctx context.Context, queries *store.Queries, actor StaffActor, current store.ApolloScheduleBlock) (Block, error) {
	row, err := queries.CancelScheduleBlock(ctx, store.CancelScheduleBlockParams{
		ID:                         current.ID,
		Version:                    current.Version,
		UpdatedByUserID:            actor.UserID,
		UpdatedBySessionID:         actor.SessionID,
		UpdatedByRole:              string(actor.Role),
		UpdatedByCapability:        string(actor.Capability),
		UpdatedTrustedSurfaceKey:   actor.TrustedSurfaceKey,
		UpdatedTrustedSurfaceLabel: nullableStringPtr(actor.TrustedSurfaceLabel),
		CancelledAt:                pgTimestamptzFromTime(s.now().UTC()),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Block{}, ErrBlockVersionStale
		}
		return Block{}, err
	}

	snapshot, err := s.loadFacilitySnapshot(ctx, queries, row.FacilityKey)
	if err != nil {
		return Block{}, err
	}
	block := blockFromStore(row)
	return s.decorateBlock(block, snapshot.blocks, snapshot, s.windowForBlock(block))
}

func (s *Service) validateFacilityAndZone(ctx context.Context, queries *store.Queries, facilityKey string, zoneKey *string) error {
	exists, err := queries.FacilityCatalogRefExists(ctx, facilityKey)
	if err != nil {
		return err
	}
	if !exists {
		return ErrResourceFacilityInvalid
	}
	if zoneKey == nil {
		return nil
	}

	zoneExists, err := queries.FacilityZoneRefExists(ctx, store.FacilityZoneRefExistsParams{
		FacilityKey: facilityKey,
		ZoneKey:     *zoneKey,
	})
	if err != nil {
		return err
	}
	if !zoneExists {
		return ErrResourceZoneInvalid
	}
	return nil
}

func (s *Service) previewCandidateBlock(ctx context.Context, queries *store.Queries, input BlockInput, snapshot *facilitySnapshot, now time.Time) (Block, CalendarWindow, error) {
	params, err := s.prepareCandidateBlock(ctx, queries, StaffActor{}, input, snapshot, now)
	if err != nil {
		return Block{}, CalendarWindow{}, err
	}

	candidate := params.candidate
	candidate.ID = uuid.New()
	return candidate, params.window, nil
}

func (s *Service) requireResource(ctx context.Context, queries *store.Queries, resourceKey string) (store.ApolloScheduleResource, error) {
	row, err := queries.GetScheduleResourceByKey(ctx, resourceKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ApolloScheduleResource{}, ErrResourceNotFound
	}
	if err != nil {
		return store.ApolloScheduleResource{}, err
	}
	return row, nil
}

func (s *Service) requireBlock(ctx context.Context, queries *store.Queries, blockID uuid.UUID) (store.ApolloScheduleBlock, error) {
	row, err := queries.GetScheduleBlockByID(ctx, blockID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ApolloScheduleBlock{}, ErrBlockNotFound
	}
	if err != nil {
		return store.ApolloScheduleBlock{}, err
	}
	return row, nil
}

func (s *Service) requireBlockForUpdate(ctx context.Context, queries *store.Queries, blockID uuid.UUID) (store.ApolloScheduleBlock, error) {
	row, err := queries.GetScheduleBlockByIDForUpdate(ctx, blockID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.ApolloScheduleBlock{}, ErrBlockNotFound
	}
	if err != nil {
		return store.ApolloScheduleBlock{}, err
	}
	return row, nil
}

type blockWriteParams struct {
	params    store.CreateScheduleBlockParams
	candidate Block
	window    CalendarWindow
}

func (s *Service) prepareCandidateBlock(ctx context.Context, queries *store.Queries, actor StaffActor, input BlockInput, snapshot *facilitySnapshot, now time.Time) (blockWriteParams, error) {
	params := store.CreateScheduleBlockParams{
		FacilityKey:                input.FacilityKey,
		ZoneKey:                    input.ZoneKey,
		ResourceKey:                input.ResourceKey,
		Scope:                      input.Scope,
		Kind:                       input.Kind,
		Effect:                     input.Effect,
		Visibility:                 input.Visibility,
		CreatedByUserID:            actor.UserID,
		CreatedBySessionID:         actor.SessionID,
		CreatedByRole:              string(actor.Role),
		CreatedByCapability:        string(actor.Capability),
		CreatedTrustedSurfaceKey:   actor.TrustedSurfaceKey,
		CreatedTrustedSurfaceLabel: nullableStringPtr(actor.TrustedSurfaceLabel),
		UpdatedByUserID:            actor.UserID,
		UpdatedBySessionID:         actor.SessionID,
		UpdatedByRole:              string(actor.Role),
		UpdatedByCapability:        string(actor.Capability),
		UpdatedTrustedSurfaceKey:   actor.TrustedSurfaceKey,
		UpdatedTrustedSurfaceLabel: nullableStringPtr(actor.TrustedSurfaceLabel),
		CreatedAt:                  pgTimestamptzFromTime(now),
		UpdatedAt:                  pgTimestamptzFromTime(now),
	}

	candidate := Block{
		FacilityKey:                input.FacilityKey,
		ZoneKey:                    input.ZoneKey,
		ResourceKey:                input.ResourceKey,
		Scope:                      input.Scope,
		Kind:                       input.Kind,
		Effect:                     input.Effect,
		Visibility:                 input.Visibility,
		Status:                     StatusScheduled,
		Version:                    1,
		CreatedByUserID:            actor.UserID,
		CreatedBySessionID:         actor.SessionID,
		CreatedByRole:              string(actor.Role),
		CreatedByCapability:        string(actor.Capability),
		CreatedTrustedSurfaceKey:   actor.TrustedSurfaceKey,
		CreatedTrustedSurfaceLabel: nullableStringPtr(actor.TrustedSurfaceLabel),
		UpdatedByUserID:            actor.UserID,
		UpdatedBySessionID:         actor.SessionID,
		UpdatedByRole:              string(actor.Role),
		UpdatedByCapability:        string(actor.Capability),
		UpdatedTrustedSurfaceKey:   actor.TrustedSurfaceKey,
		UpdatedTrustedSurfaceLabel: nullableStringPtr(actor.TrustedSurfaceLabel),
		CreatedAt:                  now,
		UpdatedAt:                  now,
	}

	window := CalendarWindow{}
	switch {
	case input.OneOff != nil:
		params.ScheduleType = ScheduleTypeOneOff
		params.StartAt = pgTimestamptzFromTime(input.OneOff.StartsAt.UTC())
		params.EndAt = pgTimestamptzFromTime(input.OneOff.EndsAt.UTC())
		candidate.ScheduleType = ScheduleTypeOneOff
		candidate.StartAt = timePtr(input.OneOff.StartsAt.UTC())
		candidate.EndAt = timePtr(input.OneOff.EndsAt.UTC())
		window = CalendarWindow{From: input.OneOff.StartsAt.UTC(), Until: input.OneOff.EndsAt.UTC()}
	case input.Weekly != nil:
		resolved, err := s.resolveWeeklyCandidate(ctx, queries, candidate, params, input, snapshot, now)
		if err != nil {
			return blockWriteParams{}, err
		}
		return resolved, nil
	default:
		return blockWriteParams{}, ErrBlockShapeInvalid
	}

	return blockWriteParams{params: params, candidate: candidate, window: window}, nil
}

func (s *Service) resolveWeeklyCandidate(ctx context.Context, queries *store.Queries, candidate Block, params store.CreateScheduleBlockParams, input BlockInput, snapshot *facilitySnapshot, now time.Time) (blockWriteParams, error) {
	weekday := int16(input.Weekly.Weekday)
	params.ScheduleType = ScheduleTypeWeekly
	params.Weekday = &weekday
	params.Timezone = &input.Weekly.Timezone

	startTime, err := pgTimeFromString(input.Weekly.StartTime)
	if err != nil {
		return blockWriteParams{}, err
	}
	endTime, err := pgTimeFromString(input.Weekly.EndTime)
	if err != nil {
		return blockWriteParams{}, err
	}
	recurrenceStartDate, err := pgDateFromString(input.Weekly.RecurrenceStartDate)
	if err != nil {
		return blockWriteParams{}, err
	}
	params.StartTime = startTime
	params.EndTime = endTime
	params.RecurrenceStartDate = recurrenceStartDate
	candidate.ScheduleType = ScheduleTypeWeekly
	candidate.Weekday = intPtr(input.Weekly.Weekday)
	candidate.StartTime = stringPtr(input.Weekly.StartTime)
	candidate.EndTime = stringPtr(input.Weekly.EndTime)
	candidate.Timezone = &input.Weekly.Timezone
	candidate.RecurrenceStartDate = stringPtr(input.Weekly.RecurrenceStartDate)

	if input.Weekly.RecurrenceEndDate == nil {
		return blockWriteParams{}, ErrBlockRecurrenceInvalid
	}

	endDate, err := pgDateFromString(*input.Weekly.RecurrenceEndDate)
	if err != nil {
		return blockWriteParams{}, err
	}
	params.RecurrenceEndDate = endDate
	candidate.RecurrenceEndDate = stringPtr(*input.Weekly.RecurrenceEndDate)

	window := blockWindow(candidate)
	if window.From.IsZero() || window.Until.IsZero() {
		return blockWriteParams{}, ErrBlockRecurrenceInvalid
	}
	if err := validateCalendarWindow(window); err != nil {
		return blockWriteParams{}, err
	}

	return blockWriteParams{params: params, candidate: candidate, window: window}, nil
}

func validateResourceInput(input ResourceInput) (ResourceInput, error) {
	input.ResourceKey = strings.TrimSpace(input.ResourceKey)
	input.FacilityKey = strings.TrimSpace(input.FacilityKey)
	input.ResourceType = strings.TrimSpace(input.ResourceType)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if input.PublicLabel != nil {
		value := strings.TrimSpace(*input.PublicLabel)
		input.PublicLabel = &value
	}

	switch {
	case input.ResourceKey == "":
		return ResourceInput{}, ErrResourceKeyRequired
	case input.FacilityKey == "":
		return ResourceInput{}, ErrResourceFacilityRequired
	case input.ResourceType == "":
		return ResourceInput{}, ErrResourceTypeRequired
	case input.DisplayName == "":
		return ResourceInput{}, ErrResourceDisplayNameRequired
	}

	return input, nil
}

func validateResourceEdgeInput(input ResourceEdgeInput) (ResourceEdgeInput, error) {
	input.ResourceKey = strings.TrimSpace(input.ResourceKey)
	input.RelatedResourceKey = strings.TrimSpace(input.RelatedResourceKey)
	input.EdgeType = strings.TrimSpace(input.EdgeType)
	switch {
	case input.ResourceKey == "" || input.RelatedResourceKey == "":
		return ResourceEdgeInput{}, ErrResourceEdgeInvalid
	case input.ResourceKey == input.RelatedResourceKey:
		return ResourceEdgeInput{}, ErrResourceEdgeSelfReference
	case input.EdgeType != EdgeContains && input.EdgeType != EdgeComposes && input.EdgeType != EdgeExclusiveWith:
		return ResourceEdgeInput{}, ErrResourceEdgeInvalid
	}
	if input.EdgeType == EdgeExclusiveWith && input.RelatedResourceKey < input.ResourceKey {
		input.ResourceKey, input.RelatedResourceKey = input.RelatedResourceKey, input.ResourceKey
	}
	return input, nil
}

func validateBlockInput(input BlockInput) (BlockInput, error) {
	input.FacilityKey = strings.TrimSpace(input.FacilityKey)
	input.Scope = strings.TrimSpace(input.Scope)
	input.Kind = strings.TrimSpace(input.Kind)
	input.Effect = strings.TrimSpace(input.Effect)
	input.Visibility = strings.TrimSpace(input.Visibility)
	if input.ZoneKey != nil {
		value := strings.TrimSpace(*input.ZoneKey)
		input.ZoneKey = &value
	}
	if input.ResourceKey != nil {
		value := strings.TrimSpace(*input.ResourceKey)
		input.ResourceKey = &value
	}

	switch {
	case input.FacilityKey == "":
		return BlockInput{}, ErrResourceFacilityRequired
	case input.Scope != ScopeFacility && input.Scope != ScopeZone && input.Scope != ScopeResource:
		return BlockInput{}, ErrBlockScopeInvalid
	case input.Kind != KindOperatingHours && input.Kind != KindClosure && input.Kind != KindEvent && input.Kind != KindHold && input.Kind != KindReservation:
		return BlockInput{}, ErrBlockKindInvalid
	case input.Effect != EffectInformational && input.Effect != EffectSoftHold && input.Effect != EffectHardReserve && input.Effect != EffectClosed:
		return BlockInput{}, ErrBlockEffectInvalid
	case input.Visibility != VisibilityInternal && input.Visibility != VisibilityPublicBusy && input.Visibility != VisibilityPublicLabeled:
		return BlockInput{}, ErrBlockVisibilityInvalid
	case (input.OneOff == nil) == (input.Weekly == nil):
		return BlockInput{}, ErrBlockShapeInvalid
	}

	switch input.Kind {
	case KindOperatingHours:
		if input.Effect != EffectInformational {
			return BlockInput{}, ErrBlockEffectInvalid
		}
	case KindClosure:
		if input.Effect != EffectClosed {
			return BlockInput{}, ErrBlockEffectInvalid
		}
	case KindHold:
		if input.Effect != EffectSoftHold || input.Visibility != VisibilityInternal {
			return BlockInput{}, ErrBlockVisibilityInvalid
		}
	case KindReservation:
		if input.Effect != EffectHardReserve {
			return BlockInput{}, ErrBlockEffectInvalid
		}
	case KindEvent:
		if input.Effect != EffectInformational && input.Effect != EffectHardReserve {
			return BlockInput{}, ErrBlockEffectInvalid
		}
	}

	if input.Scope == ScopeZone && input.ZoneKey == nil {
		return BlockInput{}, ErrBlockScopeInvalid
	}
	if input.Scope == ScopeResource && input.ResourceKey == nil {
		return BlockInput{}, ErrBlockScopeInvalid
	}

	if input.OneOff != nil && !input.OneOff.StartsAt.Before(input.OneOff.EndsAt) {
		return BlockInput{}, ErrBlockDateWindowInvalid
	}
	if input.Scope == ScopeFacility && (input.ZoneKey != nil || input.ResourceKey != nil) {
		return BlockInput{}, ErrBlockScopeInvalid
	}
	if input.Scope == ScopeZone && input.ResourceKey != nil {
		return BlockInput{}, ErrBlockScopeInvalid
	}
	if input.Scope == ScopeResource && input.ResourceKey == nil {
		return BlockInput{}, ErrBlockScopeInvalid
	}
	if input.Weekly != nil {
		input.Weekly.StartTime = strings.TrimSpace(input.Weekly.StartTime)
		input.Weekly.EndTime = strings.TrimSpace(input.Weekly.EndTime)
		input.Weekly.Timezone = strings.TrimSpace(input.Weekly.Timezone)
		input.Weekly.RecurrenceStartDate = strings.TrimSpace(input.Weekly.RecurrenceStartDate)
		if input.Weekly.RecurrenceEndDate != nil {
			value := strings.TrimSpace(*input.Weekly.RecurrenceEndDate)
			input.Weekly.RecurrenceEndDate = &value
		}
		if input.Weekly.Weekday < 1 || input.Weekly.Weekday > 7 {
			return BlockInput{}, ErrBlockRecurrenceInvalid
		}
		if input.Weekly.Timezone == "" {
			return BlockInput{}, ErrBlockTimezoneRequired
		}
		if input.Weekly.StartTime == "" || input.Weekly.EndTime == "" || input.Weekly.RecurrenceStartDate == "" {
			return BlockInput{}, ErrBlockRecurrenceInvalid
		}
		if input.Weekly.RecurrenceEndDate == nil || *input.Weekly.RecurrenceEndDate == "" {
			return BlockInput{}, ErrBlockRecurrenceInvalid
		}
	}

	return input, nil
}

func validateCalendarWindow(window CalendarWindow) error {
	if window.From.IsZero() || window.Until.IsZero() || !window.From.Before(window.Until) {
		return ErrBlockDateWindowInvalid
	}
	if window.Until.Sub(window.From) > time.Duration(DefaultCalendarWindowDays)*24*time.Hour {
		return ErrBlockDateWindowTooLarge
	}
	return nil
}

func validateStaffActor(actor StaffActor) error {
	if actor.UserID == uuid.Nil || actor.SessionID == uuid.Nil {
		return ErrActorAttributionRequired
	}
	if strings.TrimSpace(actor.TrustedSurfaceKey) == "" {
		return ErrActorTrustedSurfaceMissing
	}
	return nil
}

func scheduleBlockMatchesReservationExpectation(row store.ApolloScheduleBlock, expected ReservationCancellationExpectation) bool {
	return row.FacilityKey == strings.TrimSpace(expected.FacilityKey) &&
		row.Scope == strings.TrimSpace(expected.Scope) &&
		row.ScheduleType == ScheduleTypeOneOff &&
		row.Kind == KindReservation &&
		row.Effect == EffectHardReserve &&
		row.Visibility == VisibilityInternal &&
		optionalStringsEqual(row.ZoneKey, expected.ZoneKey) &&
		optionalStringsEqual(row.ResourceKey, expected.ResourceKey) &&
		row.StartAt.Valid &&
		row.EndAt.Valid &&
		row.StartAt.Time.UTC().Equal(expected.StartsAt.UTC()) &&
		row.EndAt.Time.UTC().Equal(expected.EndsAt.UTC())
}

func optionalStringsEqual(left *string, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func nullableStringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func timePtr(value time.Time) *time.Time {
	converted := value.UTC()
	return &converted
}

type facilitySnapshot struct {
	resources      map[string]store.ApolloScheduleResource
	edges          map[string][]string
	exclusivePairs map[string]struct{}
	exceptions     map[uuid.UUID][]string
	blocks         []Block
}

type blockOccurrence struct {
	Occurrence
	coverage map[string]struct{}
}

func (s *Service) loadFacilitySnapshotFromRepository(ctx context.Context, facilityKey string) (*facilitySnapshot, error) {
	queries := store.New(s.repository.db)
	return s.loadFacilitySnapshot(ctx, queries, facilityKey)
}

func (s *Service) loadFacilitySnapshot(ctx context.Context, queries *store.Queries, facilityKey string) (*facilitySnapshot, error) {
	resources, err := queries.ListScheduleResourcesByFacilityKey(ctx, facilityKey)
	if err != nil {
		return nil, err
	}
	edges, err := queries.ListScheduleResourceEdgesByFacilityKey(ctx, facilityKey)
	if err != nil {
		return nil, err
	}
	blockRows, err := queries.ListScheduleBlocksByFacilityKey(ctx, facilityKey)
	if err != nil {
		return nil, err
	}

	blocks := make([]Block, 0, len(blockRows))
	blockIDs := make([]uuid.UUID, 0, len(blockRows))
	for _, row := range blockRows {
		block := blockFromStore(row)
		blocks = append(blocks, block)
		if block.Status != StatusCancelled {
			blockIDs = append(blockIDs, block.ID)
		}
	}

	exceptions, err := queries.ListScheduleBlockExceptionsByBlockIDs(ctx, blockIDs)
	if err != nil {
		return nil, err
	}

	resourceMap := make(map[string]store.ApolloScheduleResource, len(resources))
	for _, resource := range resources {
		resourceMap[resource.ResourceKey] = resource
	}

	edgeMap := make(map[string][]string)
	exclusivePairs := make(map[string]struct{})
	for _, edge := range edges {
		switch edge.EdgeType {
		case EdgeContains, EdgeComposes:
			edgeMap[edge.ResourceKey] = append(edgeMap[edge.ResourceKey], edge.RelatedResourceKey)
		case EdgeExclusiveWith:
			exclusivePairs[normalizedPairKey(edge.ResourceKey, edge.RelatedResourceKey)] = struct{}{}
		}
	}

	exceptionMap := make(map[uuid.UUID][]string)
	for _, exception := range exceptions {
		exceptionMap[exception.ScheduleBlockID] = append(exceptionMap[exception.ScheduleBlockID], exceptionFromStore(exception))
	}

	return &facilitySnapshot{
		resources:      resourceMap,
		edges:          edgeMap,
		exclusivePairs: exclusivePairs,
		exceptions:     exceptionMap,
		blocks:         blocks,
	}, nil
}

func (snapshot *facilitySnapshot) resourceDescendants(root string) map[string]struct{} {
	descendants := make(map[string]struct{})
	visited := map[string]struct{}{root: {}}
	queue := []string{root}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range snapshot.edges[current] {
			if _, seen := visited[next]; seen {
				continue
			}
			visited[next] = struct{}{}
			descendants[next] = struct{}{}
			queue = append(queue, next)
		}
	}
	return descendants
}

func (snapshot *facilitySnapshot) resourceHasPath(from string, to string) bool {
	if from == to {
		return true
	}

	visited := map[string]struct{}{from: {}}
	queue := []string{from}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range snapshot.edges[current] {
			if _, seen := visited[next]; seen {
				continue
			}
			if next == to {
				return true
			}
			visited[next] = struct{}{}
			queue = append(queue, next)
		}
	}
	return false
}

func (snapshot *facilitySnapshot) exclusiveConflict(left, right string) bool {
	_, ok := snapshot.exclusivePairs[normalizedPairKey(left, right)]
	return ok
}

func (s *Service) decorateBlocks(targetBlocks []Block, allBlocks []Block, snapshot *facilitySnapshot, window CalendarWindow) ([]Block, error) {
	activeBlocks := make([]Block, 0, len(allBlocks))
	for _, block := range allBlocks {
		if block.Status != StatusCancelled {
			activeBlocks = append(activeBlocks, block)
		}
	}

	occurrenceIndex, allOccurrences, err := s.expandOccurrences(activeBlocks, snapshot, window)
	if err != nil {
		return nil, err
	}

	result := make([]Block, 0, len(targetBlocks))
	for _, block := range targetBlocks {
		if block.Status == StatusCancelled {
			if exceptions := snapshot.exceptions[block.ID]; len(exceptions) > 0 {
				block.Exceptions = append([]string(nil), exceptions...)
			}
			result = append(result, block)
			continue
		}

		blockOccurrences := occurrenceIndex[block.ID]
		block.Conflicts = dedupeConflicts(blockConflicts(block, allOccurrences, snapshot, blockOccurrences))
		if exceptions := snapshot.exceptions[block.ID]; len(exceptions) > 0 {
			block.Exceptions = append([]string(nil), exceptions...)
		}
		result = append(result, block)
	}
	return result, nil
}

func (s *Service) decorateBlock(block Block, allBlocks []Block, snapshot *facilitySnapshot, window CalendarWindow) (Block, error) {
	decorated, err := s.decorateBlocks([]Block{block}, allBlocks, snapshot, window)
	if err != nil {
		return Block{}, err
	}
	if len(decorated) == 0 {
		return Block{}, ErrBlockNotFound
	}
	return decorated[0], nil
}

func (s *Service) expandOccurrences(blocks []Block, snapshot *facilitySnapshot, window CalendarWindow) (map[uuid.UUID][]blockOccurrence, []blockOccurrence, error) {
	index := make(map[uuid.UUID][]blockOccurrence)
	all := make([]blockOccurrence, 0)
	for _, block := range blocks {
		occurrences, err := s.expandBlockOccurrences(block, snapshot, window)
		if err != nil {
			return nil, nil, err
		}
		index[block.ID] = occurrences
		all = append(all, occurrences...)
	}
	return index, all, nil
}

func (s *Service) expandBlockOccurrences(block Block, snapshot *facilitySnapshot, window CalendarWindow) ([]blockOccurrence, error) {
	coverage := effectiveCoverageFromBlock(block, snapshot)
	switch block.ScheduleType {
	case ScheduleTypeOneOff:
		if block.StartAt == nil || block.EndAt == nil {
			return nil, ErrBlockShapeInvalid
		}
		if !intervalsOverlap(*block.StartAt, *block.EndAt, window.From, window.Until) {
			return []blockOccurrence{}, nil
		}
		return []blockOccurrence{{
			Occurrence: Occurrence{
				BlockID:        block.ID,
				FacilityKey:    block.FacilityKey,
				ZoneKey:        block.ZoneKey,
				ResourceKey:    block.ResourceKey,
				Scope:          block.Scope,
				Kind:           block.Kind,
				Effect:         block.Effect,
				Visibility:     block.Visibility,
				Status:         block.Status,
				StartsAt:       *block.StartAt,
				EndsAt:         *block.EndAt,
				OccurrenceDate: block.StartAt.UTC().Format(isoDateLayout),
			},
			coverage: coverage,
		}}, nil
	case ScheduleTypeWeekly:
		if block.Weekday == nil || block.StartTime == nil || block.EndTime == nil || block.Timezone == nil || block.RecurrenceStartDate == nil {
			return nil, ErrBlockRecurrenceInvalid
		}

		location, err := time.LoadLocation(*block.Timezone)
		if err != nil {
			return nil, ErrBlockTimezoneRequired
		}
		startDateParsed, err := time.Parse(isoDateLayout, *block.RecurrenceStartDate)
		if err != nil {
			return nil, ErrBlockRecurrenceInvalid
		}
		startDate := time.Date(startDateParsed.Year(), startDateParsed.Month(), startDateParsed.Day(), 0, 0, 0, 0, location)
		windowStart := time.Date(window.From.In(location).Year(), window.From.In(location).Month(), window.From.In(location).Day(), 0, 0, 0, 0, location)
		windowEnd := time.Date(window.Until.In(location).Year(), window.Until.In(location).Month(), window.Until.In(location).Day(), 0, 0, 0, 0, location).AddDate(0, 0, 1)
		if block.RecurrenceEndDate != nil {
			parsedEnd, err := time.Parse(isoDateLayout, *block.RecurrenceEndDate)
			if err != nil {
				return nil, ErrBlockRecurrenceInvalid
			}
			parsedEndLocal := time.Date(parsedEnd.Year(), parsedEnd.Month(), parsedEnd.Day(), 0, 0, 0, 0, location)
			if parsedEndLocal.AddDate(0, 0, 1).Before(windowEnd) {
				windowEnd = parsedEndLocal.AddDate(0, 0, 1)
			}
		}
		if windowEnd.Before(windowStart) {
			return []blockOccurrence{}, nil
		}

		startClock, err := time.Parse(hhmmLayout, *block.StartTime)
		if err != nil {
			return nil, ErrBlockRecurrenceInvalid
		}
		endClock, err := time.Parse(hhmmLayout, *block.EndTime)
		if err != nil {
			return nil, ErrBlockRecurrenceInvalid
		}

		current := alignToWeekday(startDate, *block.Weekday, location)
		if current.Before(windowStart) {
			current = alignToWeekday(windowStart, *block.Weekday, location)
		}

		exceptions := snapshot.exceptions[block.ID]
		occurrences := make([]blockOccurrence, 0)
		for !current.After(windowEnd) {
			localStart := time.Date(current.Year(), current.Month(), current.Day(), startClock.Hour(), startClock.Minute(), 0, 0, location)
			localEnd := time.Date(current.Year(), current.Month(), current.Day(), endClock.Hour(), endClock.Minute(), 0, 0, location)
			if !localEnd.After(localStart) {
				return nil, ErrBlockDateWindowInvalid
			}
			if intervalsOverlap(localStart.UTC(), localEnd.UTC(), window.From, window.Until) && !exceptionMatchesDate(exceptions, current) {
				occurrences = append(occurrences, blockOccurrence{
					Occurrence: Occurrence{
						BlockID:        block.ID,
						FacilityKey:    block.FacilityKey,
						ZoneKey:        block.ZoneKey,
						ResourceKey:    block.ResourceKey,
						Scope:          block.Scope,
						Kind:           block.Kind,
						Effect:         block.Effect,
						Visibility:     block.Visibility,
						Status:         block.Status,
						StartsAt:       localStart.UTC(),
						EndsAt:         localEnd.UTC(),
						OccurrenceDate: current.Format(isoDateLayout),
					},
					coverage: coverage,
				})
			}
			current = current.AddDate(0, 0, 7)
		}
		return occurrences, nil
	default:
		return nil, ErrBlockShapeInvalid
	}
}

func (s *Service) blockCoverage(block Block, snapshot *facilitySnapshot) map[string]struct{} {
	coverage := make(map[string]struct{})
	switch block.Scope {
	case ScopeFacility:
		for resourceKey := range snapshot.resources {
			coverage[resourceKey] = struct{}{}
		}
	case ScopeZone:
		if block.ZoneKey == nil {
			return coverage
		}
		for _, resource := range snapshot.resources {
			if resource.ZoneKey != nil && *resource.ZoneKey == *block.ZoneKey {
				coverage[resource.ResourceKey] = struct{}{}
			}
		}
	case ScopeResource:
		if block.ResourceKey == nil {
			return coverage
		}
		coverage[*block.ResourceKey] = struct{}{}
		for descendant := range snapshot.resourceDescendants(*block.ResourceKey) {
			coverage[descendant] = struct{}{}
		}
	}
	return coverage
}

func (s *Service) claimCoverage(block Block, snapshot *facilitySnapshot) map[string]struct{} {
	coverage := make(map[string]struct{})
	switch block.Scope {
	case ScopeFacility:
		for resourceKey, resource := range snapshot.resources {
			if resourceIsClaimable(resource) {
				coverage[resourceKey] = struct{}{}
			}
		}
	case ScopeZone:
		if block.ZoneKey == nil {
			return coverage
		}
		for _, resource := range snapshot.resources {
			if resource.ZoneKey != nil && *resource.ZoneKey == *block.ZoneKey && resourceIsClaimable(resource) {
				coverage[resource.ResourceKey] = struct{}{}
			}
		}
	case ScopeResource:
		if block.ResourceKey == nil {
			return coverage
		}
		if resource, ok := snapshot.resources[*block.ResourceKey]; ok && resourceIsClaimable(resource) {
			coverage[*block.ResourceKey] = struct{}{}
		}
		for descendant := range snapshot.resourceDescendants(*block.ResourceKey) {
			resource, ok := snapshot.resources[descendant]
			if ok && resourceIsClaimable(resource) {
				coverage[descendant] = struct{}{}
			}
		}
	}
	return coverage
}

func (s *Service) windowForBlock(block Block) CalendarWindow {
	return blockWindow(block)
}

func (s *Service) validateBlockAgainstSnapshot(candidate Block, snapshot *facilitySnapshot) error {
	if candidate.Kind == KindOperatingHours {
		for _, existing := range snapshot.blocks {
			if existing.Status == StatusCancelled || existing.Kind != KindOperatingHours {
				continue
			}
			if !sameScopeIdentity(candidate, existing) {
				continue
			}
			overlaps, err := s.blocksHaveOverlappingOccurrences(candidate, existing, snapshot)
			if err != nil {
				return err
			}
			if overlaps {
				return ErrBlockOperatingHoursOverlap
			}
		}
		return nil
	}

	if !isHardClaimBlock(candidate) {
		return nil
	}

	if candidate.Scope == ScopeResource {
		if candidate.ResourceKey == nil {
			return ErrBlockScopeInvalid
		}
		resource, ok := snapshot.resources[*candidate.ResourceKey]
		if !ok {
			return ErrResourceNotFound
		}
		if !resourceIsClaimable(resource) {
			return ErrBlockResourceNotClaimable
		}
	}

	candidateCoverage := claimCoverageFromBlock(candidate, snapshot)
	if len(candidateCoverage) == 0 {
		return ErrBlockClaimableScopeEmpty
	}

	for _, existing := range snapshot.blocks {
		if existing.Status == StatusCancelled || !isHardClaimBlock(existing) {
			continue
		}
		if !coverageConflict(candidateCoverage, claimCoverageFromBlock(existing, snapshot), snapshot) {
			continue
		}
		overlaps, err := s.blocksHaveOverlappingOccurrences(candidate, existing, snapshot)
		if err != nil {
			return err
		}
		if overlaps {
			return ErrBlockConflictRejected
		}
	}

	return nil
}

func blockConflicts(candidate Block, allOccurrences []blockOccurrence, snapshot *facilitySnapshot, candidateOccurrences []blockOccurrence) []Conflict {
	conflicts := make([]Conflict, 0)
	candidateCoverage := effectiveCoverageFromBlock(candidate, snapshot)
	candidateHardClaim := isHardClaimBlock(candidate)
	for _, other := range allOccurrences {
		if candidate.ID == other.BlockID {
			continue
		}
		overlaps := false
		for _, candidateOccurrence := range candidateOccurrences {
			if intervalsOverlap(candidateOccurrence.StartsAt, candidateOccurrence.EndsAt, other.StartsAt, other.EndsAt) {
				overlaps = true
				break
			}
		}
		if !overlaps {
			continue
		}
		if candidate.Kind == KindOperatingHours && other.Kind == KindOperatingHours && sameScopeIdentity(candidate, Block{Scope: other.Scope, FacilityKey: other.FacilityKey, ZoneKey: other.ZoneKey, ResourceKey: other.ResourceKey}) {
			conflicts = append(conflicts, Conflict{
				BlockID:    other.BlockID,
				Reason:     "operating_hours_overlap",
				Scope:      other.Scope,
				Kind:       other.Kind,
				Effect:     other.Effect,
				Visibility: other.Visibility,
			})
			continue
		}
		otherHardClaim := isHardClaimOccurrence(other.Occurrence)
		scopeConflict := scopeInventoryConflictBlockOccurrence(candidate, other.Occurrence, snapshot)
		coverageBasedConflict := coverageConflict(candidateCoverage, other.coverage, snapshot)
		if (candidateHardClaim && otherHardClaim && coverageBasedConflict) || (!candidateHardClaim || !otherHardClaim) && (scopeConflict || coverageBasedConflict) {
			conflicts = append(conflicts, Conflict{
				BlockID:    other.BlockID,
				Reason:     "effective_inventory_overlap",
				Scope:      other.Scope,
				Kind:       other.Kind,
				Effect:     other.Effect,
				Visibility: other.Visibility,
			})
		}
	}
	return conflicts
}

func occurrenceConflicts(candidate blockOccurrence, all []blockOccurrence, snapshot *facilitySnapshot) []Conflict {
	conflicts := make([]Conflict, 0)
	candidateHardClaim := isHardClaimOccurrence(candidate.Occurrence)
	for _, other := range all {
		if candidate.BlockID == other.BlockID {
			continue
		}
		if !intervalsOverlap(candidate.StartsAt, candidate.EndsAt, other.StartsAt, other.EndsAt) {
			continue
		}
		if candidate.Kind == KindOperatingHours && other.Kind == KindOperatingHours && sameOccurrenceScope(candidate.Occurrence, other.Occurrence) {
			conflicts = append(conflicts, Conflict{
				BlockID:    other.BlockID,
				Reason:     "operating_hours_overlap",
				Scope:      other.Scope,
				Kind:       other.Kind,
				Effect:     other.Effect,
				Visibility: other.Visibility,
			})
			continue
		}
		otherHardClaim := isHardClaimOccurrence(other.Occurrence)
		scopeConflict := scopeInventoryConflictOccurrences(candidate.Occurrence, other.Occurrence, snapshot)
		coverageBasedConflict := coverageConflict(candidate.coverage, other.coverage, snapshot)
		if (candidateHardClaim && otherHardClaim && coverageBasedConflict) || (!candidateHardClaim || !otherHardClaim) && (scopeConflict || coverageBasedConflict) {
			conflicts = append(conflicts, Conflict{
				BlockID:    other.BlockID,
				Reason:     "effective_inventory_overlap",
				Scope:      other.Scope,
				Kind:       other.Kind,
				Effect:     other.Effect,
				Visibility: other.Visibility,
			})
		}
	}
	return conflicts
}

func sameScopeIdentity(left Block, right Block) bool {
	if left.Scope != right.Scope || left.FacilityKey != right.FacilityKey {
		return false
	}
	switch left.Scope {
	case ScopeFacility:
		return true
	case ScopeZone:
		return left.ZoneKey != nil && right.ZoneKey != nil && *left.ZoneKey == *right.ZoneKey
	case ScopeResource:
		return left.ResourceKey != nil && right.ResourceKey != nil && *left.ResourceKey == *right.ResourceKey
	default:
		return false
	}
}

func sameOccurrenceScope(left Occurrence, right Occurrence) bool {
	return sameScopeIdentity(Block{Scope: left.Scope, FacilityKey: left.FacilityKey, ZoneKey: left.ZoneKey, ResourceKey: left.ResourceKey}, Block{Scope: right.Scope, FacilityKey: right.FacilityKey, ZoneKey: right.ZoneKey, ResourceKey: right.ResourceKey})
}

func blockWindowsOverlap(left Block, right Block) bool {
	return intervalsOverlap(blockWindow(left).From, blockWindow(left).Until, blockWindow(right).From, blockWindow(right).Until)
}

func blockWindow(block Block) CalendarWindow {
	switch block.ScheduleType {
	case ScheduleTypeOneOff:
		if block.StartAt == nil || block.EndAt == nil {
			return CalendarWindow{}
		}
		return CalendarWindow{From: *block.StartAt, Until: *block.EndAt}
	case ScheduleTypeWeekly:
		if block.Weekday == nil || block.StartTime == nil || block.EndTime == nil || block.Timezone == nil || block.RecurrenceStartDate == nil || block.RecurrenceEndDate == nil {
			return CalendarWindow{}
		}
		location, err := time.LoadLocation(*block.Timezone)
		if err != nil {
			return CalendarWindow{}
		}

		startDateParsed, err := time.Parse(isoDateLayout, *block.RecurrenceStartDate)
		if err != nil {
			return CalendarWindow{}
		}
		endDateParsed, err := time.Parse(isoDateLayout, *block.RecurrenceEndDate)
		if err != nil {
			return CalendarWindow{}
		}

		startDate := time.Date(startDateParsed.Year(), startDateParsed.Month(), startDateParsed.Day(), 0, 0, 0, 0, location)
		endDate := time.Date(endDateParsed.Year(), endDateParsed.Month(), endDateParsed.Day(), 0, 0, 0, 0, location)
		if endDate.Before(startDate) {
			return CalendarWindow{}
		}

		startClock, err := time.Parse(hhmmLayout, *block.StartTime)
		if err != nil {
			return CalendarWindow{}
		}
		endClock, err := time.Parse(hhmmLayout, *block.EndTime)
		if err != nil {
			return CalendarWindow{}
		}

		firstDate := alignToWeekday(startDate, *block.Weekday, location)
		if firstDate.After(endDate) {
			return CalendarWindow{}
		}
		lastDate := alignToWeekdayOnOrBefore(endDate, *block.Weekday, location)
		if lastDate.Before(firstDate) {
			return CalendarWindow{}
		}

		firstStart := time.Date(firstDate.Year(), firstDate.Month(), firstDate.Day(), startClock.Hour(), startClock.Minute(), 0, 0, location)
		lastStart := time.Date(lastDate.Year(), lastDate.Month(), lastDate.Day(), startClock.Hour(), startClock.Minute(), 0, 0, location)
		lastEnd := time.Date(lastDate.Year(), lastDate.Month(), lastDate.Day(), endClock.Hour(), endClock.Minute(), 0, 0, location)
		if !lastEnd.After(lastStart) {
			return CalendarWindow{}
		}

		return CalendarWindow{From: firstStart.UTC(), Until: lastEnd.UTC()}
	default:
		return CalendarWindow{}
	}
}

func blockCoverageFromBlock(block Block, snapshot *facilitySnapshot) map[string]struct{} {
	return (&Service{}).blockCoverage(block, snapshot)
}

func claimCoverageFromBlock(block Block, snapshot *facilitySnapshot) map[string]struct{} {
	return (&Service{}).claimCoverage(block, snapshot)
}

func effectiveCoverageFromBlock(block Block, snapshot *facilitySnapshot) map[string]struct{} {
	if isHardClaimBlock(block) {
		return claimCoverageFromBlock(block, snapshot)
	}
	return blockCoverageFromBlock(block, snapshot)
}

func isHardClaimBlock(block Block) bool {
	if block.Kind == KindHold || block.Kind == KindReservation {
		return true
	}
	return block.Kind == KindEvent && block.Effect == EffectHardReserve
}

func isHardClaimOccurrence(occurrence Occurrence) bool {
	if occurrence.Kind == KindHold || occurrence.Kind == KindReservation {
		return true
	}
	return occurrence.Kind == KindEvent && occurrence.Effect == EffectHardReserve
}

func coverageConflict(left map[string]struct{}, right map[string]struct{}, snapshot *facilitySnapshot) bool {
	for leftKey := range left {
		for rightKey := range right {
			if leftKey == rightKey || snapshot.exclusiveConflict(leftKey, rightKey) {
				return true
			}
		}
	}
	return false
}

func resourceIsClaimable(resource store.ApolloScheduleResource) bool {
	return resource.Active && resource.Bookable
}

func scopeInventoryConflictBlocks(left Block, right Block, snapshot *facilitySnapshot) bool {
	if left.FacilityKey != right.FacilityKey {
		return false
	}
	if left.Scope == ScopeFacility || right.Scope == ScopeFacility {
		return true
	}
	if left.Scope == ScopeZone && right.Scope == ScopeZone {
		return left.ZoneKey != nil && right.ZoneKey != nil && *left.ZoneKey == *right.ZoneKey
	}
	if left.Scope == ScopeZone && right.Scope == ScopeResource {
		if left.ZoneKey == nil {
			return false
		}
		return zoneMatchesResource(*left.ZoneKey, right.ZoneKey, right.ResourceKey, snapshot)
	}
	if left.Scope == ScopeResource && right.Scope == ScopeZone {
		if right.ZoneKey == nil {
			return false
		}
		return zoneMatchesResource(*right.ZoneKey, left.ZoneKey, left.ResourceKey, snapshot)
	}
	return false
}

func scopeInventoryConflictBlockOccurrence(left Block, right Occurrence, snapshot *facilitySnapshot) bool {
	return scopeInventoryConflictBlocks(left, Block{
		FacilityKey: right.FacilityKey,
		ZoneKey:     right.ZoneKey,
		ResourceKey: right.ResourceKey,
		Scope:       right.Scope,
	}, snapshot)
}

func scopeInventoryConflictOccurrences(left Occurrence, right Occurrence, snapshot *facilitySnapshot) bool {
	return scopeInventoryConflictBlocks(Block{
		FacilityKey: left.FacilityKey,
		ZoneKey:     left.ZoneKey,
		ResourceKey: left.ResourceKey,
		Scope:       left.Scope,
	}, Block{
		FacilityKey: right.FacilityKey,
		ZoneKey:     right.ZoneKey,
		ResourceKey: right.ResourceKey,
		Scope:       right.Scope,
	}, snapshot)
}

func zoneMatchesResource(zoneKey string, fallbackZone *string, resourceKey *string, snapshot *facilitySnapshot) bool {
	if fallbackZone != nil {
		return *fallbackZone == zoneKey
	}
	if resourceKey == nil {
		return false
	}
	resource, ok := snapshot.resources[*resourceKey]
	if !ok || resource.ZoneKey == nil {
		return false
	}
	return *resource.ZoneKey == zoneKey
}

func (s *Service) blocksHaveOverlappingOccurrences(left Block, right Block, snapshot *facilitySnapshot) (bool, error) {
	window, ok := overlappingBlockWindow(left, right)
	if !ok {
		return false, nil
	}

	leftOccurrences, err := s.expandBlockOccurrences(left, snapshot, window)
	if err != nil {
		return false, err
	}
	rightOccurrences, err := s.expandBlockOccurrences(right, snapshot, window)
	if err != nil {
		return false, err
	}

	return occurrenceCollectionsOverlap(leftOccurrences, rightOccurrences), nil
}

func overlappingBlockWindow(left Block, right Block) (CalendarWindow, bool) {
	leftWindow := blockWindow(left)
	rightWindow := blockWindow(right)
	if leftWindow.From.IsZero() || leftWindow.Until.IsZero() || rightWindow.From.IsZero() || rightWindow.Until.IsZero() {
		return CalendarWindow{}, false
	}
	if !intervalsOverlap(leftWindow.From, leftWindow.Until, rightWindow.From, rightWindow.Until) {
		return CalendarWindow{}, false
	}

	from := leftWindow.From
	if rightWindow.From.After(from) {
		from = rightWindow.From
	}
	until := leftWindow.Until
	if rightWindow.Until.Before(until) {
		until = rightWindow.Until
	}
	if !from.Before(until) {
		return CalendarWindow{}, false
	}

	return CalendarWindow{From: from, Until: until}, true
}

func occurrenceCollectionsOverlap(left []blockOccurrence, right []blockOccurrence) bool {
	for _, leftOccurrence := range left {
		for _, rightOccurrence := range right {
			if intervalsOverlap(leftOccurrence.StartsAt, leftOccurrence.EndsAt, rightOccurrence.StartsAt, rightOccurrence.EndsAt) {
				return true
			}
		}
	}
	return false
}

func occurrenceWindow(occurrence blockOccurrence) CalendarWindow {
	return CalendarWindow{From: occurrence.StartsAt, Until: occurrence.EndsAt}
}

func candidateOccurrenceWindow(occurrences []blockOccurrence) CalendarWindow {
	if len(occurrences) == 0 {
		return CalendarWindow{}
	}

	window := CalendarWindow{From: occurrences[0].StartsAt, Until: occurrences[0].EndsAt}
	for _, occurrence := range occurrences[1:] {
		if occurrence.StartsAt.Before(window.From) {
			window.From = occurrence.StartsAt
		}
		if occurrence.EndsAt.After(window.Until) {
			window.Until = occurrence.EndsAt
		}
	}
	return window
}

func exceptionWithinBlock(block store.ApolloScheduleBlock, exceptionDate pgtype.Date) bool {
	if !exceptionDate.Valid || !block.RecurrenceStartDate.Valid {
		return false
	}
	if exceptionDate.Time.Before(block.RecurrenceStartDate.Time) {
		return false
	}
	if block.RecurrenceEndDate.Valid && exceptionDate.Time.After(block.RecurrenceEndDate.Time) {
		return false
	}
	return true
}

func exceptionMatchesDate(exceptions []string, date time.Time) bool {
	dateString := date.UTC().Format(isoDateLayout)
	for _, exception := range exceptions {
		if exception == dateString {
			return true
		}
	}
	return false
}

func intervalsOverlap(leftStart, leftEnd, rightStart, rightEnd time.Time) bool {
	return leftStart.Before(rightEnd) && rightStart.Before(leftEnd)
}

func normalizedPairKey(left, right string) string {
	if right < left {
		left, right = right, left
	}
	return left + "|" + right
}

func dedupeConflicts(conflicts []Conflict) []Conflict {
	if len(conflicts) <= 1 {
		return conflicts
	}

	seen := make(map[string]struct{}, len(conflicts))
	result := make([]Conflict, 0, len(conflicts))
	for _, conflict := range conflicts {
		key := conflict.BlockID.String() + "|" + conflict.Reason
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, conflict)
	}
	return result
}
