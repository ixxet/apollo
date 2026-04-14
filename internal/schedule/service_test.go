package schedule

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ixxet/apollo/internal/authz"
	"github.com/ixxet/apollo/internal/testutil"
)

func TestScheduleGraphConflictPropagationAndExclusiveNormalization(t *testing.T) {
	t.Run("parent blocks child and child blocks parent", func(t *testing.T) {
		service, actor, cleanup := newScheduleServiceFixture(t)
		defer cleanup()

		fullCourt := mustUpsertResource(t, service, actor, "full-court")
		halfCourt := mustUpsertResource(t, service, actor, "half-court-a")
		mustUpsertResource(t, service, actor, "half-court-b")

		if _, err := service.UpsertResourceEdge(context.Background(), actor, ResourceEdgeInput{
			ResourceKey:        fullCourt.ResourceKey,
			RelatedResourceKey: halfCourt.ResourceKey,
			EdgeType:           EdgeContains,
		}); err != nil {
			t.Fatalf("UpsertResourceEdge(full contains half) error = %v", err)
		}

		fullBlock := mustCreateOneOffBlock(t, service, actor, fullCourt.ResourceKey, "2026-04-06T10:00:00Z", "2026-04-06T11:00:00Z")
		if fullBlock.ID == uuid.Nil {
			t.Fatal("fullBlock.ID = nil")
		}
		_, err := service.CreateBlock(context.Background(), actor, BlockInput{
			FacilityKey: "ashtonbee",
			Scope:       ScopeResource,
			ResourceKey: &halfCourt.ResourceKey,
			Kind:        KindReservation,
			Effect:      EffectHardReserve,
			Visibility:  VisibilityInternal,
			OneOff: &OneOffInput{
				StartsAt: mustParseTime(t, "2026-04-06T10:00:00Z"),
				EndsAt:   mustParseTime(t, "2026-04-06T11:00:00Z"),
			},
		})
		if !errors.Is(err, ErrBlockConflictRejected) {
			t.Fatalf("CreateBlock(child against parent) error = %v, want %v", err, ErrBlockConflictRejected)
		}
	})

	t.Run("siblings remain independent until explicitly exclusive", func(t *testing.T) {
		service, actor, cleanup := newScheduleServiceFixture(t)
		defer cleanup()

		halfA := mustUpsertResource(t, service, actor, "half-court-a")
		halfB := mustUpsertResource(t, service, actor, "half-court-b")

		first, err := service.CreateBlock(context.Background(), actor, BlockInput{
			FacilityKey: "ashtonbee",
			Scope:       ScopeResource,
			ResourceKey: &halfA.ResourceKey,
			Kind:        KindReservation,
			Effect:      EffectHardReserve,
			Visibility:  VisibilityInternal,
			OneOff: &OneOffInput{
				StartsAt: mustParseTime(t, "2026-04-06T10:00:00Z"),
				EndsAt:   mustParseTime(t, "2026-04-06T11:00:00Z"),
			},
		})
		if err != nil {
			t.Fatalf("CreateBlock(half-a) error = %v", err)
		}

		second, err := service.CreateBlock(context.Background(), actor, BlockInput{
			FacilityKey: "ashtonbee",
			Scope:       ScopeResource,
			ResourceKey: &halfB.ResourceKey,
			Kind:        KindReservation,
			Effect:      EffectHardReserve,
			Visibility:  VisibilityInternal,
			OneOff: &OneOffInput{
				StartsAt: mustParseTime(t, "2026-04-06T10:00:00Z"),
				EndsAt:   mustParseTime(t, "2026-04-06T11:00:00Z"),
			},
		})
		if err != nil {
			t.Fatalf("CreateBlock(half-b) error = %v", err)
		}
		if len(first.Conflicts) != 0 || len(second.Conflicts) != 0 {
			t.Fatalf("expected sibling blocks to remain independent, got conflicts %#v %#v", first.Conflicts, second.Conflicts)
		}
	})

	t.Run("explicitly exclusive siblings conflict", func(t *testing.T) {
		service, actor, cleanup := newScheduleServiceFixture(t)
		defer cleanup()

		halfA := mustUpsertResource(t, service, actor, "half-court-a")
		halfB := mustUpsertResource(t, service, actor, "half-court-b")

		if _, err := service.CreateBlock(context.Background(), actor, BlockInput{
			FacilityKey: "ashtonbee",
			Scope:       ScopeResource,
			ResourceKey: &halfA.ResourceKey,
			Kind:        KindReservation,
			Effect:      EffectHardReserve,
			Visibility:  VisibilityInternal,
			OneOff: &OneOffInput{
				StartsAt: mustParseTime(t, "2026-04-06T10:00:00Z"),
				EndsAt:   mustParseTime(t, "2026-04-06T11:00:00Z"),
			},
		}); err != nil {
			t.Fatalf("CreateBlock(half-a) error = %v", err)
		}

		edge, err := service.UpsertResourceEdge(context.Background(), actor, ResourceEdgeInput{
			ResourceKey:        halfB.ResourceKey,
			RelatedResourceKey: halfA.ResourceKey,
			EdgeType:           EdgeExclusiveWith,
		})
		if err != nil {
			t.Fatalf("UpsertResourceEdge(exclusive) error = %v", err)
		}
		if edge.ResourceKey != halfA.ResourceKey || edge.RelatedResourceKey != halfB.ResourceKey {
			t.Fatalf("exclusive edge normalized to %s -> %s, want %s -> %s", edge.ResourceKey, edge.RelatedResourceKey, halfA.ResourceKey, halfB.ResourceKey)
		}

		_, err = service.CreateBlock(context.Background(), actor, BlockInput{
			FacilityKey: "ashtonbee",
			Scope:       ScopeResource,
			ResourceKey: &halfB.ResourceKey,
			Kind:        KindReservation,
			Effect:      EffectHardReserve,
			Visibility:  VisibilityInternal,
			OneOff: &OneOffInput{
				StartsAt: mustParseTime(t, "2026-04-06T10:30:00Z"),
				EndsAt:   mustParseTime(t, "2026-04-06T11:30:00Z"),
			},
		})
		if !errors.Is(err, ErrBlockConflictRejected) {
			t.Fatalf("CreateBlock(exclusive sibling conflict) error = %v, want %v", err, ErrBlockConflictRejected)
		}
	})

	t.Run("contains and composes cycles are rejected", func(t *testing.T) {
		service, actor, cleanup := newScheduleServiceFixture(t)
		defer cleanup()

		fullCourt := mustUpsertResource(t, service, actor, "full-court")
		halfCourt := mustUpsertResource(t, service, actor, "half-court-a")
		scoreboard := mustUpsertResource(t, service, actor, "scoreboard")

		if _, err := service.UpsertResourceEdge(context.Background(), actor, ResourceEdgeInput{
			ResourceKey:        fullCourt.ResourceKey,
			RelatedResourceKey: halfCourt.ResourceKey,
			EdgeType:           EdgeContains,
		}); err != nil {
			t.Fatalf("UpsertResourceEdge(full contains half) error = %v", err)
		}
		if _, err := service.UpsertResourceEdge(context.Background(), actor, ResourceEdgeInput{
			ResourceKey:        fullCourt.ResourceKey,
			RelatedResourceKey: scoreboard.ResourceKey,
			EdgeType:           EdgeComposes,
		}); err != nil {
			t.Fatalf("UpsertResourceEdge(full composes scoreboard) error = %v", err)
		}

		if _, err := service.UpsertResourceEdge(context.Background(), actor, ResourceEdgeInput{
			ResourceKey:        halfCourt.ResourceKey,
			RelatedResourceKey: fullCourt.ResourceKey,
			EdgeType:           EdgeContains,
		}); !errors.Is(err, ErrResourceEdgeCycle) {
			t.Fatalf("UpsertResourceEdge(contains cycle) error = %v, want %v", err, ErrResourceEdgeCycle)
		}

		if _, err := service.UpsertResourceEdge(context.Background(), actor, ResourceEdgeInput{
			ResourceKey:        scoreboard.ResourceKey,
			RelatedResourceKey: fullCourt.ResourceKey,
			EdgeType:           EdgeComposes,
		}); !errors.Is(err, ErrResourceEdgeCycle) {
			t.Fatalf("UpsertResourceEdge(composes cycle) error = %v, want %v", err, ErrResourceEdgeCycle)
		}
	})
}

