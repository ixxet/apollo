package coaching

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/planner"
	"github.com/ixxet/apollo/internal/profile"
)

const (
	ExperienceLevelBeginner     = "beginner"
	ExperienceLevelIntermediate = "intermediate"
	ExperienceLevelAdvanced     = "advanced"

	EffortLevelEasy       = "easy"
	EffortLevelManageable = "manageable"
	EffortLevelHard       = "hard"
	EffortLevelMaxed      = "maxed"

	RecoveryLevelRecovered        = "recovered"
	RecoveryLevelSlightlyFatigued = "slightly_fatigued"
	RecoveryLevelFatigued         = "fatigued"
	RecoveryLevelNotRecovered     = "not_recovered"

	KindStartConservative RecommendationKind = "start_conservative"
	KindProgress          RecommendationKind = "progress"
	KindHold              RecommendationKind = "hold"
	KindDeload            RecommendationKind = "deload"
	KindRegress           RecommendationKind = "regress"

	PolicyVersion = "tracer24/v1"
)

const (
	defaultGoalKey               = "general-fitness"
	defaultDaysPerWeek           = 3
	defaultSessionMinutes        = 45
	minSessionMinutesEvidence    = 20
	maxSessionMinutesEvidence    = 120
	maxWeightKgRecommendation    = 9999.5
	maxPlannerItemWeightBoundary = 9999.99
)

var (
	ErrInvalidEffortLevel   = errors.New("invalid effort_level")
	ErrInvalidRecoveryLevel = errors.New("invalid recovery_level")
	ErrWorkoutNotFound      = errors.New("workout not found")
	ErrWorkoutNotFinished   = errors.New("workout is not finished")
)

type RecommendationKind string

type Clock func() time.Time

type Store interface {
	GetWorkoutByIDForUser(ctx context.Context, workoutID uuid.UUID, userID uuid.UUID) (*WorkoutSnapshot, error)
	GetLatestFinishedWorkoutByUserID(ctx context.Context, userID uuid.UUID) (*WorkoutSnapshot, error)
	UpsertEffortFeedbackForFinishedWorkout(ctx context.Context, workoutID uuid.UUID, userID uuid.UUID, effortLevel string) (*EffortFeedbackRecord, error)
	UpsertRecoveryFeedbackForFinishedWorkout(ctx context.Context, workoutID uuid.UUID, userID uuid.UUID, recoveryLevel string) (*RecoveryFeedbackRecord, error)
	GetEffortFeedbackByWorkoutID(ctx context.Context, workoutID uuid.UUID) (*EffortFeedbackRecord, error)
	GetRecoveryFeedbackByWorkoutID(ctx context.Context, workoutID uuid.UUID) (*RecoveryFeedbackRecord, error)
}

type PlannerReader interface {
	GetWeek(ctx context.Context, userID uuid.UUID, weekStart string) (planner.Week, error)
}

type ProfileReader interface {
	GetProfile(ctx context.Context, userID uuid.UUID) (profile.MemberProfile, error)
}

type Service struct {
	store         Store
	plannerReader PlannerReader
	profileReader ProfileReader
	now           Clock
}

type WorkoutSnapshot struct {
	ID         uuid.UUID
	Status     string
	FinishedAt *time.Time
}

type EffortFeedbackRecord struct {
	WorkoutID   uuid.UUID
	EffortLevel string
	CreatedAt   time.Time
}

type RecoveryFeedbackRecord struct {
	WorkoutID     uuid.UUID
	RecoveryLevel string
	CreatedAt     time.Time
}

type EffortFeedbackInput struct {
	EffortLevel string `json:"effort_level"`
}

type RecoveryFeedbackInput struct {
	RecoveryLevel string `json:"recovery_level"`
}

type EffortFeedback struct {
	WorkoutID   uuid.UUID `json:"workout_id"`
	EffortLevel string    `json:"effort_level"`
	CreatedAt   time.Time `json:"created_at"`
}

type RecoveryFeedback struct {
	WorkoutID     uuid.UUID `json:"workout_id"`
	RecoveryLevel string    `json:"recovery_level"`
	CreatedAt     time.Time `json:"created_at"`
}

