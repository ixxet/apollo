package sports

import (
	"context"
	"errors"

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

func (r *Repository) ListSports(ctx context.Context) ([]Sport, error) {
	rows, err := r.queries.ListSports(ctx)
	if err != nil {
		return nil, err
	}

	sports := make([]Sport, 0, len(rows))
	for _, row := range rows {
		sports = append(sports, Sport{
			SportKey:                    row.SportKey,
			DisplayName:                 row.DisplayName,
			Description:                 row.Description,
			CompetitionMode:             row.CompetitionMode,
			SidesPerMatch:               int(row.SidesPerMatch),
			ParticipantsPerSideMin:      int(row.ParticipantsPerSideMin),
			ParticipantsPerSideMax:      int(row.ParticipantsPerSideMax),
			ScoringModel:                row.ScoringModel,
			DefaultMatchDurationMinutes: int(row.DefaultMatchDurationMinutes),
			RulesSummary:                row.RulesSummary,
		})
	}

	return sports, nil
}

func (r *Repository) GetSportByKey(ctx context.Context, sportKey string) (*Sport, error) {
	row, err := r.queries.GetSportByKey(ctx, sportKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &Sport{
		SportKey:                    row.SportKey,
		DisplayName:                 row.DisplayName,
		Description:                 row.Description,
		CompetitionMode:             row.CompetitionMode,
		SidesPerMatch:               int(row.SidesPerMatch),
		ParticipantsPerSideMin:      int(row.ParticipantsPerSideMin),
		ParticipantsPerSideMax:      int(row.ParticipantsPerSideMax),
		ScoringModel:                row.ScoringModel,
		DefaultMatchDurationMinutes: int(row.DefaultMatchDurationMinutes),
		RulesSummary:                row.RulesSummary,
	}, nil
}

func (r *Repository) FacilityExists(ctx context.Context, facilityKey string) (bool, error) {
	return r.queries.FacilityCatalogRefExists(ctx, facilityKey)
}

func (r *Repository) ListFacilityCapabilities(ctx context.Context) ([]FacilityCapability, error) {
	rows, err := r.queries.ListSportFacilityCapabilities(ctx)
	if err != nil {
		return nil, err
	}

	capabilities := make([]FacilityCapability, 0, len(rows))
	indexByKey := make(map[string]int, len(rows))
	for _, row := range rows {
		key := row.SportKey + "\x00" + row.FacilityKey
		index, exists := indexByKey[key]
		if !exists {
			index = len(capabilities)
			indexByKey[key] = index
			capabilities = append(capabilities, FacilityCapability{
				SportKey:    row.SportKey,
				FacilityKey: row.FacilityKey,
				ZoneKeys:    nil,
			})
		}

		if row.ZoneKey != nil {
			capabilities[index].ZoneKeys = append(capabilities[index].ZoneKeys, *row.ZoneKey)
		}
	}

	return capabilities, nil
}
