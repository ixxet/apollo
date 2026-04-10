package coaching

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/helper"
	"github.com/ixxet/apollo/internal/planner"
	"github.com/ixxet/apollo/internal/profile"
)

type stubStore struct {
	workoutByID           *WorkoutSnapshot
	latestFinishedWorkout *WorkoutSnapshot
	effortFeedback        *EffortFeedbackRecord
	recoveryFeedback      *RecoveryFeedbackRecord
	latestEffortLevel     string
	latestRecoveryLevel   string
	err                   error
}

func (s *stubStore) GetWorkoutByIDForUser(context.Context, uuid.UUID, uuid.UUID) (*WorkoutSnapshot, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.workoutByID, nil
}

func (s *stubStore) GetLatestFinishedWorkoutByUserID(context.Context, uuid.UUID) (*WorkoutSnapshot, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.latestFinishedWorkout, nil
}

func (s *stubStore) UpsertEffortFeedbackForFinishedWorkout(_ context.Context, workoutID uuid.UUID, _ uuid.UUID, effortLevel string) (*EffortFeedbackRecord, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.latestEffortLevel = effortLevel
	return &EffortFeedbackRecord{
		WorkoutID:   workoutID,
		EffortLevel: effortLevel,
		CreatedAt:   time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
	}, nil
}

func (s *stubStore) UpsertRecoveryFeedbackForFinishedWorkout(_ context.Context, workoutID uuid.UUID, _ uuid.UUID, recoveryLevel string) (*RecoveryFeedbackRecord, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.latestRecoveryLevel = recoveryLevel
	return &RecoveryFeedbackRecord{
		WorkoutID:     workoutID,
		RecoveryLevel: recoveryLevel,
		CreatedAt:     time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
	}, nil
}

func (s *stubStore) GetEffortFeedbackByWorkoutID(context.Context, uuid.UUID) (*EffortFeedbackRecord, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.effortFeedback, nil
}

func (s *stubStore) GetRecoveryFeedbackByWorkoutID(context.Context, uuid.UUID) (*RecoveryFeedbackRecord, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.recoveryFeedback, nil
}

type stubPlannerReader struct {
	week planner.Week
	err  error
}

func (s stubPlannerReader) GetWeek(context.Context, uuid.UUID, string) (planner.Week, error) {
	if s.err != nil {
		return planner.Week{}, s.err
	}
	return s.week, nil
}

type stubProfileReader struct {
	profile profile.MemberProfile
	err     error
}

func (s stubProfileReader) GetProfile(context.Context, uuid.UUID) (profile.MemberProfile, error) {
	if s.err != nil {
		return profile.MemberProfile{}, s.err
	}
	return s.profile, nil
}

func TestPutFeedbackEnforcesEnumAndFinishedWorkoutOwnership(t *testing.T) {
	service := NewService(&stubStore{}, stubPlannerReader{}, stubProfileReader{})
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	workoutID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	if _, err := service.PutEffortFeedback(context.Background(), userID, workoutID, EffortFeedbackInput{EffortLevel: "elite"}); !errors.Is(err, ErrInvalidEffortLevel) {
		t.Fatalf("PutEffortFeedback() error = %v, want %v", err, ErrInvalidEffortLevel)
	}
	if _, err := service.PutRecoveryFeedback(context.Background(), userID, workoutID, RecoveryFeedbackInput{RecoveryLevel: "fresh"}); !errors.Is(err, ErrInvalidRecoveryLevel) {
		t.Fatalf("PutRecoveryFeedback() error = %v, want %v", err, ErrInvalidRecoveryLevel)
	}

	notFoundStore := &stubStore{}
	service = NewService(notFoundStore, stubPlannerReader{}, stubProfileReader{})
	if _, err := service.PutEffortFeedback(context.Background(), userID, workoutID, EffortFeedbackInput{EffortLevel: EffortLevelEasy}); !errors.Is(err, ErrWorkoutNotFound) {
		t.Fatalf("PutEffortFeedback() error = %v, want %v", err, ErrWorkoutNotFound)
	}

	inProgressStore := &stubStore{workoutByID: &WorkoutSnapshot{ID: workoutID, Status: "in_progress"}}
	service = NewService(inProgressStore, stubPlannerReader{}, stubProfileReader{})
	if _, err := service.PutRecoveryFeedback(context.Background(), userID, workoutID, RecoveryFeedbackInput{RecoveryLevel: RecoveryLevelRecovered}); !errors.Is(err, ErrWorkoutNotFinished) {
		t.Fatalf("PutRecoveryFeedback() error = %v, want %v", err, ErrWorkoutNotFinished)
	}

	finishedStore := &stubStore{workoutByID: &WorkoutSnapshot{ID: workoutID, Status: "finished"}}
	service = NewService(finishedStore, stubPlannerReader{}, stubProfileReader{})
	feedback, err := service.PutEffortFeedback(context.Background(), userID, workoutID, EffortFeedbackInput{EffortLevel: " EASY "})
	if err != nil {
		t.Fatalf("PutEffortFeedback() error = %v", err)
	}
	if feedback.EffortLevel != EffortLevelEasy {
		t.Fatalf("feedback.EffortLevel = %q, want %q", feedback.EffortLevel, EffortLevelEasy)
	}
}