type TrainingGoal struct {
	GoalKey              string `json:"goal_key"`
	TargetDaysPerWeek    int    `json:"target_days_per_week"`
	TargetSessionMinutes int    `json:"target_session_minutes"`
}

type PlanItemValues struct {
	Sets     int      `json:"sets"`
	Reps     int      `json:"reps"`
	WeightKg *float64 `json:"weight_kg,omitempty"`
	RPE      *float64 `json:"rpe,omitempty"`
}

type PlanChange struct {
	DayIndex        int            `json:"day_index"`
	SessionPosition int            `json:"session_position"`
	ItemPosition    int            `json:"item_position"`
	Before          PlanItemValues `json:"before"`
	After           PlanItemValues `json:"after"`
}

type PlanChangeProposal struct {
	WeekStart string       `json:"week_start"`
	Changes   []PlanChange `json:"changes"`
}

type CoachingExplanationEvidence struct {
	GoalKey              string     `json:"goal_key"`
	TargetDaysPerWeek    int        `json:"target_days_per_week"`
	TargetSessionMinutes int        `json:"target_session_minutes"`
	ExperienceLevel      *string    `json:"experience_level,omitempty"`
	SourceWorkoutID      *uuid.UUID `json:"source_workout_id,omitempty"`
	EffortLevel          *string    `json:"effort_level,omitempty"`
	RecoveryLevel        *string    `json:"recovery_level,omitempty"`
}

type CoachingExplanation struct {
	ReasonCodes []string                    `json:"reason_codes"`
	Evidence    CoachingExplanationEvidence `json:"evidence"`
	Limitations []string                    `json:"limitations"`
}

type CoachingRecommendation struct {
	Kind            RecommendationKind  `json:"kind"`
	TargetWeekStart string              `json:"target_week_start"`
	SourceWorkoutID *uuid.UUID          `json:"source_workout_id,omitempty"`
	TrainingGoal    TrainingGoal        `json:"training_goal"`
	Proposal        PlanChangeProposal  `json:"proposal"`
	Explanation     CoachingExplanation `json:"explanation"`
	PolicyVersion   string              `json:"policy_version"`
	GeneratedAt     time.Time           `json:"generated_at"`
}

func NewService(store Store, plannerReader PlannerReader, profileReader ProfileReader) *Service {
	return &Service{
		store:         store,
		plannerReader: plannerReader,
		profileReader: profileReader,
		now:           time.Now,
	}
}

func (s *Service) PutEffortFeedback(ctx context.Context, userID uuid.UUID, workoutID uuid.UUID, input EffortFeedbackInput) (EffortFeedback, error) {
	level, ok := normalizeEffortLevel(input.EffortLevel)
	if !ok {
		return EffortFeedback{}, ErrInvalidEffortLevel
	}

	workout, err := s.store.GetWorkoutByIDForUser(ctx, workoutID, userID)
	if err != nil {
		return EffortFeedback{}, err
	}
	if workout == nil {
		return EffortFeedback{}, ErrWorkoutNotFound
	}
	if workout.Status != "finished" {
		return EffortFeedback{}, ErrWorkoutNotFinished
	}

	record, err := s.store.UpsertEffortFeedbackForFinishedWorkout(ctx, workoutID, userID, level)
	if err != nil {
		return EffortFeedback{}, err
	}
	if record == nil {
		return EffortFeedback{}, ErrWorkoutNotFinished
	}

	return EffortFeedback{
		WorkoutID:   record.WorkoutID,
		EffortLevel: record.EffortLevel,
		CreatedAt:   record.CreatedAt.UTC(),
	}, nil
}

