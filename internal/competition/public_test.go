package competition

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/rating"
)

func TestPublicCompetitionReadinessUsesExplicitPublicContract(t *testing.T) {
	svc := NewService(stubStore{
		publicReadiness: func(context.Context) (publicCompetitionReadinessRecord, error) {
			return publicCompetitionReadinessRecord{
				AvailableLeaderboards:     3,
				AvailableCanonicalResults: 2,
			}, nil
		},
	})

	readiness, err := svc.PublicCompetitionReadiness(context.Background())
	if err != nil {
		t.Fatalf("PublicCompetitionReadiness() error = %v", err)
	}
	if readiness.ContractVersion != publicCompetitionContractVersion {
		t.Fatalf("ContractVersion = %q, want %q", readiness.ContractVersion, publicCompetitionContractVersion)
	}
	if readiness.ProjectionVersion != competitionAnalyticsProjectionVersion {
		t.Fatalf("ProjectionVersion = %q, want %q", readiness.ProjectionVersion, competitionAnalyticsProjectionVersion)
	}
	if readiness.ResultSource != publicCompetitionResultSource {
		t.Fatalf("ResultSource = %q, want %q", readiness.ResultSource, publicCompetitionResultSource)
	}
	if readiness.RatingSource != publicCompetitionRatingSource {
		t.Fatalf("RatingSource = %q, want %q", readiness.RatingSource, publicCompetitionRatingSource)
	}
	if readiness.Status != publicCompetitionStatusAvailable {
		t.Fatalf("Status = %q, want %q", readiness.Status, publicCompetitionStatusAvailable)
	}
	if slices.Contains(readiness.Deferred, "cp") || !slices.Contains(readiness.Deferred, "rating_read_path_switch") {
		t.Fatalf("Deferred = %#v, want public competition deferments", readiness.Deferred)
	}
}

func TestListPublicCompetitionLeaderboardNormalizesAndRedactsParticipants(t *testing.T) {
	computedAt := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	var captured PublicCompetitionLeaderboardInput
	svc := NewService(stubStore{
		publicLeaderboard: func(_ context.Context, input PublicCompetitionLeaderboardInput) ([]publicCompetitionLeaderboardRowRecord, error) {
			captured = input
			return []publicCompetitionLeaderboardRowRecord{{
				SportKey:    "badminton",
				ModeKey:     "head_to_head:s2-p1",
				FacilityKey: "gym-floor",
				TeamScope:   analyticsDimensionAll,
				StatType:    analyticsStatWins,
				StatValue:   4,
				SampleSize:  4,
				Confidence:  1,
				ComputedAt:  computedAt,
			}}, nil
		},
	})

	leaderboard, err := svc.ListPublicCompetitionLeaderboard(context.Background(), PublicCompetitionLeaderboardInput{
		StatType:  "openskill_delta",
		TeamScope: "private_scope",
		Limit:     1000,
	})
	if err != nil {
		t.Fatalf("ListPublicCompetitionLeaderboard() error = %v", err)
	}
	if captured.StatType != analyticsStatWins {
		t.Fatalf("captured.StatType = %q, want %q", captured.StatType, analyticsStatWins)
	}
	if captured.TeamScope != analyticsDimensionAll {
		t.Fatalf("captured.TeamScope = %q, want %q", captured.TeamScope, analyticsDimensionAll)
	}
	if captured.Limit != maxPublicLeaderboardLimit {
		t.Fatalf("captured.Limit = %d, want %d", captured.Limit, maxPublicLeaderboardLimit)
	}
	if got, want := len(leaderboard.Leaderboard), 1; got != want {
		t.Fatalf("len(leaderboard.Leaderboard) = %d, want %d", got, want)
	}
	row := leaderboard.Leaderboard[0]
	if row.Participant != "participant_1" {
		t.Fatalf("Participant = %q, want redacted participant label", row.Participant)
	}
	if row.StatType != analyticsStatWins || row.StatValue != 4 {
		t.Fatalf("row = %+v, want wins projection", row)
	}
}

