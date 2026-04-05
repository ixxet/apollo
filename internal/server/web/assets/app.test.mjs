import assert from "node:assert/strict";
import test from "node:test";

import {
  buildWorkoutPayload,
  extractErrorMessage,
  membershipSummary,
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

test("membershipSummary maps explicit lobby membership states", () => {
  const joined = membershipSummary({ status: "joined", joined_at: "2026-04-05T12:00:00Z" });
  assert.equal(joined.headline, "Joined lobby");
  assert.match(joined.detail, /APOLLO recorded explicit lobby membership at/);

  assert.deepEqual(membershipSummary({ status: "not_joined" }), {
    headline: "Not joined",
    detail: "Lobby membership stays explicit. Join only when you intend to be in the lobby.",
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

test("shell bootstrap maps total fetch rejection into explicit error UI without leaking rejections", async () => {
  await assertShellFailureState({
    fetchImpl: async () => {
      throw new Error("network down");
    },
    exerciseRefresh: false,
  });
});

test("shell refresh maps total fetch rejection into explicit error UI without leaking rejections", async () => {
  let requestCount = 0;
  await assertShellFailureState({
    fetchImpl: async (path) => {
      requestCount += 1;
      if (requestCount <= 4) {
        return successResponseForPath(path);
      }
      throw new Error("network down");
    },
    exerciseRefresh: true,
  });
});

test("shell bootstrap renders explicit lobby membership state", async () => {
  const { cleanup, elements, loadShellModule } = installShellHarness((path) => successResponseForPath(path));

  try {
    await loadShellModule();

    assert.equal(elements["#membership-status"].textContent, "Membership loaded.");
    assert.match(elements["#membership-card"].innerHTML, /Not joined/);
    assert.equal(elements["#join-lobby"].hidden, false);
    assert.equal(elements["#leave-lobby"].hidden, true);
  } finally {
    cleanup();
  }
});

test("shell join success updates membership state and actions", async () => {
  let joined = false;
  const { cleanup, elements, loadShellModule } = installShellHarness(async (path, options = {}) => {
    if (path === "/api/v1/lobby/membership/join" && options.method === "POST") {
      joined = true;
      return {
        ok: true,
        status: 200,
        async json() {
          return {
            status: "joined",
            joined_at: "2026-04-05T12:30:00Z",
          };
        },
      };
    }

    return successResponseForPath(path, { membershipStatus: joined ? "joined" : "not_joined" });
  });

  try {
    await loadShellModule();
    await elements["#join-lobby"].trigger("click");
    await settle();

    assert.equal(elements["#membership-status"].textContent, "Lobby membership joined.");
    assert.equal(elements["#membership-status"].classNames.has("success-message"), true);
    assert.match(elements["#membership-card"].innerHTML, /Joined lobby/);
    assert.equal(elements["#join-lobby"].hidden, true);
    assert.equal(elements["#leave-lobby"].hidden, false);
  } finally {
    cleanup();
  }
});

test("shell leave success updates membership state and actions", async () => {
  const { cleanup, elements, loadShellModule } = installShellHarness(async (path, options = {}) => {
    if (path === "/api/v1/lobby/membership" && (!options.method || options.method === "GET")) {
      return successResponseForPath(path, { membershipStatus: "joined" });
    }
    if (path === "/api/v1/lobby/membership/leave" && options.method === "POST") {
      return {
        ok: true,
        status: 200,
        async json() {
          return {
            status: "not_joined",
            joined_at: "2026-04-05T12:30:00Z",
            left_at: "2026-04-05T13:00:00Z",
          };
        },
      };
    }

    return successResponseForPath(path, { membershipStatus: "joined" });
  });

  try {
    await loadShellModule();
    await elements["#leave-lobby"].trigger("click");
    await settle();

    assert.equal(elements["#membership-status"].textContent, "Lobby membership left.");
    assert.equal(elements["#membership-status"].classNames.has("success-message"), true);
    assert.match(elements["#membership-card"].innerHTML, /Not joined/);
    assert.equal(elements["#join-lobby"].hidden, false);
    assert.equal(elements["#leave-lobby"].hidden, true);
  } finally {
    cleanup();
  }
});

test("shell join failure maps explicit API error without inventing joined state", async () => {
  const { cleanup, elements, loadShellModule } = installShellHarness(async (path, options = {}) => {
    if (path === "/api/v1/lobby/membership/join" && options.method === "POST") {
      return {
        ok: false,
        status: 409,
        async json() {
          return { error: "member is not eligible for lobby membership: visibility_ghost" };
        },
      };
    }

    return successResponseForPath(path);
  });

  try {
    await loadShellModule();
    await elements["#join-lobby"].trigger("click");
    await settle();

    assert.equal(elements["#membership-status"].textContent, "member is not eligible for lobby membership: visibility_ghost");
    assert.equal(elements["#membership-status"].classNames.has("error-message"), true);
    assert.match(elements["#membership-card"].innerHTML, /Not joined/);
    assert.equal(elements["#join-lobby"].hidden, false);
    assert.equal(elements["#leave-lobby"].hidden, true);
  } finally {
    cleanup();
  }
});

test("shell join network failure maps explicit UI error without leaking rejection", async () => {
  const { cleanup, elements, loadShellModule } = installShellHarness(async (path, options = {}) => {
    if (path === "/api/v1/lobby/membership/join" && options.method === "POST") {
      throw new Error("network down");
    }

    return successResponseForPath(path);
  });
  let unhandled = null;
  const handleUnhandledRejection = (error) => {
    unhandled = error;
  };
  process.on("unhandledRejection", handleUnhandledRejection);

  try {
    await loadShellModule();
    await elements["#join-lobby"].trigger("click");
    await settle();

    assert.equal(unhandled, null);
    assert.equal(elements["#membership-status"].textContent, "Unable to update lobby membership. Check your connection and try again.");
    assert.equal(elements["#membership-status"].classNames.has("error-message"), true);
    assert.match(elements["#membership-card"].innerHTML, /Unable to update lobby membership/);
  } finally {
    process.off("unhandledRejection", handleUnhandledRejection);
    cleanup();
  }
});

async function assertShellFailureState({ fetchImpl, exerciseRefresh }) {
  const { cleanup, elements, loadShellModule } = installShellHarness(fetchImpl);
  let unhandled = null;
  const handleUnhandledRejection = (error) => {
    unhandled = error;
  };
  process.on("unhandledRejection", handleUnhandledRejection);

  try {
    await loadShellModule();
    if (exerciseRefresh) {
      elements["#refresh-shell"].trigger("click");
      await settle();
    }

    assert.equal(unhandled, null);
    assert.equal(elements["#profile-status"].textContent, "Unable to load profile. Check your connection and refresh.");
    assert.equal(elements["#membership-status"].textContent, "Unable to load lobby membership. Check your connection and refresh.");
    assert.equal(elements["#recommendation-status"].textContent, "Unable to load recommendation. Check your connection and refresh.");
    assert.equal(elements["#workouts-status"].textContent, "Unable to load workouts. Check your connection and refresh.");
    assert.equal(elements["#profile-status"].classNames.has("error-message"), true);
    assert.equal(elements["#membership-status"].classNames.has("error-message"), true);
    assert.equal(elements["#recommendation-status"].classNames.has("error-message"), true);
    assert.equal(elements["#workouts-status"].classNames.has("error-message"), true);
    assert.match(elements["#membership-card"].innerHTML, /Unable to load lobby membership/);
    assert.match(elements["#recommendation-card"].innerHTML, /Unable to load recommendation/);
    assert.match(elements["#workout-list"].innerHTML, /Unable to load workouts/);
    assert.equal(elements["#profile-summary"].innerHTML, "");
    assert.notEqual(elements["#profile-status"].textContent, "Loading profile…");
    assert.notEqual(elements["#membership-status"].textContent, "Loading membership…");
    assert.notEqual(elements["#recommendation-status"].textContent, "Loading recommendation…");
    assert.notEqual(elements["#workouts-status"].textContent, "Loading workouts…");
  } finally {
    process.off("unhandledRejection", handleUnhandledRejection);
    cleanup();
  }
}

function installShellHarness(fetchImpl) {
  const previousDocument = globalThis.document;
  const previousWindow = globalThis.window;
  const previousFetch = globalThis.fetch;

  const elements = {
    "#profile-summary": new FakeElement(),
    "#profile-status": new FakeElement(),
    "#membership-card": new FakeElement(),
    "#membership-status": new FakeElement(),
    "#join-lobby": new FakeElement(),
    "#leave-lobby": new FakeElement(),
    "#recommendation-card": new FakeElement(),
    "#recommendation-status": new FakeElement(),
    "#workout-list": new FakeElement(),
    "#workouts-status": new FakeElement(),
    "#workout-detail-title": new FakeElement(),
    "#workout-detail-state": new FakeElement(),
    "#workout-notes": new FakeElement(),
    "#exercise-list": new FakeElement(),
    "#workout-error": new FakeElement(),
    "#save-workout": new FakeElement(),
    "#finish-workout": new FakeElement(),
    "#refresh-shell": new FakeElement(),
    "#logout-shell": new FakeElement(),
    "#create-workout": new FakeElement(),
    "#add-exercise": new FakeElement(),
    "#workout-editor": new FakeElement(),
  };

  globalThis.document = {
    body: { dataset: { apolloView: "shell" } },
    querySelector(selector) {
      return elements[selector] ?? null;
    },
  };
  globalThis.window = {
    location: {
      href: "http://127.0.0.1/app",
      assign() {},
    },
  };
  globalThis.fetch = fetchImpl;

  return {
    elements,
    async loadShellModule() {
      await import(new URL(`./app.mjs?test=${Date.now()}-${Math.random()}`, import.meta.url));
      await settle();
    },
    cleanup() {
      globalThis.document = previousDocument;
      globalThis.window = previousWindow;
      globalThis.fetch = previousFetch;
    },
  };
}

function successResponseForPath(path, options = {}) {
  const membershipStatus = options.membershipStatus ?? "not_joined";

  if (path === "/api/v1/lobby/membership") {
    return {
      ok: true,
      status: 200,
      async json() {
        if (membershipStatus === "joined") {
          return {
            status: "joined",
            joined_at: "2026-04-05T12:30:00Z",
          };
        }

        return {
          status: "not_joined",
        };
      },
    };
  }

  if (path === "/api/v1/profile") {
    return {
      ok: true,
      status: 200,
      async json() {
        return {
          display_name: "member",
          student_id: "student-011",
          email: "member@example.com",
          email_verified: true,
          visibility_mode: "ghost",
          availability_mode: "unavailable",
        };
      },
    };
  }

  if (path === "/api/v1/workouts") {
    return {
      ok: true,
      status: 200,
      async json() {
        return [];
      },
    };
  }

  if (path === "/api/v1/recommendations/workout") {
    return {
      ok: true,
      status: 200,
      async json() {
        return {
          type: "start_first_workout",
          reason: "no_finished_workouts",
          generated_at: "2026-04-05T12:00:00Z",
          evidence: {},
        };
      },
    };
  }

  throw new Error(`unexpected path ${path}`);
}

class FakeElement {
  constructor(value = "") {
    this.value = value;
    this.textContent = "";
    this.innerHTML = "";
    this.disabled = false;
    this.hidden = false;
    this.listeners = new Map();
    this.classNames = new Set();
    this.classList = {
      add: (...names) => names.forEach((name) => this.classNames.add(name)),
      remove: (...names) => names.forEach((name) => this.classNames.delete(name)),
    };
  }

  addEventListener(type, handler) {
    this.listeners.set(type, handler);
  }

  querySelector() {
    return null;
  }

  querySelectorAll() {
    return [];
  }

  trigger(type, event = {}) {
    const handler = this.listeners.get(type);
    if (!handler) {
      throw new Error(`missing handler for ${type}`);
    }
    return handler({
      preventDefault() {},
      target: this,
      ...event,
    });
  }
}

async function settle() {
  await new Promise((resolve) => setTimeout(resolve, 0));
  await new Promise((resolve) => setTimeout(resolve, 0));
}