func (s *Service) PutRecoveryFeedback(ctx context.Context, userID uuid.UUID, workoutID uuid.UUID, input RecoveryFeedbackInput) (RecoveryFeedback, error) {
	level, ok := normalizeRecoveryLevel(input.RecoveryLevel)
	if !ok {
		return RecoveryFeedback{}, ErrInvalidRecoveryLevel
	}

	workout, err := s.store.GetWorkoutByIDForUser(ctx, workoutID, userID)
	if err != nil {
		return RecoveryFeedback{}, err
	}
	if workout == nil {
		return RecoveryFeedback{}, ErrWorkoutNotFound
	}
	if workout.Status != "finished" {
		return RecoveryFeedback{}, ErrWorkoutNotFinished
	}

	record, err := s.store.UpsertRecoveryFeedbackForFinishedWorkout(ctx, workoutID, userID, level)
	if err != nil {
		return RecoveryFeedback{}, err
	}
	if record == nil {
		return RecoveryFeedback{}, ErrWorkoutNotFinished
	}

	return RecoveryFeedback{
		WorkoutID:     record.WorkoutID,
		RecoveryLevel: record.RecoveryLevel,
		CreatedAt:     record.CreatedAt.UTC(),
	}, nil
}

func (s *Service) GetCoachingRecommendation(ctx context.Context, userID uuid.UUID, weekStart string) (CoachingRecommendation, error) {
	week, err := s.plannerReader.GetWeek(ctx, userID, strings.TrimSpace(weekStart))
	if err != nil {
		return CoachingRecommendation{}, err
	}

	memberProfile, err := s.profileReader.GetProfile(ctx, userID)
	if err != nil {
		return CoachingRecommendation{}, err
	}

	generatedAt := s.now().UTC()
	trainingGoal, experienceLevel, reasonCodes, limitations := deriveTrainingGoal(memberProfile.CoachingProfile)
	evidence := CoachingExplanationEvidence{
		GoalKey:              trainingGoal.GoalKey,
		TargetDaysPerWeek:    trainingGoal.TargetDaysPerWeek,
		TargetSessionMinutes: trainingGoal.TargetSessionMinutes,
		ExperienceLevel:      stringPtr(experienceLevel),
	}

	if !weekHasPlannerItems(week) {
		reasonCodes = append(reasonCodes, "no_planner_items")
		limitations = append(limitations, "no existing planner truth to adjust")
		proposal := PlanChangeProposal{
			WeekStart: week.WeekStart,
			Changes:   []PlanChange{},
		}
		return CoachingRecommendation{
			Kind:            KindHold,
			TargetWeekStart: week.WeekStart,
			TrainingGoal:    trainingGoal,
			Proposal:        proposal,
			Explanation: CoachingExplanation{
				ReasonCodes: reasonCodes,
				Evidence:    evidence,
				Limitations: limitations,
			},
			PolicyVersion: PolicyVersion,
			GeneratedAt:   generatedAt,
		}, nil
	}

	latestWorkout, err := s.store.GetLatestFinishedWorkoutByUserID(ctx, userID)
	if err != nil {
		return CoachingRecommendation{}, err
	}

	kind := KindStartConservative
	var sourceWorkoutID *uuid.UUID

	if latestWorkout == nil {
		reasonCodes = append(reasonCodes, "no_finished_workout")
		limitations = append(limitations, "missing finished workout history")
	} else {
		sourceWorkoutID = uuidPtr(latestWorkout.ID)
		evidence.SourceWorkoutID = uuidPtr(latestWorkout.ID)

		effortFeedback, err := s.store.GetEffortFeedbackByWorkoutID(ctx, latestWorkout.ID)
		if err != nil {
			return CoachingRecommendation{}, err
		}
		recoveryFeedback, err := s.store.GetRecoveryFeedbackByWorkoutID(ctx, latestWorkout.ID)
		if err != nil {
			return CoachingRecommendation{}, err
		}

		if effortFeedback == nil || recoveryFeedback == nil {
			reasonCodes = append(reasonCodes, "missing_feedback_for_latest_finished_workout")
			limitations = append(limitations, "missing effort or recovery feedback for latest finished workout")
		} else {
			evidence.EffortLevel = stringPtr(effortFeedback.EffortLevel)
			evidence.RecoveryLevel = stringPtr(recoveryFeedback.RecoveryLevel)
			kind = evaluateRecommendationKind(effortFeedback.EffortLevel, recoveryFeedback.RecoveryLevel)
			switch kind {
			case KindRegress:
				reasonCodes = append(reasonCodes, "effort_maxed_or_recovery_not_recovered")
			case KindDeload:
				reasonCodes = append(reasonCodes, "effort_hard_or_recovery_fatigued")
			case KindProgress:
				reasonCodes = append(reasonCodes, "effort_easy_and_recovery_recovered")
			default:
				reasonCodes = append(reasonCodes, "default_hold_with_feedback")
			}
		}
	}

	proposal := buildPlanChangeProposal(week, kind, experienceLevel)
	return CoachingRecommendation{
		Kind:            kind,
		TargetWeekStart: week.WeekStart,
		SourceWorkoutID: sourceWorkoutID,
		TrainingGoal:    trainingGoal,
		Proposal:        proposal,
		Explanation: CoachingExplanation{
			ReasonCodes: reasonCodes,
			Evidence:    evidence,
			Limitations: limitations,
		},
		PolicyVersion: PolicyVersion,
		GeneratedAt:   generatedAt,
	}, nil
}