func TestPublicAndMemberCompetitionReadsDoNotExposeOpenSkillComparison(t *testing.T) {
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	recordedAt := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	svc := NewService(stubStore{
		publicReadiness: func(context.Context) (publicCompetitionReadinessRecord, error) {
			return publicCompetitionReadinessRecord{
				AvailableLeaderboards:     1,
				AvailableCanonicalResults: 1,
			}, nil
		},
		publicLeaderboard: func(_ context.Context, input PublicCompetitionLeaderboardInput) ([]publicCompetitionLeaderboardRowRecord, error) {
			if input.StatType != analyticsStatWins {
				t.Fatalf("input.StatType = %q, want public-safe normalization to wins", input.StatType)
			}
			return []publicCompetitionLeaderboardRowRecord{{
				SportKey:   "badminton",
				ModeKey:    "head_to_head:s2-p1",
				TeamScope:  analyticsDimensionAll,
				StatType:   analyticsStatWins,
				StatValue:  1,
				ComputedAt: recordedAt,
			}}, nil
		},
		listMemberStatRowsByUser: func(context.Context, uuid.UUID) ([]memberStatRowRecord, error) {
			return []memberStatRowRecord{{
				SportKey:            "badminton",
				CompetitionMode:     "head_to_head",
				SidesPerMatch:       2,
				ParticipantsPerSide: 1,
				RecordedAt:          recordedAt,
				Outcome:             matchOutcomeWin,
			}}, nil
		},
		listMemberRatingsByUser: func(context.Context, uuid.UUID) ([]memberRatingRecord, error) {
			return []memberRatingRecord{{
				UserID:            userID,
				SportKey:          "badminton",
				ModeKey:           "head_to_head:s2-p1",
				Mu:                26.5,
				Sigma:             8,
				MatchesPlayed:     1,
				RatingEngine:      rating.EngineLegacyEloLike,
				EngineVersion:     rating.EngineVersionLegacy,
				PolicyVersion:     rating.PolicyVersionActive,
				CalibrationStatus: rating.CalibrationStatusProvisional,
			}}, nil
		},
	})

	readiness, err := svc.PublicCompetitionReadiness(context.Background())
	if err != nil {
		t.Fatalf("PublicCompetitionReadiness() error = %v", err)
	}
	leaderboard, err := svc.ListPublicCompetitionLeaderboard(context.Background(), PublicCompetitionLeaderboardInput{StatType: "openskill_delta"})
	if err != nil {
		t.Fatalf("ListPublicCompetitionLeaderboard() error = %v", err)
	}
	stats, err := svc.ListMemberStats(context.Background(), userID)
	if err != nil {
		t.Fatalf("ListMemberStats() error = %v", err)
	}
	if len(stats) != 1 || stats[0].RatingEngine != rating.EngineLegacyEloLike || stats[0].RatingPolicyVersion != rating.PolicyVersionActive {
		t.Fatalf("member stats = %+v, want active legacy-engine policy metadata", stats)
	}

	raw, err := json.Marshal(struct {
		Readiness   PublicCompetitionReadiness   `json:"readiness"`
		Leaderboard PublicCompetitionLeaderboard `json:"leaderboard"`
		MemberStats []MemberStat                 `json:"member_stats"`
	}{readiness, leaderboard, stats})
	if err != nil {
		t.Fatalf("json.Marshal(reads) error = %v", err)
	}
	body := strings.ToLower(string(raw))
	for _, forbidden := range []string{
		"openskill",
		"comparison",
		"delta_from_legacy",
		"accepted_delta_budget",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("public/member reads leaked %q: %s", forbidden, body)
		}
	}
}