func TestScheduleRecurrenceExceptionsAndStaleWrites(t *testing.T) {
	service, actor, cleanup := newScheduleServiceFixture(t)
	defer cleanup()

	weeklyBlock, err := service.CreateBlock(context.Background(), actor, BlockInput{
		FacilityKey: "ashtonbee",
		Scope:       ScopeFacility,
		Kind:        KindOperatingHours,
		Effect:      EffectInformational,
		Visibility:  VisibilityPublicLabeled,
		Weekly: &WeeklyInput{
			Weekday:             1,
			StartTime:           "09:00",
			EndTime:             "10:00",
			Timezone:            "America/Toronto",
			RecurrenceStartDate: "2026-04-06",
			RecurrenceEndDate:   stringPtr("2026-04-20"),
		},
	})
	if err != nil {
		t.Fatalf("CreateBlock(weekly) error = %v", err)
	}

	occurrences, err := service.GetCalendar(context.Background(), "ashtonbee", CalendarWindow{
		From:  mustParseTime(t, "2026-04-06T00:00:00Z"),
		Until: mustParseTime(t, "2026-04-27T00:00:00Z"),
	})
	if err != nil {
		t.Fatalf("GetCalendar() error = %v", err)
	}
	if len(occurrences) != 3 {
		t.Fatalf("len(occurrences) = %d, want 3", len(occurrences))
	}

	updated, err := service.AddException(context.Background(), actor, weeklyBlock.ID, 1, BlockExceptionInput{ExceptionDate: "2026-04-13"})
	if err != nil {
		t.Fatalf("AddException() error = %v", err)
	}
	if updated.Version != 2 {
		t.Fatalf("updated.Version = %d, want 2", updated.Version)
	}

	occurrences, err = service.GetCalendar(context.Background(), "ashtonbee", CalendarWindow{
		From:  mustParseTime(t, "2026-04-06T00:00:00Z"),
		Until: mustParseTime(t, "2026-04-27T00:00:00Z"),
	})
	if err != nil {
		t.Fatalf("GetCalendar(after exception) error = %v", err)
	}
	if len(occurrences) != 2 {
		t.Fatalf("len(occurrences) = %d, want 2", len(occurrences))
	}
	for _, occurrence := range occurrences {
		if occurrence.OccurrenceDate == "2026-04-13" {
			t.Fatal("exception date still present in calendar output")
		}
	}

	if _, err := service.AddException(context.Background(), actor, weeklyBlock.ID, 1, BlockExceptionInput{ExceptionDate: "2026-04-20"}); !errors.Is(err, ErrBlockVersionStale) {
		t.Fatalf("AddException(stale) error = %v, want %v", err, ErrBlockVersionStale)
	}
	if _, err := service.CancelBlock(context.Background(), actor, weeklyBlock.ID, 1); !errors.Is(err, ErrBlockVersionStale) {
		t.Fatalf("CancelBlock(stale) error = %v, want %v", err, ErrBlockVersionStale)
	}

	cancelled, err := service.CancelBlock(context.Background(), actor, weeklyBlock.ID, 2)
	if err != nil {
		t.Fatalf("CancelBlock() error = %v", err)
	}
	if cancelled.Status != StatusCancelled {
		t.Fatalf("cancelled.Status = %q, want %q", cancelled.Status, StatusCancelled)
	}

	if _, err := service.CreateBlock(context.Background(), actor, BlockInput{
		FacilityKey: "ashtonbee",
		Scope:       ScopeFacility,
		Kind:        KindOperatingHours,
		Effect:      EffectInformational,
		Visibility:  VisibilityPublicLabeled,
		Weekly: &WeeklyInput{
			Weekday:             1,
			StartTime:           "09:00",
			EndTime:             "10:00",
			Timezone:            "America/Toronto",
			RecurrenceStartDate: "2026-04-06",
		},
	}); !errors.Is(err, ErrBlockRecurrenceInvalid) {
		t.Fatalf("CreateBlock(missing recurrence end) error = %v, want %v", err, ErrBlockRecurrenceInvalid)
	}

	if _, err := service.GetCalendar(context.Background(), "ashtonbee", CalendarWindow{
		From:  mustParseTime(t, "2026-04-01T00:00:00Z"),
		Until: mustParseTime(t, "2026-08-01T00:00:00Z"),
	}); !errors.Is(err, ErrBlockDateWindowTooLarge) {
		t.Fatalf("GetCalendar(oversized window) error = %v, want %v", err, ErrBlockDateWindowTooLarge)
	}
}

