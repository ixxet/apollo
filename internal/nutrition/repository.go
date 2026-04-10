package nutrition

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

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

func (r *Repository) ListMealLogs(ctx context.Context, userID uuid.UUID) ([]MealLog, error) {
	rows, err := r.queries.ListMealLogsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]MealLog, 0, len(rows))
	for _, row := range rows {
		mealLog, err := mealLogFromStore(row)
		if err != nil {
			return nil, err
		}
		result = append(result, mealLog)
	}
	return result, nil
}

func (r *Repository) GetMealTemplate(ctx context.Context, userID uuid.UUID, templateID uuid.UUID) (*MealTemplate, error) {
	row, err := r.queries.GetMealTemplateByIDForUser(ctx, store.GetMealTemplateByIDForUserParams{
		ID:     templateID,
		UserID: userID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	template, err := mealTemplateFromStore(row)
	if err != nil {
		return nil, err
	}
	return &template, nil
}

func (r *Repository) ListMealTemplates(ctx context.Context, userID uuid.UUID) ([]MealTemplate, error) {
	rows, err := r.queries.ListMealTemplatesByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := make([]MealTemplate, 0, len(rows))
	for _, row := range rows {
		template, err := mealTemplateFromStore(row)
		if err != nil {
			return nil, err
		}
		result = append(result, template)
	}
	return result, nil
}

func (r *Repository) CreateMealLog(ctx context.Context, userID uuid.UUID, input PersistedMealLogInput) (MealLog, error) {
	row, err := r.queries.CreateMealLog(ctx, store.CreateMealLogParams{
		UserID:           userID,
		SourceTemplateID: uuidParam(input.SourceTemplateID),
		Name:             input.Name,
		MealType:         input.MealType,
		LoggedAt:         timestamptzParam(input.LoggedAt),
		Calories:         int32Pointer(input.Calories),
		ProteinGrams:     int32Pointer(input.ProteinGrams),
		CarbsGrams:       int32Pointer(input.CarbsGrams),
		FatGrams:         int32Pointer(input.FatGrams),
		Notes:            input.Notes,
	})
	if err != nil {
		return MealLog{}, err
	}

	return mealLogFromStore(row)
}

func (r *Repository) UpdateMealLog(ctx context.Context, userID uuid.UUID, mealLogID uuid.UUID, input PersistedMealLogInput) (*MealLog, error) {
	row, err := r.queries.UpdateMealLogForUser(ctx, store.UpdateMealLogForUserParams{
		ID:               mealLogID,
		UserID:           userID,
		SourceTemplateID: uuidParam(input.SourceTemplateID),
		Name:             input.Name,
		MealType:         input.MealType,
		LoggedAt:         timestamptzParam(input.LoggedAt),
		Calories:         int32Pointer(input.Calories),
		ProteinGrams:     int32Pointer(input.ProteinGrams),
		CarbsGrams:       int32Pointer(input.CarbsGrams),
		FatGrams:         int32Pointer(input.FatGrams),
		Notes:            input.Notes,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	mealLog, err := mealLogFromStore(row)
	if err != nil {
		return nil, err
	}
	return &mealLog, nil
}

func (r *Repository) CreateMealTemplate(ctx context.Context, userID uuid.UUID, input MealTemplateInput) (MealTemplate, error) {
	row, err := r.queries.CreateMealTemplate(ctx, store.CreateMealTemplateParams{
		UserID:       userID,
		Name:         input.Name,
		MealType:     input.MealType,
		Calories:     int32Pointer(input.Calories),
		ProteinGrams: int32Pointer(input.ProteinGrams),
		CarbsGrams:   int32Pointer(input.CarbsGrams),
		FatGrams:     int32Pointer(input.FatGrams),
		Notes:        input.Notes,
	})
	if isUniqueViolation(err) {
		return MealTemplate{}, ErrDuplicateMealTemplateName
	}
	if err != nil {
		return MealTemplate{}, err
	}

	return mealTemplateFromStore(row)
}

func (r *Repository) UpdateMealTemplate(ctx context.Context, userID uuid.UUID, templateID uuid.UUID, input MealTemplateInput) (*MealTemplate, error) {
	row, err := r.queries.UpdateMealTemplateForUser(ctx, store.UpdateMealTemplateForUserParams{
		ID:           templateID,
		UserID:       userID,
		Name:         input.Name,
		MealType:     input.MealType,
		Calories:     int32Pointer(input.Calories),
		ProteinGrams: int32Pointer(input.ProteinGrams),
		CarbsGrams:   int32Pointer(input.CarbsGrams),
		FatGrams:     int32Pointer(input.FatGrams),
		Notes:        input.Notes,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if isUniqueViolation(err) {
		return nil, ErrDuplicateMealTemplateName
	}
	if err != nil {
		return nil, err
	}

	template, err := mealTemplateFromStore(row)
	if err != nil {
		return nil, err
	}
	return &template, nil
}

func mealLogFromStore(row store.ApolloNutritionMealLog) (MealLog, error) {
	if !row.LoggedAt.Valid {
		return MealLog{}, fmt.Errorf("meal log %s missing logged_at", row.ID)
	}

	mealLog := MealLog{
		ID:           row.ID,
		MealType:     row.MealType,
		LoggedAt:     row.LoggedAt.Time.UTC(),
		Name:         row.Name,
		Calories:     intPointer(row.Calories),
		ProteinGrams: intPointer(row.ProteinGrams),
		CarbsGrams:   intPointer(row.CarbsGrams),
		FatGrams:     intPointer(row.FatGrams),
		Notes:        row.Notes,
	}
	if row.SourceTemplateID.Valid {
		templateID := uuid.UUID(row.SourceTemplateID.Bytes)
		mealLog.SourceTemplateID = &templateID
	}

	return mealLog, nil
}

func mealTemplateFromStore(row store.ApolloNutritionMealTemplate) (MealTemplate, error) {
	if !row.CreatedAt.Valid || !row.UpdatedAt.Valid {
		return MealTemplate{}, fmt.Errorf("meal template %s missing timestamps", row.ID)
	}

	return MealTemplate{
		ID:           row.ID,
		Name:         row.Name,
		MealType:     row.MealType,
		Calories:     intPointer(row.Calories),
		ProteinGrams: intPointer(row.ProteinGrams),
		CarbsGrams:   intPointer(row.CarbsGrams),
		FatGrams:     intPointer(row.FatGrams),
		Notes:        row.Notes,
		CreatedAt:    row.CreatedAt.Time.UTC(),
		UpdatedAt:    row.UpdatedAt.Time.UTC(),
	}, nil
}

func uuidParam(value *uuid.UUID) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{
		Bytes: *value,
		Valid: true,
	}
}

func timestamptzParam(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{
		Time:  value.UTC(),
		Valid: true,
	}
}

func int32Pointer(value *int) *int32 {
	if value == nil {
		return nil
	}
	converted := int32(*value)
	return &converted
}

func intPointer(value *int32) *int {
	if value == nil {
		return nil
	}
	converted := int(*value)
	return &converted
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