func deriveTrainingGoal(current profile.CoachingProfile) (TrainingGoal, string, []string, []string) {
	reasonCodes := []string{}
	limitations := []string{}

	goalKey := defaultGoalKey
	if current.GoalKey == nil {
		reasonCodes = append(reasonCodes, "goal_key_defaulted")
		limitations = append(limitations, "goal_key missing; defaulted to general-fitness")
	} else {
		trimmed := strings.TrimSpace(*current.GoalKey)
		if trimmed == "" {
			reasonCodes = append(reasonCodes, "goal_key_defaulted")
			limitations = append(limitations, "goal_key missing; defaulted to general-fitness")
		} else {
			goalKey = trimmed
		}
	}

	targetDays := defaultDaysPerWeek
	if current.DaysPerWeek == nil {
		reasonCodes = append(reasonCodes, "target_days_per_week_defaulted")
		limitations = append(limitations, "days_per_week missing; defaulted to 3")
	} else if *current.DaysPerWeek < 1 || *current.DaysPerWeek > 7 {
		reasonCodes = append(reasonCodes, "target_days_per_week_defaulted")
		limitations = append(limitations, "days_per_week out of range; defaulted to 3")
	} else {
		targetDays = *current.DaysPerWeek
	}

	targetMinutes := defaultSessionMinutes
	if current.SessionMinutes == nil {
		reasonCodes = append(reasonCodes, "target_session_minutes_defaulted")
		limitations = append(limitations, "session_minutes missing; defaulted to 45")
	} else {
		targetMinutes = clampInt(*current.SessionMinutes, minSessionMinutesEvidence, maxSessionMinutesEvidence)
		if targetMinutes != *current.SessionMinutes {
			reasonCodes = append(reasonCodes, "target_session_minutes_clamped")
			limitations = append(limitations, "session_minutes clamped to conservative range")
		}
	}

	experienceLevel := ExperienceLevelBeginner
	if current.ExperienceLevel == nil || !isValidExperienceLevel(*current.ExperienceLevel) {
		reasonCodes = append(reasonCodes, "experience_level_defaulted_beginner")
		limitations = append(limitations, "experience_level missing or unknown; defaulted to beginner")
	} else {
		experienceLevel = *current.ExperienceLevel
	}

	return TrainingGoal{
		GoalKey:              goalKey,
		TargetDaysPerWeek:    targetDays,
		TargetSessionMinutes: targetMinutes,
	}, experienceLevel, reasonCodes, limitations
}

func evaluateRecommendationKind(effortLevel string, recoveryLevel string) RecommendationKind {
	if effortLevel == EffortLevelMaxed || recoveryLevel == RecoveryLevelNotRecovered {
		return KindRegress
	}
	if effortLevel == EffortLevelHard || recoveryLevel == RecoveryLevelFatigued {
		return KindDeload
	}
	if effortLevel == EffortLevelEasy && recoveryLevel == RecoveryLevelRecovered {
		return KindProgress
	}
	return KindHold
}

func weekHasPlannerItems(week planner.Week) bool {
	for _, session := range week.Sessions {
		if len(session.Items) > 0 {
			return true
		}
	}
	return false
}