func TestScheduleValidationAndVisibilityRules(t *testing.T) {
	service, actor, cleanup := newScheduleServiceFixture(t)
	defer cleanup()

	if _, err := service.CreateBlock(context.Background(), actor, BlockInput{
		FacilityKey: "ashtonbee",
		Scope:       ScopeZone,
		ResourceKey: stringPtr("half-court-a"),
		Kind:        KindReservation,
		Effect:      EffectHardReserve,
		Visibility:  VisibilityInternal,
		OneOff: &OneOffInput{
			StartsAt: mustParseTime(t, "2026-04-06T10:00:00Z"),
			EndsAt:   mustParseTime(t, "2026-04-06T11:00:00Z"),
		},
	}); !errors.Is(err, ErrBlockScopeInvalid) {
		t.Fatalf("CreateBlock(invalid scope refs) error = %v, want %v", err, ErrBlockScopeInvalid)
	}

	if _, err := service.CreateBlock(context.Background(), actor, BlockInput{
		FacilityKey: "ashtonbee",
		Scope:       ScopeFacility,
		Kind:        KindHold,
		Effect:      EffectSoftHold,
		Visibility:  VisibilityPublicBusy,
		OneOff: &OneOffInput{
			StartsAt: mustParseTime(t, "2026-04-06T10:00:00Z"),
			EndsAt:   mustParseTime(t, "2026-04-06T11:00:00Z"),
		},
	}); !errors.Is(err, ErrBlockVisibilityInvalid) {
		t.Fatalf("CreateBlock(hold visibility) error = %v, want %v", err, ErrBlockVisibilityInvalid)
	}

	operating, err := service.CreateBlock(context.Background(), actor, BlockInput{
		FacilityKey: "ashtonbee",
		Scope:       ScopeFacility,
		Kind:        KindOperatingHours,
		Effect:      EffectInformational,
		Visibility:  VisibilityPublicLabeled,
		OneOff: &OneOffInput{
			StartsAt: mustParseTime(t, "2026-04-06T10:00:00Z"),
			EndsAt:   mustParseTime(t, "2026-04-06T11:00:00Z"),
		},
	})
	if err != nil {
		t.Fatalf("CreateBlock(operating_hours) error = %v", err)
	}

	if _, err := service.CreateBlock(context.Background(), actor, BlockInput{
		FacilityKey: "ashtonbee",
		Scope:       ScopeFacility,
		Kind:        KindOperatingHours,
		Effect:      EffectInformational,
		Visibility:  VisibilityPublicLabeled,
		OneOff: &OneOffInput{
			StartsAt: mustParseTime(t, "2026-04-06T10:30:00Z"),
			EndsAt:   mustParseTime(t, "2026-04-06T11:30:00Z"),
		},
	}); !errors.Is(err, ErrBlockOperatingHoursOverlap) {
		t.Fatalf("CreateBlock(overlapping operating_hours) error = %v, want %v", err, ErrBlockOperatingHoursOverlap)
	}

	closure, err := service.CreateBlock(context.Background(), actor, BlockInput{
		FacilityKey: "ashtonbee",
		Scope:       ScopeFacility,
		Kind:        KindClosure,
		Effect:      EffectClosed,
		Visibility:  VisibilityInternal,
		OneOff: &OneOffInput{
			StartsAt: mustParseTime(t, "2026-04-06T10:00:00Z"),
			EndsAt:   mustParseTime(t, "2026-04-06T11:00:00Z"),
		},
	})
	if err != nil {
		t.Fatalf("CreateBlock(closure) error = %v", err)
	}
	if len(closure.Conflicts) == 0 {
		t.Fatal("closure.Conflicts = empty, want surfaced overlap")
	}

	event, err := service.CreateBlock(context.Background(), actor, BlockInput{
		FacilityKey: "ashtonbee",
		Scope:       ScopeFacility,
		Kind:        KindEvent,
		Effect:      EffectInformational,
		Visibility:  VisibilityPublicBusy,
		OneOff: &OneOffInput{
			StartsAt: mustParseTime(t, "2026-04-06T10:00:00Z"),
			EndsAt:   mustParseTime(t, "2026-04-06T11:00:00Z"),
		},
	})
	if err != nil {
		t.Fatalf("CreateBlock(event) error = %v", err)
	}
	if len(event.Conflicts) == 0 {
		t.Fatal("event.Conflicts = empty, want surfaced overlap")
	}

	weeklyMonday, err := service.CreateBlock(context.Background(), actor, BlockInput{
		FacilityKey: "ashtonbee",
		Scope:       ScopeFacility,
		Kind:        KindOperatingHours,
		Effect:      EffectInformational,
		Visibility:  VisibilityPublicLabeled,
		Weekly: &WeeklyInput{
			Weekday:             1,
			StartTime:           "09:00",
			EndTime:             "10:00",
			Timezone:            "America/Toronto",
			RecurrenceStartDate: "2026-04-06",
			RecurrenceEndDate:   stringPtr("2026-04-20"),
		},
	})
	if err != nil {
		t.Fatalf("CreateBlock(weekly Monday operating_hours) error = %v", err)
	}

	weeklyTuesday, err := service.CreateBlock(context.Background(), actor, BlockInput{
		FacilityKey: "ashtonbee",
		Scope:       ScopeFacility,
		Kind:        KindOperatingHours,
		Effect:      EffectInformational,
		Visibility:  VisibilityPublicLabeled,
		Weekly: &WeeklyInput{
			Weekday:             2,
			StartTime:           "09:00",
			EndTime:             "10:00",
			Timezone:            "America/Toronto",
			RecurrenceStartDate: "2026-04-06",
			RecurrenceEndDate:   stringPtr("2026-04-20"),
		},
	})
	if err != nil {
		t.Fatalf("CreateBlock(weekly Tuesday operating_hours) error = %v", err)
	}
	if len(weeklyMonday.Conflicts) != 0 || len(weeklyTuesday.Conflicts) != 0 {
		t.Fatalf("weekly operating hours should not conflict across distinct weekdays, got %#v %#v", weeklyMonday.Conflicts, weeklyTuesday.Conflicts)
	}

	halfA := mustUpsertResource(t, service, actor, "half-court-a")
	halfB := mustUpsertResource(t, service, actor, "half-court-b")
	if _, err := service.CreateBlock(context.Background(), actor, BlockInput{
		FacilityKey: "ashtonbee",
		Scope:       ScopeResource,
		ResourceKey: &halfA.ResourceKey,
		Kind:        KindReservation,
		Effect:      EffectHardReserve,
		Visibility:  VisibilityInternal,
		OneOff: &OneOffInput{
			StartsAt: mustParseTime(t, "2026-04-07T10:00:00Z"),
			EndsAt:   mustParseTime(t, "2026-04-07T11:00:00Z"),
		},
	}); err != nil {
		t.Fatalf("CreateBlock(half-a reservation) error = %v", err)
	}

	unrelatedClosure, err := service.CreateBlock(context.Background(), actor, BlockInput{
		FacilityKey: "ashtonbee",
		Scope:       ScopeResource,
		ResourceKey: &halfB.ResourceKey,
		Kind:        KindClosure,
		Effect:      EffectClosed,
		Visibility:  VisibilityInternal,
		OneOff: &OneOffInput{
			StartsAt: mustParseTime(t, "2026-04-07T10:00:00Z"),
			EndsAt:   mustParseTime(t, "2026-04-07T11:00:00Z"),
		},
	})
	if err != nil {
		t.Fatalf("CreateBlock(unrelated closure) error = %v", err)
	}
	if len(unrelatedClosure.Conflicts) != 0 {
		t.Fatalf("unrelated closure conflicts = %#v, want none", unrelatedClosure.Conflicts)
	}

	_ = operating
}

