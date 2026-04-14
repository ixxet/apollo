import assert from "node:assert/strict";
import test from "node:test";

import {
  buildProfilePatchPayload,
  buildWorkoutPayload,
  DEFAULT_MEMBER_SHELL_SECTION,
  extractErrorMessage,
  memberShellPath,
  membershipSummary,
  normalizeShellSection,
  recommendationSummary,
  SECTION_API_PATHS,
  selectWorkoutID,
  shellSectionFromPath,
  workoutListLabel,
} from "./app.mjs";

test("normalizeShellSection and memberShellPath keep the routed shell stable", () => {
  assert.equal(normalizeShellSection("workouts"), "workouts");
  assert.equal(normalizeShellSection("WORKOUTS"), "workouts");
  assert.equal(normalizeShellSection("missing"), DEFAULT_MEMBER_SHELL_SECTION);
  assert.equal(memberShellPath("settings"), "/app/settings");
  assert.equal(shellSectionFromPath("/app/tournaments"), "tournaments");
  assert.equal(shellSectionFromPath("/app"), DEFAULT_MEMBER_SHELL_SECTION);
});

test("SECTION_API_PATHS stays member-safe and avoids staff schedule drift", () => {
  const joinedPaths = Object.values(SECTION_API_PATHS).flat().join("\n");
  assert.doesNotMatch(joinedPaths, /\/api\/v1\/schedule\//);
  assert.doesNotMatch(joinedPaths, /\/api\/v1\/competition\/sessions/);
});

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

test("buildProfilePatchPayload keeps settings writes backend-authoritative", () => {
  assert.deepEqual(
    buildProfilePatchPayload({
      visibilityMode: "discoverable",
      availabilityMode: "available_now",
      goalKey: "general-fitness",
      daysPerWeek: "4",
      sessionMinutes: "60",
      experienceLevel: "intermediate",
      preferredEquipmentKeys: "dumbbells, barbell",
      dietaryRestrictions: "vegetarian, nut_free",
      cuisinePreferences: "mediterranean, korean",
      budgetPreference: "moderate",
      cookingCapability: "basic_kitchen",
    }),
    {
      visibility_mode: "discoverable",
      availability_mode: "available_now",
      coaching_profile: {
        goal_key: "general-fitness",
        days_per_week: 4,
        session_minutes: 60,
        experience_level: "intermediate",
        preferred_equipment_keys: ["dumbbells", "barbell"],
      },
      nutrition_profile: {
        dietary_restrictions: ["vegetarian", "nut_free"],
        meal_preference: {
          cuisine_preferences: ["mediterranean", "korean"],
        },
        budget_preference: "moderate",
        cooking_capability: "basic_kitchen",
      },
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

test("shell bootstrap renders the routed home section over member-safe APIs", async () => {
  const { cleanup, elements, fetchCalls, loadShellModule } = installShellHarness({
    section: "home",
    fetchImpl: async (path) => successResponseForPath(path),
  });

  try {
    await loadShellModule();

    assert.equal(elements["#shell-status"].textContent, "Member shell ready.");
    assert.match(elements["#section-shell"].innerHTML, /Schedule boundary/);
    assert.deepEqual(fetchCalls, [
      "/api/v1/profile",
      "/api/v1/presence",
      "/api/v1/lobby/eligibility",
      "/api/v1/lobby/membership",
      "/api/v1/recommendations/workout",
    ]);
  } finally {
    cleanup();
  }
});

test("shell bootstrap maps total fetch rejection into explicit error UI without leaking rejections", async () => {
  const { cleanup, elements, loadShellModule } = installShellHarness({
    section: "home",
    fetchImpl: async () => {
      throw new Error("network down");
    },
  });

  try {
    await loadShellModule();
    assert.equal(elements["#shell-status"].textContent, "network down");
    assert.match(elements["#section-shell"].innerHTML, /Member shell bootstrap failed/);
  } finally {
    cleanup();
  }
});

test("tournaments section fetches only member-safe routes", async () => {
  const { cleanup, fetchCalls, loadShellModule } = installShellHarness({
    section: "tournaments",
    fetchImpl: async (path) => successResponseForPath(path),
  });

  try {
    await loadShellModule();
    assert.deepEqual(fetchCalls, [
      "/api/v1/profile",
      "/api/v1/lobby/membership",
      "/api/v1/lobby/match-preview",
      "/api/v1/competition/member-stats",
    ]);
  } finally {
    cleanup();
  }
});

function installShellHarness({ section, fetchImpl }) {
  const originals = {
    fetch: global.fetch,
    document: global.document,
    window: global.window,
    HTMLElement: global.HTMLElement,
  };

  const navLinks = ["home", "workouts", "meals", "tournaments", "settings"].map((navSection) => new FakeElement(`nav-${navSection}`, { dataset: { navSection } }));
  const elements = {
    "#refresh-shell": new FakeElement("refresh"),
    "#logout-shell": new FakeElement("logout"),
    "#shell-status": new FakeElement("shell-status"),
    "#section-shell": new FakeElement("section-shell"),
    "#section-eyebrow": new FakeElement("section-eyebrow"),
    "#section-title": new FakeElement("section-title"),
    "#section-copy": new FakeElement("section-copy"),
    "#section-retry": new FakeElement("section-retry"),
    "#membership-status": new FakeElement("membership-status"),
    "#join-lobby": new FakeElement("join-lobby"),
    "#leave-lobby": new FakeElement("leave-lobby"),
  };
  const fetchCalls = [];

  global.fetch = async (path, options) => {
    fetchCalls.push(path);
    return fetchImpl(path, options);
  };
  global.HTMLElement = FakeElement;
  global.window = {
    location: {
      pathname: `/app/${section}`,
      assigned: null,
      assign(value) {
        this.assigned = value;
      },
    },
  };
  global.document = {
    body: {
      dataset: {
        apolloView: "shell",
        apolloSection: section,
      },
    },
    querySelector(selector) {
      return elements[selector] || null;
    },
    querySelectorAll(selector) {
      if (selector === ".member-nav-link") {
        return navLinks;
      }
      return [];
    },
  };

  return {
    elements,
    fetchCalls,
    async loadShellModule() {
      const cacheBuster = Date.now() + Math.random();
      await import(new URL(`./app.mjs?case=${cacheBuster}`, import.meta.url));
      await settle();
    },
    cleanup() {
      global.fetch = originals.fetch;
      global.document = originals.document;
      global.window = originals.window;
      global.HTMLElement = originals.HTMLElement;
    },
  };
}

function successResponseForPath(path) {
  const payloads = {
    "/api/v1/profile": {
      user_id: "11111111-1111-1111-1111-111111111111",
      student_id: "student-001",
      display_name: "Member One",
      email: "member@example.com",
      email_verified: true,
      visibility_mode: "ghost",
      availability_mode: "unavailable",
      coaching_profile: {},
      nutrition_profile: {},
    },
    "/api/v1/presence": {
      facilities: [
        {
          facility_key: "ashtonbee",
          status: "present",
          recent_visits: [],
          streak: { status: "active", current_count: 3 },
        },
      ],
    },
    "/api/v1/lobby/eligibility": {
      eligible: false,
      reason: "availability_unavailable",
      visibility_mode: "ghost",
      availability_mode: "unavailable",
    },
    "/api/v1/lobby/membership": {
      status: "not_joined",
    },
    "/api/v1/recommendations/workout": {
      type: "start_first_workout",
      reason: "no_finished_workouts",
      generated_at: "2026-04-05T12:00:00Z",
    },
    "/api/v1/lobby/match-preview": {
      generated_at: "2026-04-06T12:00:00Z",
      candidate_count: 0,
      preview_version: "v1",
      matches: [],
      unmatched_member_ids: [],
      unmatched_labels: [],
    },
    "/api/v1/competition/member-stats": [],
  };

  const payload = payloads[path];
  if (!payload) {
    throw new Error(`unexpected path ${path}`);
  }

  return {
    ok: true,
    status: 200,
    headers: {
      get(name) {
        return name.toLowerCase() === "content-type" ? "application/json" : null;
      },
    },
    async json() {
      return payload;
    },
  };
}

class FakeElement {
  constructor(id, options = {}) {
    this.id = id;
    this.dataset = options.dataset || {};
    this.textContent = "";
    this.innerHTML = "";
    this.hidden = false;
    this.disabled = false;
    this.attributes = new Map();
    this.listeners = new Map();
    this.classList = new FakeClassList();
  }

  addEventListener(eventName, callback) {
    this.listeners.set(eventName, callback);
  }

  setAttribute(name, value) {
    this.attributes.set(name, value);
  }

  getAttribute(name) {
    return this.attributes.get(name) || null;
  }

  querySelector() {
    return null;
  }

  querySelectorAll() {
    return [];
  }

  insertAdjacentHTML(_position, value) {
    this.innerHTML += value;
  }
}

class FakeClassList {
  constructor() {
    this.classNames = new Set();
  }

  add(...names) {
    for (const name of names) {
      this.classNames.add(name);
    }
  }

  remove(...names) {
    for (const name of names) {
      this.classNames.delete(name);
    }
  }

  toggle(name, force) {
    if (force) {
      this.classNames.add(name);
      return true;
    }
    this.classNames.delete(name);
    return false;
  }
}

async function settle() {
  for (let index = 0; index < 5; index += 1) {
    await Promise.resolve();
    await new Promise((resolve) => setTimeout(resolve, 0));
  }
}