func buildPlanChangeProposal(week planner.Week, kind RecommendationKind, experienceLevel string) PlanChangeProposal {
	changes := make([]PlanChange, 0)
	weightFactor := weightFactorForKind(kind, experienceLevel)
	for _, session := range week.Sessions {
		for _, item := range session.Items {
			before := PlanItemValues{
				Sets:     item.Sets,
				Reps:     item.Reps,
				WeightKg: cloneFloatPtr(item.WeightKg),
				RPE:      cloneFloatPtr(item.RPE),
			}
			after := PlanItemValues{
				Sets:     before.Sets,
				Reps:     before.Reps,
				WeightKg: cloneFloatPtr(before.WeightKg),
				RPE:      cloneFloatPtr(before.RPE),
			}

			changed := false
			if before.WeightKg != nil {
				targetWeight := clampWeightKg(roundToNearestHalf(*before.WeightKg * weightFactor))
				if !floatEqual(*before.WeightKg, targetWeight) {
					after.WeightKg = floatPtr(targetWeight)
					changed = true
				}
			} else {
				targetRPE := targetRPEForKind(kind, before.RPE)
				targetRPE = clampFloat(targetRPE, 0, 10)
				if before.RPE == nil || !floatEqual(*before.RPE, targetRPE) {
					after.RPE = floatPtr(targetRPE)
					changed = true
				}
			}

			if !changed {
				continue
			}

			changes = append(changes, PlanChange{
				DayIndex:        session.DayIndex,
				SessionPosition: session.Position,
				ItemPosition:    item.Position,
				Before:          before,
				After:           after,
			})
		}
	}

	return PlanChangeProposal{
		WeekStart: week.WeekStart,
		Changes:   changes,
	}
}

func weightFactorForKind(kind RecommendationKind, experienceLevel string) float64 {
	switch kind {
	case KindStartConservative:
		switch experienceLevel {
		case ExperienceLevelAdvanced:
			return 0.95
		case ExperienceLevelIntermediate:
			return 0.90
		default:
			return 0.85
		}
	case KindProgress:
		return 1.025
	case KindDeload:
		return 0.90
	case KindRegress:
		return 0.85
	default:
		return 1.0
	}
}

func targetRPEForKind(kind RecommendationKind, current *float64) float64 {
	switch kind {
	case KindStartConservative, KindDeload, KindRegress:
		return 6.0
	case KindProgress:
		if current == nil {
			return 7.0
		}
		return math.Min(*current+0.5, 8.0)
	default:
		if current == nil {
			return 6.5
		}
		return *current
	}
}

func normalizeEffortLevel(value string) (string, bool) {
	normalized := strings.TrimSpace(strings.ToLower(value))
	switch normalized {
	case EffortLevelEasy, EffortLevelManageable, EffortLevelHard, EffortLevelMaxed:
		return normalized, true
	default:
		return "", false
	}
}

func normalizeRecoveryLevel(value string) (string, bool) {
	normalized := strings.TrimSpace(strings.ToLower(value))
	switch normalized {
	case RecoveryLevelRecovered, RecoveryLevelSlightlyFatigued, RecoveryLevelFatigued, RecoveryLevelNotRecovered:
		return normalized, true
	default:
		return "", false
	}
}

func isValidExperienceLevel(value string) bool {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case ExperienceLevelBeginner, ExperienceLevelIntermediate, ExperienceLevelAdvanced:
		return true
	default:
		return false
	}
}

func roundToNearestHalf(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Round(value*2) / 2
}

func clampWeightKg(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	if value < 0 {
		return 0
	}
	if value > maxPlannerItemWeightBoundary {
		return maxWeightKgRecommendation
	}
	return value
}

func clampFloat(value float64, min float64, max float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return min
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func clampInt(value int, min int, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func floatEqual(left float64, right float64) bool {
	return math.Abs(left-right) < 0.000001
}

func cloneFloatPtr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	return floatPtr(*value)
}

func floatPtr(value float64) *float64 {
	result := value
	return &result
}

func stringPtr(value string) *string {
	result := value
	return &result
}

func uuidPtr(value uuid.UUID) *uuid.UUID {
	result := value
	return &result
}