func TestScheduleClaimableInventoryRules(t *testing.T) {
	t.Run("resource hard claims require an active and bookable target", func(t *testing.T) {
		service, actor, cleanup := newScheduleServiceFixture(t)
		defer cleanup()

		inactive := mustUpsertResourceWithState(t, service, actor, "inactive-court", true, false)
		if _, err := service.CreateBlock(context.Background(), actor, BlockInput{
			FacilityKey: "ashtonbee",
			Scope:       ScopeResource,
			ResourceKey: &inactive.ResourceKey,
			Kind:        KindReservation,
			Effect:      EffectHardReserve,
			Visibility:  VisibilityInternal,
			OneOff: &OneOffInput{
				StartsAt: mustParseTime(t, "2026-04-06T10:00:00Z"),
				EndsAt:   mustParseTime(t, "2026-04-06T11:00:00Z"),
			},
		}); !errors.Is(err, ErrBlockResourceNotClaimable) {
			t.Fatalf("CreateBlock(inactive reservation) error = %v, want %v", err, ErrBlockResourceNotClaimable)
		}

		nonBookable := mustUpsertResourceWithState(t, service, actor, "private-court", false, true)
		if _, err := service.CreateBlock(context.Background(), actor, BlockInput{
			FacilityKey: "ashtonbee",
			Scope:       ScopeResource,
			ResourceKey: &nonBookable.ResourceKey,
			Kind:        KindHold,
			Effect:      EffectSoftHold,
			Visibility:  VisibilityInternal,
			OneOff: &OneOffInput{
				StartsAt: mustParseTime(t, "2026-04-06T11:00:00Z"),
				EndsAt:   mustParseTime(t, "2026-04-06T12:00:00Z"),
			},
		}); !errors.Is(err, ErrBlockResourceNotClaimable) {
			t.Fatalf("CreateBlock(non-bookable hold) error = %v, want %v", err, ErrBlockResourceNotClaimable)
		}
	})

	t.Run("zone and facility hard claims require claimable coverage", func(t *testing.T) {
		service, actor, cleanup := newScheduleServiceFixture(t)
		defer cleanup()

		mustUpsertResourceWithState(t, service, actor, "inactive-zone-court", true, false)

		zoneKey := "gym-floor"
		if _, err := service.CreateBlock(context.Background(), actor, BlockInput{
			FacilityKey: "ashtonbee",
			ZoneKey:     &zoneKey,
			Scope:       ScopeZone,
			Kind:        KindReservation,
			Effect:      EffectHardReserve,
			Visibility:  VisibilityInternal,
			OneOff: &OneOffInput{
				StartsAt: mustParseTime(t, "2026-04-06T10:00:00Z"),
				EndsAt:   mustParseTime(t, "2026-04-06T11:00:00Z"),
			},
		}); !errors.Is(err, ErrBlockClaimableScopeEmpty) {
			t.Fatalf("CreateBlock(zone reservation without claimable inventory) error = %v, want %v", err, ErrBlockClaimableScopeEmpty)
		}

		if _, err := service.CreateBlock(context.Background(), actor, BlockInput{
			FacilityKey: "ashtonbee",
			Scope:       ScopeFacility,
			Kind:        KindHold,
			Effect:      EffectSoftHold,
			Visibility:  VisibilityInternal,
			OneOff: &OneOffInput{
				StartsAt: mustParseTime(t, "2026-04-06T11:00:00Z"),
				EndsAt:   mustParseTime(t, "2026-04-06T12:00:00Z"),
			},
		}); !errors.Is(err, ErrBlockClaimableScopeEmpty) {
			t.Fatalf("CreateBlock(facility hold without claimable inventory) error = %v, want %v", err, ErrBlockClaimableScopeEmpty)
		}
	})

	t.Run("descriptive blocks still use scope semantics", func(t *testing.T) {
		service, actor, cleanup := newScheduleServiceFixture(t)
		defer cleanup()

		inactive := mustUpsertResourceWithState(t, service, actor, "inactive-court", true, false)
		nonBookable := mustUpsertResourceWithState(t, service, actor, "private-court", false, true)

		if _, err := service.CreateBlock(context.Background(), actor, BlockInput{
			FacilityKey: "ashtonbee",
			Scope:       ScopeResource,
			ResourceKey: &inactive.ResourceKey,
			Kind:        KindClosure,
			Effect:      EffectClosed,
			Visibility:  VisibilityInternal,
			OneOff: &OneOffInput{
				StartsAt: mustParseTime(t, "2026-04-06T10:00:00Z"),
				EndsAt:   mustParseTime(t, "2026-04-06T11:00:00Z"),
			},
		}); err != nil {
			t.Fatalf("CreateBlock(inactive closure) error = %v", err)
		}

		if _, err := service.CreateBlock(context.Background(), actor, BlockInput{
			FacilityKey: "ashtonbee",
			Scope:       ScopeResource,
			ResourceKey: &nonBookable.ResourceKey,
			Kind:        KindEvent,
			Effect:      EffectInformational,
			Visibility:  VisibilityInternal,
			OneOff: &OneOffInput{
				StartsAt: mustParseTime(t, "2026-04-06T11:00:00Z"),
				EndsAt:   mustParseTime(t, "2026-04-06T12:00:00Z"),
			},
		}); err != nil {
			t.Fatalf("CreateBlock(non-bookable informational event) error = %v", err)
		}
	})

	t.Run("hard claim coverage excludes inactive and non-bookable inventory", func(t *testing.T) {
		service, actor, cleanup := newScheduleServiceFixture(t)
		defer cleanup()

		mustUpsertResourceWithState(t, service, actor, "claimable-court", true, true)
		mustUpsertResourceWithState(t, service, actor, "inactive-court", true, false)
		mustUpsertResourceWithState(t, service, actor, "private-court", false, true)

		snapshot, err := service.loadFacilitySnapshotFromRepository(context.Background(), "ashtonbee")
		if err != nil {
			t.Fatalf("loadFacilitySnapshotFromRepository() error = %v", err)
		}

		coverage := claimCoverageFromBlock(Block{
			FacilityKey: "ashtonbee",
			Scope:       ScopeFacility,
			Kind:        KindReservation,
			Effect:      EffectHardReserve,
			Visibility:  VisibilityInternal,
		}, snapshot)
		if len(coverage) != 1 {
			t.Fatalf("len(claim coverage) = %d, want 1", len(coverage))
		}
		if _, ok := coverage["claimable-court"]; !ok {
			t.Fatalf("claim coverage missing claimable-court: %#v", coverage)
		}
		if _, ok := coverage["inactive-court"]; ok {
			t.Fatalf("claim coverage unexpectedly includes inactive-court: %#v", coverage)
		}
		if _, ok := coverage["private-court"]; ok {
			t.Fatalf("claim coverage unexpectedly includes private-court: %#v", coverage)
		}
	})
}