func TestGetCoachingRecommendationUsesDeterministicLadderAndProposalRules(t *testing.T) {
	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	workoutID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	weight := 100.0
	week := planner.Week{
		WeekStart: "2026-04-06",
		Sessions: []planner.WeekSession{
			{
				DayIndex: 0,
				Position: 1,
				Items: []planner.WeekSessionItem{
					{Position: 1, Sets: 5, Reps: 5, WeightKg: &weight},
					{Position: 2, Sets: 3, Reps: 12},
				},
			},
		},
	}

	service := NewService(
		&stubStore{
			latestFinishedWorkout: nil,
		},
		stubPlannerReader{week: week},
		stubProfileReader{profile: profile.MemberProfile{
			CoachingProfile: profile.CoachingProfile{
				SessionMinutes: intPtr(300),
			},
		}},
	)
	service.now = func() time.Time { return now }

	recommendation, err := service.GetCoachingRecommendation(context.Background(), userID, "2026-04-06")
	if err != nil {
		t.Fatalf("GetCoachingRecommendation() error = %v", err)
	}
	if recommendation.Kind != KindStartConservative {
		t.Fatalf("recommendation.Kind = %q, want %q", recommendation.Kind, KindStartConservative)
	}
	if recommendation.TrainingGoal.TargetSessionMinutes != 120 {
		t.Fatalf("target session minutes = %d, want 120", recommendation.TrainingGoal.TargetSessionMinutes)
	}
	if len(recommendation.Proposal.Changes) != 2 {
		t.Fatalf("len(changes) = %d, want 2", len(recommendation.Proposal.Changes))
	}
	if recommendation.Proposal.Changes[0].After.WeightKg == nil || *recommendation.Proposal.Changes[0].After.WeightKg != 85 {
		t.Fatalf("first change weight = %#v, want 85", recommendation.Proposal.Changes[0].After.WeightKg)
	}
	if recommendation.Proposal.Changes[1].After.RPE == nil || *recommendation.Proposal.Changes[1].After.RPE != 6.0 {
		t.Fatalf("second change rpe = %#v, want 6.0", recommendation.Proposal.Changes[1].After.RPE)
	}

	progressStore := &stubStore{
		latestFinishedWorkout: &WorkoutSnapshot{ID: workoutID, Status: "finished"},
		effortFeedback:        &EffortFeedbackRecord{WorkoutID: workoutID, EffortLevel: EffortLevelEasy},
		recoveryFeedback:      &RecoveryFeedbackRecord{WorkoutID: workoutID, RecoveryLevel: RecoveryLevelRecovered},
	}
	progressService := NewService(progressStore, stubPlannerReader{week: week}, stubProfileReader{profile: profile.MemberProfile{
		CoachingProfile: profile.CoachingProfile{
			GoalKey:         stringPtr("build-strength"),
			DaysPerWeek:     intPtr(4),
			SessionMinutes:  intPtr(60),
			ExperienceLevel: stringPtr(ExperienceLevelIntermediate),
		},
	}})
	progressService.now = func() time.Time { return now }

	progressRecommendation, err := progressService.GetCoachingRecommendation(context.Background(), userID, "2026-04-06")
	if err != nil {
		t.Fatalf("GetCoachingRecommendation(progress) error = %v", err)
	}
	if progressRecommendation.Kind != KindProgress {
		t.Fatalf("progress kind = %q, want %q", progressRecommendation.Kind, KindProgress)
	}
	if progressRecommendation.SourceWorkoutID == nil || *progressRecommendation.SourceWorkoutID != workoutID {
		t.Fatalf("source workout id = %#v, want %s", progressRecommendation.SourceWorkoutID, workoutID)
	}
	if progressRecommendation.Proposal.Changes[0].After.WeightKg == nil || *progressRecommendation.Proposal.Changes[0].After.WeightKg != 102.5 {
		t.Fatalf("progress weight = %#v, want 102.5", progressRecommendation.Proposal.Changes[0].After.WeightKg)
	}
	if progressRecommendation.Proposal.Changes[1].After.RPE == nil || *progressRecommendation.Proposal.Changes[1].After.RPE != 7.0 {
		t.Fatalf("progress rpe = %#v, want 7.0", progressRecommendation.Proposal.Changes[1].After.RPE)
	}
}

