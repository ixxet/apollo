package exercises

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

type EquipmentDefinition struct {
	EquipmentKey string `json:"equipment_key"`
	DisplayName  string `json:"display_name"`
	IsMachine    bool   `json:"is_machine"`
}

type ExerciseDefinition struct {
	ExerciseKey          string   `json:"exercise_key"`
	DisplayName          string   `json:"display_name"`
	AllowedEquipmentKeys []string `json:"allowed_equipment_keys"`
}

type EquipmentRef struct {
	ID           uuid.UUID
	EquipmentKey string
	DisplayName  string
	IsMachine    bool
}

type ExerciseRef struct {
	ID                   uuid.UUID
	ExerciseKey          string
	DisplayName          string
	AllowedEquipmentKeys []string
	allowedEquipmentSet  map[string]struct{}
}

type Store interface {
	ListEquipment(ctx context.Context) ([]EquipmentRef, error)
	ResolveEquipment(ctx context.Context, keys []string) (map[string]EquipmentRef, error)
	ListExercises(ctx context.Context) ([]ExerciseRef, error)
	ResolveExercises(ctx context.Context, keys []string) (map[string]ExerciseRef, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) ListEquipment(ctx context.Context) ([]EquipmentDefinition, error) {
	items, err := s.store.ListEquipment(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]EquipmentDefinition, 0, len(items))
	for _, item := range items {
		result = append(result, EquipmentDefinition{
			EquipmentKey: item.EquipmentKey,
			DisplayName:  item.DisplayName,
			IsMachine:    item.IsMachine,
		})
	}

	return result, nil
}

func (s *Service) ResolveEquipment(ctx context.Context, keys []string) (map[string]EquipmentRef, error) {
	return s.store.ResolveEquipment(ctx, normalizeKeys(keys))
}

func (s *Service) ListExercises(ctx context.Context) ([]ExerciseDefinition, error) {
	items, err := s.store.ListExercises(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]ExerciseDefinition, 0, len(items))
	for _, item := range items {
		result = append(result, ExerciseDefinition{
			ExerciseKey:          item.ExerciseKey,
			DisplayName:          item.DisplayName,
			AllowedEquipmentKeys: append([]string(nil), item.AllowedEquipmentKeys...),
		})
	}

	return result, nil
}

func (s *Service) ResolveExercises(ctx context.Context, keys []string) (map[string]ExerciseRef, error) {
	return s.store.ResolveExercises(ctx, normalizeKeys(keys))
}

func (e ExerciseRef) AllowsEquipmentKey(key string) bool {
	if key == "" {
		return true
	}
	_, ok := e.allowedEquipmentSet[key]
	return ok
}

func normalizeKeys(keys []string) []string {
	if len(keys) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(keys))
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		normalized := strings.TrimSpace(key)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}

	return result
}
