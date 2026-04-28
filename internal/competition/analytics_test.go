package competition

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAnalyticsProjectionFactsAreDeterministic(t *testing.T) {
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	firstResultID := uuid.MustParse("00000000-0000-0000-0000-000000000101")
	secondResultID := uuid.MustParse("00000000-0000-0000-0000-000000000102")
	firstMatchID := uuid.MustParse("00000000-0000-0000-0000-000000000201")
	secondMatchID := uuid.MustParse("00000000-0000-0000-0000-000000000202")

	soloKey := analyticsProjectionKey{
		userID:      userID,
		sportKey:    "badminton",
		facilityKey: analyticsDimensionAll,
		modeKey:     analyticsDimensionAll,
		teamScope:   analyticsTeamScopeSolo,
	}
	teamKey := soloKey
	teamKey.teamScope = analyticsTeamScopeTeam

	first := map[analyticsProjectionKey]*analyticsAggregate{
		soloKey: {
			key:            soloKey,
			matchesPlayed:  2,
			wins:           1,
			losses:         1,
			currentStreak:  -1,
			sourceMatchID:  firstMatchID,
			sourceResultID: firstResultID,
			lastRecordedAt: time.Date(2026, 4, 28, 10, 0, 0, 0, time.UTC),
		},
		teamKey: {
			key:            teamKey,
			matchesPlayed:  1,
			wins:           1,
			currentStreak:  1,
			sourceMatchID:  secondMatchID,
			sourceResultID: secondResultID,
			lastRecordedAt: time.Date(2026, 4, 28, 11, 0, 0, 0, time.UTC),
		},
	}
	second := map[analyticsProjectionKey]*analyticsAggregate{
		teamKey: first[teamKey],
		soloKey: first[soloKey],
	}

	firstFacts := analyticsProjectionFacts(first)
	secondFacts := analyticsProjectionFacts(second)
	if !reflect.DeepEqual(firstFacts, secondFacts) {
		t.Fatalf("analyticsProjectionFacts order changed\nfirst=%+v\nsecond=%+v", firstFacts, secondFacts)
	}

	foundTeamDelta := false
	for _, fact := range firstFacts {
		if fact.statType == analyticsStatTeamVsSoloDelta {
			foundTeamDelta = true
			if fact.statValue != 0.5 || fact.sampleSize != 3 || fact.sourceResultID != secondResultID {
				t.Fatalf("team-vs-solo fact = %+v, want delta 0.5 over latest source result", fact)
			}
		}
	}
	if !foundTeamDelta {
		t.Fatal("team-vs-solo projection fact missing")
	}
}