func TestPublicGameIdentityProjectionIsDeterministicAndPolicyVersioned(t *testing.T) {
	firstUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	secondUserID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	computedAt := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	lastResultAt := time.Date(2026, 4, 29, 11, 0, 0, 0, time.UTC)
	var captured GameIdentityProjectionInput
	svc := NewService(stubStore{
		gameIdentityRows: func(_ context.Context, input GameIdentityProjectionInput) ([]gameIdentityProjectionRowRecord, error) {
			captured = input
			return []gameIdentityProjectionRowRecord{
				{
					UserID:        secondUserID,
					SportKey:      "badminton",
					ModeKey:       "head_to_head:s2-p1",
					FacilityKey:   "gym-floor",
					TeamScope:     analyticsDimensionAll,
					MatchesPlayed: 2,
					Wins:          2,
					ComputedAt:    computedAt,
				},
				{
					UserID:        firstUserID,
					SportKey:      "badminton",
					ModeKey:       "head_to_head:s2-p1",
					FacilityKey:   "gym-floor",
					TeamScope:     analyticsDimensionAll,
					MatchesPlayed: 5,
					Wins:          2,
					Losses:        3,
					LastResultAt:  &lastResultAt,
					ComputedAt:    computedAt.Add(time.Minute),
				},
			}, nil
		},
	})

	projection, err := svc.PublicGameIdentity(context.Background(), PublicGameIdentityInput{
		TeamScope: "private_scope",
		Limit:     1000,
	})
	if err != nil {
		t.Fatalf("PublicGameIdentity() error = %v", err)
	}
	if captured.TeamScope != analyticsDimensionAll || captured.Limit != maxGameIdentityLimit {
		t.Fatalf("captured input = %+v, want normalized public-safe scope and limit", captured)
	}
	if projection.ContractVersion != gameIdentityContractVersion || projection.ProjectionVersion != gameIdentityProjectionVersion {
		t.Fatalf("projection versions = %q/%q, want game identity versions", projection.ContractVersion, projection.ProjectionVersion)
	}
	if projection.CPPolicyVersion != gameIdentityCPPolicyVersion || projection.BadgePolicyVersion != gameIdentityBadgePolicyVersion || projection.RivalryPolicyVersion != gameIdentityRivalryPolicyVersion || projection.SquadPolicyVersion != gameIdentitySquadPolicyVersion {
		t.Fatalf("policy versions missing from projection: %+v", projection)
	}
	if got, want := projection.CP[0].Participant, "participant_1"; got != want {
		t.Fatalf("top participant = %q, want %q", got, want)
	}
	if got, want := projection.CP[0].CP, 125; got != want {
		t.Fatalf("top CP = %d, want %d", got, want)
	}
	if got, want := len(projection.BadgeAwards), 5; got != want {
		t.Fatalf("len(BadgeAwards) = %d, want %d", got, want)
	}
	if got, want := projection.RivalryStates[0].State, "active"; got != want {
		t.Fatalf("rivalry state = %q, want %q", got, want)
	}
	if got, want := projection.RivalryStates[0].CPGap, 45; got != want {
		t.Fatalf("rivalry CP gap = %d, want %d", got, want)
	}
	if got, want := projection.SquadIdentities[0].CPTotal, 205; got != want {
		t.Fatalf("squad CP total = %d, want %d", got, want)
	}

	raw, err := json.Marshal(projection)
	if err != nil {
		t.Fatalf("json.Marshal(projection) error = %v", err)
	}
	body := strings.ToLower(string(raw))
	for _, forbidden := range []string{
		firstUserID.String(),
		secondUserID.String(),
		"user_id",
		"source_result_id",
		"canonical_result_id",
		"openskill",
		"sample_size",
		"confidence",
		"projection_watermark",
		"safety",
		"trusted_surface",
		"command",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("game identity projection leaked %q: %s", forbidden, body)
		}
	}
}

func TestPublicGameIdentityRivalryStatesStayWithinProjectionContext(t *testing.T) {
	firstUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	secondUserID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	thirdUserID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	computedAt := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	svc := NewService(stubStore{
		gameIdentityRows: func(_ context.Context, _ GameIdentityProjectionInput) ([]gameIdentityProjectionRowRecord, error) {
			return []gameIdentityProjectionRowRecord{
				{
					UserID:        firstUserID,
					SportKey:      "badminton",
					ModeKey:       "head_to_head:s2-p1",
					FacilityKey:   "gym-floor",
					TeamScope:     analyticsDimensionAll,
					MatchesPlayed: 6,
					Wins:          5,
					ComputedAt:    computedAt,
				},
				{
					UserID:        secondUserID,
					SportKey:      "basketball",
					ModeKey:       "three_on_three",
					FacilityKey:   "court-a",
					TeamScope:     analyticsDimensionAll,
					MatchesPlayed: 5,
					Wins:          4,
					ComputedAt:    computedAt.Add(time.Minute),
				},
				{
					UserID:        thirdUserID,
					SportKey:      "badminton",
					ModeKey:       "head_to_head:s2-p1",
					FacilityKey:   "gym-floor",
					TeamScope:     analyticsDimensionAll,
					MatchesPlayed: 3,
					Wins:          2,
					ComputedAt:    computedAt.Add(2 * time.Minute),
				},
			}, nil
		},
	})

	projection, err := svc.PublicGameIdentity(context.Background(), PublicGameIdentityInput{})
	if err != nil {
		t.Fatalf("PublicGameIdentity() error = %v", err)
	}
	if got, want := len(projection.RivalryStates), 1; got != want {
		t.Fatalf("len(RivalryStates) = %d, want %d: %+v", got, want, projection.RivalryStates)
	}
	rivalry := projection.RivalryStates[0]
	if rivalry.SportKey != "badminton" || rivalry.ModeKey != "head_to_head:s2-p1" || rivalry.FacilityKey != "gym-floor" {
		t.Fatalf("rivalry context = %s/%s/%s, want badminton head_to_head:s2-p1 gym-floor", rivalry.SportKey, rivalry.ModeKey, rivalry.FacilityKey)
	}
	if got, want := rivalry.Participants, []string{"participant_1", "participant_3"}; !slices.Equal(got, want) {
		t.Fatalf("rivalry participants = %v, want %v", got, want)
	}
	if strings.Contains(strings.Join(rivalry.Participants, ","), "participant_2") {
		t.Fatalf("rivalry crossed projection context: %+v", rivalry)
	}
}