func TestGetCoachingRecommendationReturnsHoldWithEmptyProposalForEmptyWeek(t *testing.T) {
	service := NewService(
		&stubStore{},
		stubPlannerReader{week: planner.Week{WeekStart: "2026-04-06", Sessions: []planner.WeekSession{}}},
		stubProfileReader{profile: profile.MemberProfile{}},
	)
	service.now = func() time.Time {
		return time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	}

	recommendation, err := service.GetCoachingRecommendation(context.Background(), uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), "2026-04-06")
	if err != nil {
		t.Fatalf("GetCoachingRecommendation() error = %v", err)
	}
	if recommendation.Kind != KindHold {
		t.Fatalf("recommendation.Kind = %q, want %q", recommendation.Kind, KindHold)
	}
	if len(recommendation.Proposal.Changes) != 0 {
		t.Fatalf("len(recommendation.Proposal.Changes) = %d, want 0", len(recommendation.Proposal.Changes))
	}
	if !contains(recommendation.Explanation.ReasonCodes, "no_planner_items") {
		t.Fatalf("reason codes = %#v, want no_planner_items", recommendation.Explanation.ReasonCodes)
	}
}

func TestGetCoachingRecommendationIsDeterministicAcrossRepeatedReruns(t *testing.T) {
	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	workoutID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	rpe := 7.0
	service := NewService(
		&stubStore{
			latestFinishedWorkout: &WorkoutSnapshot{ID: workoutID, Status: "finished"},
			effortFeedback:        &EffortFeedbackRecord{WorkoutID: workoutID, EffortLevel: EffortLevelHard},
			recoveryFeedback:      &RecoveryFeedbackRecord{WorkoutID: workoutID, RecoveryLevel: RecoveryLevelSlightlyFatigued},
		},
		stubPlannerReader{week: planner.Week{
			WeekStart: "2026-04-06",
			Sessions: []planner.WeekSession{
				{DayIndex: 1, Position: 1, Items: []planner.WeekSessionItem{{Position: 1, Sets: 3, Reps: 10, RPE: &rpe}}},
			},
		}},
		stubProfileReader{profile: profile.MemberProfile{CoachingProfile: profile.CoachingProfile{
			GoalKey:         stringPtr("build-strength"),
			DaysPerWeek:     intPtr(4),
			SessionMinutes:  intPtr(60),
			ExperienceLevel: stringPtr(ExperienceLevelAdvanced),
		}}},
	)
	service.now = func() time.Time { return now }

	first, err := service.GetCoachingRecommendation(context.Background(), userID, "2026-04-06")
	if err != nil {
		t.Fatalf("first GetCoachingRecommendation() error = %v", err)
	}
	second, err := service.GetCoachingRecommendation(context.Background(), userID, "2026-04-06")
	if err != nil {
		t.Fatalf("second GetCoachingRecommendation() error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("repeated recommendations differ:\nfirst=%#v\nsecond=%#v", first, second)
	}
}

func TestHelperReadAndWhyStayBoundedToDeterministicRecommendation(t *testing.T) {
	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	workoutID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	weight := 100.0
	service := NewService(
		&stubStore{
			latestFinishedWorkout: &WorkoutSnapshot{ID: workoutID, Status: "finished"},
			effortFeedback:        &EffortFeedbackRecord{WorkoutID: workoutID, EffortLevel: EffortLevelEasy},
			recoveryFeedback:      &RecoveryFeedbackRecord{WorkoutID: workoutID, RecoveryLevel: RecoveryLevelRecovered},
		},
		stubPlannerReader{week: planner.Week{
			WeekStart: "2026-04-06",
			Sessions: []planner.WeekSession{
				{DayIndex: 0, Position: 1, Items: []planner.WeekSessionItem{{Position: 1, Sets: 5, Reps: 5, WeightKg: &weight}}},
			},
		}},
		stubProfileReader{profile: profile.MemberProfile{CoachingProfile: profile.CoachingProfile{
			GoalKey:         stringPtr("build-strength"),
			DaysPerWeek:     intPtr(4),
			SessionMinutes:  intPtr(60),
			ExperienceLevel: stringPtr(ExperienceLevelIntermediate),
		}}},
	)
	service.now = func() time.Time { return now }

	read, err := service.GetHelperRead(context.Background(), userID, "2026-04-06")
	if err != nil {
		t.Fatalf("GetHelperRead() error = %v", err)
	}
	if read.PreviewMode != "read_only" {
		t.Fatalf("PreviewMode = %q, want read_only", read.PreviewMode)
	}
	if read.Recommendation.Kind != KindProgress {
		t.Fatalf("Recommendation.Kind = %q, want %q", read.Recommendation.Kind, KindProgress)
	}
	if len(read.WhyOptions) != 4 {
		t.Fatalf("len(WhyOptions) = %d, want 4", len(read.WhyOptions))
	}
	if len(read.VariationOptions) != 2 {
		t.Fatalf("len(VariationOptions) = %d, want 2", len(read.VariationOptions))
	}
	if read.Summary.Headline == "" || read.Summary.Detail == "" {
		t.Fatalf("summary = %#v, want non-empty headline/detail", read.Summary)
	}

	why, err := service.AskWhy(context.Background(), userID, "2026-04-06", WhyTopicProposal)
	if err != nil {
		t.Fatalf("AskWhy() error = %v", err)
	}
	if why.Topic != WhyTopicProposal {
		t.Fatalf("Topic = %q, want %q", why.Topic, WhyTopicProposal)
	}
	if len(why.Summary.Bullets) == 0 {
		t.Fatalf("len(Summary.Bullets) = %d, want > 0", len(why.Summary.Bullets))
	}
	if why.Recommendation.Kind != read.Recommendation.Kind {
		t.Fatalf("Recommendation.Kind = %q, want %q", why.Recommendation.Kind, read.Recommendation.Kind)
	}
}

func TestPreviewVariationReturnsReadOnlyAdjacentRecommendation(t *testing.T) {
	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	workoutID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	weight := 100.0
	service := NewService(
		&stubStore{
			latestFinishedWorkout: &WorkoutSnapshot{ID: workoutID, Status: "finished"},
			effortFeedback:        &EffortFeedbackRecord{WorkoutID: workoutID, EffortLevel: EffortLevelEasy},
			recoveryFeedback:      &RecoveryFeedbackRecord{WorkoutID: workoutID, RecoveryLevel: RecoveryLevelRecovered},
		},
		stubPlannerReader{week: planner.Week{
			WeekStart: "2026-04-06",
			Sessions: []planner.WeekSession{
				{DayIndex: 0, Position: 1, Items: []planner.WeekSessionItem{{Position: 1, Sets: 5, Reps: 5, WeightKg: &weight}}},
			},
		}},
		stubProfileReader{profile: profile.MemberProfile{CoachingProfile: profile.CoachingProfile{
			GoalKey:         stringPtr("build-strength"),
			DaysPerWeek:     intPtr(4),
			SessionMinutes:  intPtr(60),
			ExperienceLevel: stringPtr(ExperienceLevelIntermediate),
		}}},
	)
	service.now = func() time.Time { return now }

	preview, err := service.PreviewVariation(context.Background(), userID, "2026-04-06", VariationEasier)
	if err != nil {
		t.Fatalf("PreviewVariation() error = %v", err)
	}
	if preview.PreviewMode != "read_only" {
		t.Fatalf("PreviewMode = %q, want read_only", preview.PreviewMode)
	}
	if preview.BaseKind != KindProgress {
		t.Fatalf("BaseKind = %q, want %q", preview.BaseKind, KindProgress)
	}
	if preview.Recommendation.Kind != KindHold {
		t.Fatalf("Recommendation.Kind = %q, want %q", preview.Recommendation.Kind, KindHold)
	}
	if !contains(preview.Recommendation.Explanation.ReasonCodes, "helper_variation_preview") {
		t.Fatalf("ReasonCodes = %#v, want helper_variation_preview", preview.Recommendation.Explanation.ReasonCodes)
	}
	if len(preview.Recommendation.Proposal.Changes) != 0 {
		t.Fatalf("len(Preview changes) = %d, want 0 for hold preview", len(preview.Recommendation.Proposal.Changes))
	}
	if preview.Summary.Detail == "" {
		t.Fatalf("Summary.Detail = empty, want non-empty detail")
	}
}

func TestHelperActionsRejectUnsupportedTopicsAndVariations(t *testing.T) {
	service := NewService(&stubStore{}, stubPlannerReader{}, stubProfileReader{})
	userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")

	if _, err := service.AskWhy(context.Background(), userID, "2026-04-06", "timeline"); err != helper.ErrUnsupportedWhyTopic {
		t.Fatalf("AskWhy() error = %v, want %v", err, helper.ErrUnsupportedWhyTopic)
	}
	if _, err := service.PreviewVariation(context.Background(), userID, "2026-04-06", "longer"); err != helper.ErrUnsupportedVariation {
		t.Fatalf("PreviewVariation() error = %v, want %v", err, helper.ErrUnsupportedVariation)
	}
}

func contains(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}

func intPtr(value int) *int {
	return &value
}
