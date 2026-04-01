package visits

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ixxet/apollo/internal/store"
)

type Outcome string

const (
	OutcomeCreated          Outcome = "created"
	OutcomeDuplicate        Outcome = "duplicate"
	OutcomeAlreadyOpen      Outcome = "already_open"
	OutcomeUnknownTag       Outcome = "unknown_tag"
	OutcomeIgnoredAnonymous Outcome = "ignored_anonymous"
)

type Finder interface {
	FindActiveUserByTagHash(ctx context.Context, tagHash string) (*store.ApolloUser, error)
	GetVisitBySourceEventID(ctx context.Context, sourceEventID string) (*store.ApolloVisit, error)
	GetOpenVisitByUserAndFacility(ctx context.Context, userID uuid.UUID, facilityKey string) (*store.ApolloVisit, error)
	CreateVisit(ctx context.Context, params store.CreateVisitParams) (*store.ApolloVisit, error)
}

type Service struct {
	repository Finder
}

type ArrivalInput struct {
	SourceEventID        string
	FacilityKey          string
	ZoneKey              *string
	ExternalIdentityHash string
	ArrivedAt            time.Time
}

type Result struct {
	Outcome Outcome
	Visit   *store.ApolloVisit
}

func NewService(repository Finder) *Service {
	return &Service{repository: repository}
}

func (s *Service) RecordArrival(ctx context.Context, input ArrivalInput) (Result, error) {
	if strings.TrimSpace(input.ExternalIdentityHash) == "" {
		return Result{Outcome: OutcomeIgnoredAnonymous}, nil
	}
	if strings.TrimSpace(input.SourceEventID) == "" {
		return Result{}, fmt.Errorf("arrival missing source_event_id")
	}
	if strings.TrimSpace(input.FacilityKey) == "" {
		return Result{}, fmt.Errorf("arrival missing facility_key")
	}
	if input.ArrivedAt.IsZero() {
		return Result{}, fmt.Errorf("arrival missing arrived_at")
	}

	user, err := s.repository.FindActiveUserByTagHash(ctx, input.ExternalIdentityHash)
	if err != nil {
		return Result{}, err
	}
	if user == nil {
		return Result{Outcome: OutcomeUnknownTag}, nil
	}

	existingBySource, err := s.repository.GetVisitBySourceEventID(ctx, input.SourceEventID)
	if err != nil {
		return Result{}, err
	}
	if existingBySource != nil {
		return Result{Outcome: OutcomeDuplicate, Visit: existingBySource}, nil
	}

	openVisit, err := s.repository.GetOpenVisitByUserAndFacility(ctx, user.ID, input.FacilityKey)
	if err != nil {
		return Result{}, err
	}
	if openVisit != nil {
		return Result{Outcome: OutcomeAlreadyOpen, Visit: openVisit}, nil
	}

	visit, err := s.repository.CreateVisit(ctx, store.CreateVisitParams{
		UserID:        user.ID,
		FacilityKey:   input.FacilityKey,
		ZoneKey:       input.ZoneKey,
		SourceEventID: &input.SourceEventID,
		ArrivedAt: pgtype.Timestamptz{
			Time:  input.ArrivedAt.UTC(),
			Valid: true,
		},
	})
	if err != nil {
		if isUniqueViolation(err) {
			if existingBySource, lookupErr := s.repository.GetVisitBySourceEventID(ctx, input.SourceEventID); lookupErr == nil && existingBySource != nil {
				return Result{Outcome: OutcomeDuplicate, Visit: existingBySource}, nil
			}
			if openVisit, lookupErr := s.repository.GetOpenVisitByUserAndFacility(ctx, user.ID, input.FacilityKey); lookupErr == nil && openVisit != nil {
				return Result{Outcome: OutcomeAlreadyOpen, Visit: openVisit}, nil
			}
		}
		return Result{}, err
	}

	return Result{Outcome: OutcomeCreated, Visit: visit}, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	return pgErr.Code == "23505"
}
