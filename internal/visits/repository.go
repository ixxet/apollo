package visits

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ixxet/apollo/internal/store"
)

type Repository struct {
	queries *store.Queries
}

func NewRepository(db store.DBTX) *Repository {
	return &Repository{
		queries: store.New(db),
	}
}

func (r *Repository) FindActiveUserByTagHash(ctx context.Context, tagHash string) (*store.ApolloUser, error) {
	user, err := r.queries.GetActiveUserByTagHash(ctx, tagHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *Repository) GetVisitBySourceEventID(ctx context.Context, sourceEventID string) (*store.ApolloVisit, error) {
	visit, err := r.queries.GetVisitBySourceEventID(ctx, &sourceEventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &visit, nil
}

func (r *Repository) GetVisitByDepartureSourceEventID(ctx context.Context, sourceEventID string) (*store.ApolloVisit, error) {
	visit, err := r.queries.GetVisitByDepartureSourceEventID(ctx, &sourceEventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &visit, nil
}

func (r *Repository) GetOpenVisitByUserAndFacility(ctx context.Context, userID uuid.UUID, facilityKey string) (*store.ApolloVisit, error) {
	visit, err := r.queries.GetOpenVisitByUserAndFacility(ctx, store.GetOpenVisitByUserAndFacilityParams{
		UserID:      userID,
		FacilityKey: facilityKey,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &visit, nil
}

func (r *Repository) CloseVisit(ctx context.Context, params store.CloseVisitParams) (*store.ApolloVisit, error) {
	row, err := r.queries.CloseVisit(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &store.ApolloVisit{
		ID:                     row.ID,
		UserID:                 row.UserID,
		FacilityKey:            row.FacilityKey,
		ZoneKey:                row.ZoneKey,
		SourceEventID:          row.SourceEventID,
		ArrivedAt:              row.ArrivedAt,
		DepartedAt:             row.DepartedAt,
		Metadata:               row.Metadata,
		DepartureSourceEventID: row.DepartureSourceEventID,
	}, nil
}

func (r *Repository) CreateVisit(ctx context.Context, params store.CreateVisitParams) (*store.ApolloVisit, error) {
	visit, err := r.queries.CreateVisit(ctx, params)
	if err != nil {
		return nil, err
	}

	return &store.ApolloVisit{
		ID:            visit.ID,
		UserID:        visit.UserID,
		FacilityKey:   visit.FacilityKey,
		ZoneKey:       visit.ZoneKey,
		SourceEventID: visit.SourceEventID,
		ArrivedAt:     visit.ArrivedAt,
		DepartedAt:    visit.DepartedAt,
		Metadata:      visit.Metadata,
	}, nil
}

func (r *Repository) ListByStudentID(ctx context.Context, studentID string) ([]store.ApolloVisit, error) {
	return r.queries.ListVisitsByStudentID(ctx, studentID)
}