func TestPublicGameIdentityLabelsStayScopedToProjectionRow(t *testing.T) {
	firstUserID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	secondUserID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	computedAt := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	svc := NewService(stubStore{
		gameIdentityRows: func(_ context.Context, _ GameIdentityProjectionInput) ([]gameIdentityProjectionRowRecord, error) {
			return []gameIdentityProjectionRowRecord{
				{
					UserID:        firstUserID,
					SportKey:      "badminton",
					ModeKey:       "head_to_head:s2-p1",
					FacilityKey:   "gym-floor",
					TeamScope:     analyticsDimensionAll,
					MatchesPlayed: 6,
					Wins:          5,
					ComputedAt:    computedAt,
				},
				{
					UserID:        secondUserID,
					SportKey:      "badminton",
					ModeKey:       "head_to_head:s2-p1",
					FacilityKey:   "gym-floor",
					TeamScope:     analyticsDimensionAll,
					MatchesPlayed: 3,
					Wins:          2,
					ComputedAt:    computedAt.Add(time.Minute),
				},
				{
					UserID:        firstUserID,
					SportKey:      "basketball",
					ModeKey:       "three_on_three",
					FacilityKey:   "court-a",
					TeamScope:     analyticsDimensionAll,
					MatchesPlayed: 4,
					Wins:          4,
					ComputedAt:    computedAt.Add(2 * time.Minute),
				},
			}, nil
		},
	})

	projection, err := svc.PublicGameIdentity(context.Background(), PublicGameIdentityInput{})
	if err != nil {
		t.Fatalf("PublicGameIdentity() error = %v", err)
	}
	if got, want := []string{projection.CP[0].Participant, projection.CP[1].Participant, projection.CP[2].Participant}, []string{"participant_1", "participant_2", "participant_3"}; !slices.Equal(got, want) {
		t.Fatalf("CP participants = %v, want %v", got, want)
	}
	assertBadgeParticipant := func(sportKey string, want string) {
		t.Helper()
		for _, award := range projection.BadgeAwards {
			if award.SportKey == sportKey && award.BadgeKey == "first_win" {
				if award.Participant != want {
					t.Fatalf("%s first_win participant = %q, want %q", sportKey, award.Participant, want)
				}
				return
			}
		}
		t.Fatalf("missing %s first_win badge in %+v", sportKey, projection.BadgeAwards)
	}
	assertBadgeParticipant("badminton", "participant_1")
	assertBadgeParticipant("basketball", "participant_2")

	for _, rivalry := range projection.RivalryStates {
		if rivalry.SportKey == "badminton" {
			if got, want := rivalry.Participants, []string{"participant_1", "participant_3"}; !slices.Equal(got, want) {
				t.Fatalf("badminton rivalry participants = %v, want %v", got, want)
			}
			return
		}
	}
	t.Fatalf("missing badminton rivalry in %+v", projection.RivalryStates)
}

func TestMemberGameIdentityScopesToCaller(t *testing.T) {
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	computedAt := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	var captured GameIdentityProjectionInput
	svc := NewService(stubStore{
		gameIdentityRows: func(_ context.Context, input GameIdentityProjectionInput) ([]gameIdentityProjectionRowRecord, error) {
			captured = input
			return []gameIdentityProjectionRowRecord{{
				UserID:        userID,
				SportKey:      "badminton",
				ModeKey:       analyticsDimensionAll,
				FacilityKey:   analyticsDimensionAll,
				TeamScope:     analyticsDimensionAll,
				MatchesPlayed: 1,
				Wins:          1,
				ComputedAt:    computedAt,
			}}, nil
		},
	})

	projection, err := svc.MemberGameIdentity(context.Background(), userID, PublicGameIdentityInput{})
	if err != nil {
		t.Fatalf("MemberGameIdentity() error = %v", err)
	}
	if captured.UserID == nil || *captured.UserID != userID {
		t.Fatalf("captured.UserID = %v, want caller scope", captured.UserID)
	}
	if got, want := projection.CP[0].Participant, "member_self"; got != want {
		t.Fatalf("participant = %q, want %q", got, want)
	}
}