func newScheduleServiceFixture(t *testing.T) (*Service, StaffActor, func()) {
	t.Helper()

	ctx := context.Background()
	postgresEnv, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres() error = %v", err)
	}
	if err := testutil.ApplyApolloSchema(ctx, postgresEnv.DB); err != nil {
		_ = postgresEnv.Close()
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}

	userID := insertScheduleUser(t, ctx, postgresEnv, "schedule-owner-001", "Schedule Owner", "schedule-owner-001@example.com")
	sessionID := insertScheduleSession(t, ctx, postgresEnv, userID)

	service := NewService(NewRepository(postgresEnv.DB))
	actor := StaffActor{
		UserID:              userID,
		SessionID:           sessionID,
		Role:                authz.RoleOwner,
		Capability:          authz.CapabilityScheduleManage,
		TrustedSurfaceKey:   "staff-console",
		TrustedSurfaceLabel: "staff-console",
	}

	return service, actor, func() {
		if closeErr := postgresEnv.Close(); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	}
}

func mustUpsertResource(t *testing.T, service *Service, actor StaffActor, resourceKey string) Resource {
	t.Helper()

	return mustUpsertResourceWithState(t, service, actor, resourceKey, true, true)
}

func mustUpsertResourceWithState(t *testing.T, service *Service, actor StaffActor, resourceKey string, bookable bool, active bool) Resource {
	t.Helper()

	row, err := service.UpsertResource(context.Background(), actor, ResourceInput{
		ResourceKey:  resourceKey,
		FacilityKey:  "ashtonbee",
		ZoneKey:      stringPtr("gym-floor"),
		ResourceType: "court",
		DisplayName:  resourceKey,
		Bookable:     bookable,
		Active:       active,
	})
	if err != nil {
		t.Fatalf("UpsertResource(%s) error = %v", resourceKey, err)
	}
	return row
}

