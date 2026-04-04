import assert from "node:assert/strict";
import test from "node:test";

import {
  buildWorkoutPayload,
  extractErrorMessage,
  recommendationSummary,
  selectWorkoutID,
  workoutListLabel,
} from "./app.mjs";

test("selectWorkoutID prefers the existing selection when it still exists", () => {
  const workouts = [{ id: "a" }, { id: "b" }];
  assert.equal(selectWorkoutID(workouts, "b"), "b");
});

test("selectWorkoutID falls back to the first workout in server order", () => {
  const workouts = [{ id: "newest" }, { id: "older" }];
  assert.equal(selectWorkoutID(workouts, "missing"), "newest");
  assert.equal(selectWorkoutID([], "missing"), null);
});

test("buildWorkoutPayload preserves explicit exercise rows and optional fields", () => {
  assert.deepEqual(
    buildWorkoutPayload("  upper  ", [
      {
        name: " bench press ",
        sets: "3",
        reps: "8",
        weightKg: "84.5",
        rpe: "8.5",
        notes: " heavy ",
      },
      {
        name: "row",
        sets: "3",
        reps: "10",
        weightKg: "",
        rpe: "",
        notes: "",
      },
    ]),
    {
      notes: "upper",
      exercises: [
        {
          name: "bench press",
          sets: 3,
          reps: 8,
          weight_kg: 84.5,
          rpe: 8.5,
          notes: "heavy",
        },
        {
          name: "row",
          sets: 3,
          reps: 10,
          weight_kg: null,
          rpe: null,
          notes: null,
        },
      ],
    },
  );
});

test("recommendationSummary maps deterministic recommendation types", () => {
  assert.deepEqual(recommendationSummary({ type: "recovery_day", reason: "last_finished_within_recovery_window" }), {
    headline: "Take a recovery day",
    detail: "Your latest finished workout is still inside the 24-hour recovery window.",
  });
});

test("extractErrorMessage prefers API error strings and falls back clearly", () => {
  assert.equal(extractErrorMessage({ error: "workout is already finished" }, "fallback"), "workout is already finished");
  assert.equal(extractErrorMessage({}, "fallback"), "fallback");
});

test("workoutListLabel uses server timestamps and exercise counts without reordering", () => {
  const label = workoutListLabel({
    started_at: "2026-04-04T12:00:00Z",
    exercises: [{}, {}],
  });
  assert.match(label, /2 exercises/);
});
