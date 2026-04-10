package coaching

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/helper"
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

const (
	WhyTopicRecommendation = "recommendation"
	WhyTopicGoal           = "goal"
	WhyTopicProposal       = "proposal"
	WhyTopicFeedback       = "feedback"

	VariationEasier = "easier"
	VariationHarder = "harder"
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

type CoachingHelperRead struct {
	PreviewMode      helper.PreviewMode `json:"preview_mode"`
	Recommendation   CoachingRecommendation
	Summary          helper.Summary  `json:"summary"`
	WhyOptions       []helper.Option `json:"why_options"`
	VariationOptions []helper.Option `json:"variation_options"`
}

type CoachingHelperWhy struct {
	PreviewMode    helper.PreviewMode     `json:"preview_mode"`
	Topic          string                 `json:"topic"`
	Recommendation CoachingRecommendation `json:"recommendation"`
	Summary        helper.Summary         `json:"summary"`
}

type CoachingVariationPreview struct {
	PreviewMode    helper.PreviewMode     `json:"preview_mode"`
	Variation      string                 `json:"variation"`
	BaseKind       RecommendationKind     `json:"base_kind"`
	Recommendation CoachingRecommendation `json:"recommendation"`
	Summary        helper.Summary         `json:"summary"`
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
	return s.getCoachingRecommendation(ctx, userID, weekStart, nil)
}

func (s *Service) GetHelperRead(ctx context.Context, userID uuid.UUID, weekStart string) (CoachingHelperRead, error) {
	recommendation, err := s.GetCoachingRecommendation(ctx, userID, weekStart)
	if err != nil {
		return CoachingHelperRead{}, err
	}

	return CoachingHelperRead{
		PreviewMode:      helper.PreviewModeReadOnly,
		Recommendation:   recommendation,
		Summary:          buildHelperSummary(recommendation),
		WhyOptions:       helperOptionsForWhy(),
		VariationOptions: helperOptionsForVariation(),
	}, nil
}

func (s *Service) AskWhy(ctx context.Context, userID uuid.UUID, weekStart string, topic string) (CoachingHelperWhy, error) {
	recommendation, err := s.GetCoachingRecommendation(ctx, userID, weekStart)
	if err != nil {
		return CoachingHelperWhy{}, err
	}

	summary, err := buildWhySummary(recommendation, topic)
	if err != nil {
		return CoachingHelperWhy{}, err
	}

	return CoachingHelperWhy{
		PreviewMode:    helper.PreviewModeReadOnly,
		Topic:          normalizeHelperKey(topic),
		Recommendation: recommendation,
		Summary:        summary,
	}, nil
}

func (s *Service) PreviewVariation(ctx context.Context, userID uuid.UUID, weekStart string, variation string) (CoachingVariationPreview, error) {
	baseRecommendation, err := s.GetCoachingRecommendation(ctx, userID, weekStart)
	if err != nil {
		return CoachingVariationPreview{}, err
	}

	variantKind, err := variationKindFor(baseRecommendation.Kind, variation)
	if err != nil {
		return CoachingVariationPreview{}, err
	}

	variantRecommendation, err := s.getCoachingRecommendation(ctx, userID, weekStart, &variantKind)
	if err != nil {
		return CoachingVariationPreview{}, err
	}

	return CoachingVariationPreview{
		PreviewMode:    helper.PreviewModeReadOnly,
		Variation:      normalizeHelperKey(variation),
		BaseKind:       baseRecommendation.Kind,
		Recommendation: variantRecommendation,
		Summary:        buildVariationSummary(baseRecommendation, variantRecommendation, variation),
	}, nil
}

func (s *Service) getCoachingRecommendation(ctx context.Context, userID uuid.UUID, weekStart string, previewKind *RecommendationKind) (CoachingRecommendation, error) {
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
		if previewKind != nil {
			limitations = append(limitations, "helper variation preview stays read-only and requires planner truth to produce a new diff")
		}
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

	if previewKind != nil {
		kind = *previewKind
		reasonCodes = append(reasonCodes, "helper_variation_preview")
		limitations = append(limitations, "helper variation previews are read-only and do not mutate planner truth")
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

func helperOptionsForWhy() []helper.Option {
	return []helper.Option{
		{Key: WhyTopicRecommendation, Label: "Why this recommendation"},
		{Key: WhyTopicGoal, Label: "Why this goal setup"},
		{Key: WhyTopicProposal, Label: "Why these plan changes"},
		{Key: WhyTopicFeedback, Label: "How feedback affected it"},
	}
}

func helperOptionsForVariation() []helper.Option {
	return []helper.Option{
		{Key: VariationEasier, Label: "Make it easier"},
		{Key: VariationHarder, Label: "Make it harder"},
	}
}

func buildHelperSummary(recommendation CoachingRecommendation) helper.Summary {
	bullets := []string{
		fmt.Sprintf("training goal: %s (%d days/week, %d minutes)", recommendation.TrainingGoal.GoalKey, recommendation.TrainingGoal.TargetDaysPerWeek, recommendation.TrainingGoal.TargetSessionMinutes),
		fmt.Sprintf("planner preview changes: %d item(s)", len(recommendation.Proposal.Changes)),
	}
	if recommendation.Explanation.Evidence.EffortLevel != nil || recommendation.Explanation.Evidence.RecoveryLevel != nil {
		effort := "missing"
		if recommendation.Explanation.Evidence.EffortLevel != nil {
			effort = *recommendation.Explanation.Evidence.EffortLevel
		}
		recovery := "missing"
		if recommendation.Explanation.Evidence.RecoveryLevel != nil {
			recovery = *recommendation.Explanation.Evidence.RecoveryLevel
		}
		bullets = append(bullets, fmt.Sprintf("latest feedback: effort=%s, recovery=%s", effort, recovery))
	} else {
		bullets = append(bullets, "latest feedback: unavailable or incomplete")
	}

	return helper.Summary{
		Headline:    headlineForKind(recommendation.Kind),
		Detail:      detailForSummary(recommendation),
		Bullets:     bullets,
		Limitations: append([]string(nil), recommendation.Explanation.Limitations...),
	}
}

func buildWhySummary(recommendation CoachingRecommendation, topic string) (helper.Summary, error) {
	switch normalizeHelperKey(topic) {
	case WhyTopicRecommendation:
		return helper.Summary{
			Headline:    "Why this coaching recommendation",
			Detail:      detailForSummary(recommendation),
			Bullets:     []string{fmt.Sprintf("recommendation kind: %s", recommendation.Kind), fmt.Sprintf("reason codes: %s", strings.Join(recommendation.Explanation.ReasonCodes, ", "))},
			Limitations: append([]string(nil), recommendation.Explanation.Limitations...),
		}, nil
	case WhyTopicGoal:
		return helper.Summary{
			Headline: "Why this goal setup",
			Detail:   "The deterministic coaching core uses the stored goal, days-per-week target, session length, and experience tier to bound the previewed changes.",
			Bullets: []string{
				fmt.Sprintf("goal key: %s", recommendation.TrainingGoal.GoalKey),
				fmt.Sprintf("target days per week: %d", recommendation.TrainingGoal.TargetDaysPerWeek),
				fmt.Sprintf("target session minutes: %d", recommendation.TrainingGoal.TargetSessionMinutes),
			},
			Limitations: append([]string(nil), recommendation.Explanation.Limitations...),
		}, nil
	case WhyTopicProposal:
		return helper.Summary{
			Headline:    "Why these plan changes",
			Detail:      proposalDetail(recommendation),
			Bullets:     proposalBullets(recommendation),
			Limitations: append([]string(nil), recommendation.Explanation.Limitations...),
		}, nil
	case WhyTopicFeedback:
		return helper.Summary{
			Headline:    "How feedback affected the preview",
			Detail:      feedbackDetail(recommendation),
			Bullets:     feedbackBullets(recommendation),
			Limitations: append([]string(nil), recommendation.Explanation.Limitations...),
		}, nil
	default:
		return helper.Summary{}, helper.ErrUnsupportedWhyTopic
	}
}

func buildVariationSummary(base CoachingRecommendation, variant CoachingRecommendation, variation string) helper.Summary {
	return helper.Summary{
		Headline: fmt.Sprintf("Preview a %s coaching variation", normalizeHelperKey(variation)),
		Detail: fmt.Sprintf(
			"This read-only helper preview shifts the deterministic coaching output from %s to %s without mutating planner truth.",
			base.Kind,
			variant.Kind,
		),
		Bullets: []string{
			fmt.Sprintf("base recommendation: %s", base.Kind),
			fmt.Sprintf("preview recommendation: %s", variant.Kind),
			fmt.Sprintf("preview plan changes: %d item(s)", len(variant.Proposal.Changes)),
		},
		Limitations: append([]string(nil), variant.Explanation.Limitations...),
	}
}

func headlineForKind(kind RecommendationKind) string {
	switch kind {
	case KindProgress:
		return "Progress the next planned week slightly"
	case KindDeload:
		return "Deload the next planned week"
	case KindRegress:
		return "Regress the next planned week conservatively"
	case KindStartConservative:
		return "Start the planned week conservatively"
	default:
		return "Hold the current planned week steady"
	}
}

func detailForSummary(recommendation CoachingRecommendation) string {
	switch {
	case containsCode(recommendation.Explanation.ReasonCodes, "no_planner_items"):
		return "There is no existing planner truth to adjust, so the helper can only explain the current hold without inventing a plan diff."
	case recommendation.SourceWorkoutID == nil:
		return "The deterministic coaching core used planner truth and stored profile inputs without a finished-workout feedback anchor."
	case recommendation.Explanation.Evidence.EffortLevel == nil || recommendation.Explanation.Evidence.RecoveryLevel == nil:
		return "The deterministic coaching core found a finished workout but stayed conservative because the latest feedback was incomplete."
	default:
		return "The deterministic coaching core used the latest finished-workout feedback plus stored planner and profile truth to select this bounded preview."
	}
}

func proposalDetail(recommendation CoachingRecommendation) string {
	if len(recommendation.Proposal.Changes) == 0 {
		return "The helper preview contains no plan changes because the current deterministic coaching output stayed at a no-change hold."
	}
	return "The helper preview only adjusts existing planner items. It does not create sessions, swap exercises, or write planner state."
}

func proposalBullets(recommendation CoachingRecommendation) []string {
	if len(recommendation.Proposal.Changes) == 0 {
		return []string{"planner preview changes: none"}
	}

	limit := 3
	if len(recommendation.Proposal.Changes) < limit {
		limit = len(recommendation.Proposal.Changes)
	}
	bullets := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		change := recommendation.Proposal.Changes[i]
		bullets = append(bullets, fmt.Sprintf(
			"day %d session %d item %d: %s -> %s",
			change.DayIndex,
			change.SessionPosition,
			change.ItemPosition,
			describePlanItem(change.Before),
			describePlanItem(change.After),
		))
	}
	return bullets
}

func feedbackDetail(recommendation CoachingRecommendation) string {
	if recommendation.SourceWorkoutID == nil {
		return "No finished workout was available, so the helper could not anchor the preview to recent effort or recovery feedback."
	}
	if recommendation.Explanation.Evidence.EffortLevel == nil || recommendation.Explanation.Evidence.RecoveryLevel == nil {
		return "A finished workout exists, but the helper stayed conservative because either effort or recovery feedback is still missing."
	}
	return "The helper used the most recent finished-workout effort and recovery feedback exactly as stored to explain the deterministic recommendation kind."
}

func feedbackBullets(recommendation CoachingRecommendation) []string {
	if recommendation.SourceWorkoutID == nil {
		return []string{"source workout: unavailable"}
	}

	effort := "missing"
	if recommendation.Explanation.Evidence.EffortLevel != nil {
		effort = *recommendation.Explanation.Evidence.EffortLevel
	}
	recovery := "missing"
	if recommendation.Explanation.Evidence.RecoveryLevel != nil {
		recovery = *recommendation.Explanation.Evidence.RecoveryLevel
	}

	return []string{
		fmt.Sprintf("source workout: %s", recommendation.SourceWorkoutID.String()),
		fmt.Sprintf("effort level: %s", effort),
		fmt.Sprintf("recovery level: %s", recovery),
	}
}

func describePlanItem(values PlanItemValues) string {
	if values.WeightKg != nil {
		return fmt.Sprintf("%d x %d @ %.1fkg", values.Sets, values.Reps, *values.WeightKg)
	}
	if values.RPE != nil {
		return fmt.Sprintf("%d x %d @ RPE %.1f", values.Sets, values.Reps, *values.RPE)
	}
	return fmt.Sprintf("%d x %d", values.Sets, values.Reps)
}

func variationKindFor(base RecommendationKind, variation string) (RecommendationKind, error) {
	switch normalizeHelperKey(variation) {
	case VariationEasier:
		switch base {
		case KindProgress:
			return KindHold, nil
		case KindHold:
			return KindStartConservative, nil
		case KindStartConservative:
			return KindDeload, nil
		case KindDeload:
			return KindRegress, nil
		default:
			return KindRegress, nil
		}
	case VariationHarder:
		switch base {
		case KindRegress:
			return KindDeload, nil
		case KindDeload:
			return KindStartConservative, nil
		case KindStartConservative:
			return KindHold, nil
		case KindHold:
			return KindProgress, nil
		default:
			return KindProgress, nil
		}
	default:
		return KindHold, helper.ErrUnsupportedVariation
	}
}

func normalizeHelperKey(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func containsCode(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
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