func mustCreateOneOffBlock(t *testing.T, service *Service, actor StaffActor, resourceKey string, startsAt string, endsAt string) Block {
	t.Helper()

	block, err := service.CreateBlock(context.Background(), actor, BlockInput{
		FacilityKey: "ashtonbee",
		Scope:       ScopeResource,
		ResourceKey: &resourceKey,
		Kind:        KindReservation,
		Effect:      EffectHardReserve,
		Visibility:  VisibilityInternal,
		OneOff: &OneOffInput{
			StartsAt: mustParseTime(t, startsAt),
			EndsAt:   mustParseTime(t, endsAt),
		},
	})
	if err != nil {
		t.Fatalf("CreateBlock(%s) error = %v", resourceKey, err)
	}
	return block
}

func mustParseTime(t *testing.T, raw string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("time.Parse(%q) error = %v", raw, err)
	}
	return parsed
}

func insertScheduleUser(t *testing.T, ctx context.Context, postgresEnv *testutil.PostgresEnv, studentID string, displayName string, email string) uuid.UUID {
	t.Helper()

	var userID uuid.UUID
	if err := postgresEnv.DB.QueryRow(ctx, `
INSERT INTO apollo.users (student_id, display_name, email)
VALUES ($1, $2, $3)
RETURNING id
`, studentID, displayName, email).Scan(&userID); err != nil {
		t.Fatalf("insert schedule user error = %v", err)
	}
	return userID
}

func insertScheduleSession(t *testing.T, ctx context.Context, postgresEnv *testutil.PostgresEnv, ownerUserID uuid.UUID) uuid.UUID {
	t.Helper()

	var sessionID uuid.UUID
	if err := postgresEnv.DB.QueryRow(ctx, `
INSERT INTO apollo.sessions (user_id, expires_at, revoked_at)
VALUES ($1, NOW() + INTERVAL '1 hour', NULL)
RETURNING id
`, ownerUserID).Scan(&sessionID); err != nil {
		t.Fatalf("insert schedule session error = %v", err)
	}
	return sessionID
}
