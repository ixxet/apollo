export const MEMBER_SHELL_SECTIONS = ["home", "workouts", "meals", "tournaments", "settings"];
export const DEFAULT_MEMBER_SHELL_SECTION = "home";

const SECTION_META = {
  home: {
    eyebrow: "Home",
    title: "Member-safe summaries",
    copy: "Profile, presence, lobby intent, recommendation, and explicit shell boundaries over already-real member APIs.",
  },
  workouts: {
    eyebrow: "Workouts",
    title: "Workout runtime",
    copy: "Workout CRUD stays backend-authoritative. Planner reads stay bounded to existing member APIs.",
  },
  meals: {
    eyebrow: "Meals",
    title: "Nutrition and meal history",
    copy: "Nutrition guidance, meal logs, and meal templates stay read-only in this shell packet.",
  },
  tournaments: {
    eyebrow: "Tournaments",
    title: "Member competition posture",
    copy: "Lobby intent, deterministic match preview, and self-scoped competition stats only.",
  },
  settings: {
    eyebrow: "Settings",
    title: "Profile settings",
    copy: "Profile writes remain backend-authoritative through the existing member profile patch surface.",
  },
};

export const SECTION_API_PATHS = {
  home: [
    "/api/v1/presence",
    "/api/v1/lobby/eligibility",
    "/api/v1/lobby/membership",
    "/api/v1/recommendations/workout",
  ],
  workouts: [
    "/api/v1/workouts",
    "/api/v1/planner/templates",
    "/api/v1/planner/exercises",
    "/api/v1/planner/equipment",
  ],
  meals: [
    "/api/v1/recommendations/nutrition",
    "/api/v1/nutrition/meal-logs",
    "/api/v1/nutrition/meal-templates",
  ],
  tournaments: [
    "/api/v1/lobby/membership",
    "/api/v1/lobby/match-preview",
    "/api/v1/competition/member-stats",
  ],
  settings: ["/api/v1/profile"],
};

const shellState = {
  profile: null,
  section: DEFAULT_MEMBER_SHELL_SECTION,
  workouts: [],
  selectedWorkoutID: null,
  sectionMessage: "",
};

export function normalizeShellSection(value) {
  const normalized = String(value || "")
    .trim()
    .toLowerCase();
  return MEMBER_SHELL_SECTIONS.includes(normalized) ? normalized : DEFAULT_MEMBER_SHELL_SECTION;
}

export function memberShellPath(section) {
  return `/app/${normalizeShellSection(section)}`;
}

export function shellSectionFromPath(pathname) {
  const value = String(pathname || "").replace(/\/+$/, "");
  if (value === "/app" || value === "") {
    return DEFAULT_MEMBER_SHELL_SECTION;
  }
  const parts = value.split("/").filter(Boolean);
  return parts[0] === "app" ? normalizeShellSection(parts[1]) : DEFAULT_MEMBER_SHELL_SECTION;
}

export function currentISOWeekStart(date) {
  const current = new Date(date);
  const utcDay = current.getUTCDay();
  const diff = utcDay === 0 ? -6 : 1 - utcDay;
  current.setUTCDate(current.getUTCDate() + diff);
  current.setUTCHours(0, 0, 0, 0);
  return current.toISOString().slice(0, 10);
}

export function formatTimestamp(value) {
  if (!value) {
    return "Not recorded";
  }

  const parsed = new Date(value);
  if (Number.isNaN(parsed.valueOf())) {
    return "Invalid timestamp";
  }

  return parsed.toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  });
}

export function extractErrorMessage(payload, fallback) {
  if (payload && typeof payload.error === "string" && payload.error.trim() !== "") {
    return payload.error.trim();
  }

  return fallback;
}

export function recommendationSummary(recommendation) {
  switch (recommendation?.type) {
    case "resume_in_progress_workout":
      return {
        headline: "Resume your in-progress workout",
        detail: "APOLLO already has an in-progress workout for you, so the member shell keeps that as the next action.",
      };
    case "start_first_workout":
      return {
        headline: "Start your first workout",
        detail: "No finished workouts exist yet, so the deterministic recommendation is to start a first session.",
      };
    case "repeat_last_finished_workout":
      return {
        headline: "Repeat the last finished workout",
        detail: "The latest finished workout is outside the recovery window, so repeating it is the current deterministic read.",
      };
    case "recovery_day":
      return {
        headline: "Take a recovery day",
        detail: "Your latest finished workout is still inside the 24-hour recovery window.",
      };
    default:
      return {
        headline: "Recommendation unavailable",
        detail: "APOLLO did not return a deterministic recommendation payload.",
      };
  }
}

export function membershipSummary(membership) {
  if (membership?.status === "joined") {
    return {
      headline: "Joined lobby",
      detail: `APOLLO recorded explicit lobby membership at ${formatTimestamp(membership.joined_at)}.`,
    };
  }

  return {
    headline: "Not joined",
    detail: "Lobby membership stays explicit. Join only when you intend to be in the lobby.",
  };
}

export function workoutListLabel(workout) {
  const exerciseCount = Array.isArray(workout?.exercises) ? workout.exercises.length : 0;
  const countLabel = `${exerciseCount} ${exerciseCount === 1 ? "exercise" : "exercises"}`;
  return `${formatTimestamp(workout?.started_at)} | ${countLabel}`;
}

export function selectWorkoutID(workouts, currentSelection) {
  if (Array.isArray(workouts) && workouts.some((workout) => workout.id === currentSelection)) {
    return currentSelection;
  }
  if (Array.isArray(workouts) && workouts.length > 0) {
    return workouts[0].id;
  }
  return null;
}

export function buildWorkoutPayload(notesValue, exerciseRows) {
  const notes = normalizeOptionalString(notesValue);
  const exercises = (exerciseRows || [])
    .map((row) => ({
      name: normalizeOptionalString(row.name),
      sets: parseWholeNumber(row.sets),
      reps: parseWholeNumber(row.reps),
      weight_kg: parseOptionalDecimal(row.weightKg),
      rpe: parseOptionalDecimal(row.rpe),
      notes: normalizeOptionalString(row.notes),
    }))
    .filter((row) => row.name);

  return {
    notes,
    exercises,
  };
}

export function buildProfilePatchPayload(values) {
  const goalKey = normalizeOptionalString(values.goalKey);
  const experienceLevel = normalizeOptionalString(values.experienceLevel);
  const budgetPreference = normalizeOptionalString(values.budgetPreference);
  const cookingCapability = normalizeOptionalString(values.cookingCapability);
  const coachingDays = parseWholeNumber(values.daysPerWeek);
  const sessionMinutes = parseWholeNumber(values.sessionMinutes);

  return {
    visibility_mode: values.visibilityMode,
    availability_mode: values.availabilityMode,
    coaching_profile: {
      goal_key: goalKey ?? undefined,
      days_per_week: Number.isInteger(coachingDays) ? coachingDays : undefined,
      session_minutes: Number.isInteger(sessionMinutes) ? sessionMinutes : undefined,
      experience_level: experienceLevel ?? undefined,
      preferred_equipment_keys: parseCSV(values.preferredEquipmentKeys),
    },
    nutrition_profile: {
      dietary_restrictions: parseCSV(values.dietaryRestrictions),
      meal_preference: {
        cuisine_preferences: parseCSV(values.cuisinePreferences),
      },
      budget_preference: budgetPreference ?? undefined,
      cooking_capability: cookingCapability ?? undefined,
    },
  };
}

async function requestJSON(path, options = {}) {
  const response = await fetch(path, {
    credentials: "same-origin",
    headers: {
      ...(options.body ? { "Content-Type": "application/json" } : {}),
      ...(options.headers || {}),
    },
    ...options,
  });

  let payload = null;
  const contentType = response.headers?.get?.("content-type") || "";
  if (response.status !== 204 && contentType.includes("application/json")) {
    payload = await response.json();
  }

  return { ok: response.ok, status: response.status, payload };
}

async function boot() {
  const view = document?.body?.dataset?.apolloView;
  if (view === "login") {
    await initLoginView();
    return;
  }
  if (view === "shell") {
    await initShellView();
  }
}

async function initLoginView() {
  const form = document.querySelector("#start-verification-form");
  if (!form) {
    return;
  }

  const studentIDInput = document.querySelector("#student-id");
  const emailInput = document.querySelector("#email");
  const verifyTokenInput = document.querySelector("#verification-token");
  const requestStatus = document.querySelector("#start-verification-message");
  const verifyStatus = document.querySelector("#verify-token-message");

  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    setMessage(requestStatus, "Sending verification email...", "");

    try {
      const result = await requestJSON("/api/v1/auth/verification/start", {
        method: "POST",
        body: JSON.stringify({
          student_id: studentIDInput?.value || "",
          email: emailInput?.value || "",
        }),
      });

      if (!result.ok) {
        setMessage(requestStatus, extractErrorMessage(result.payload, "Verification request failed."), "error");
        return;
      }

      setMessage(requestStatus, "Verification email sent.", "success");
    } catch (error) {
      setMessage(requestStatus, error instanceof Error ? error.message : "Verification request failed.", "error");
    }
  });

  const verifyForm = document.querySelector("#verify-token-form");
  verifyForm?.addEventListener("submit", async (event) => {
    event.preventDefault();
    setMessage(verifyStatus, "Verifying token...", "");

    try {
      const token = verifyTokenInput?.value || "";
      const result = await requestJSON("/api/v1/auth/verify", {
        method: "POST",
        body: JSON.stringify({ token }),
      });
      if (!result.ok) {
        setMessage(verifyStatus, extractErrorMessage(result.payload, "Verification failed."), "error");
        return;
      }

      window.location.assign(memberShellPath(DEFAULT_MEMBER_SHELL_SECTION));
    } catch (error) {
      setMessage(verifyStatus, error instanceof Error ? error.message : "Verification failed.", "error");
    }
  });
}

async function initShellView() {
  shellState.section = normalizeShellSection(document.body.dataset.apolloSection || shellSectionFromPath(window.location.pathname));
  highlightNav(shellState.section);
  applySectionMeta(shellState.section);

  const refreshButton = document.querySelector("#refresh-shell");
  const logoutButton = document.querySelector("#logout-shell");

  refreshButton?.addEventListener("click", async () => {
    await bootShell();
  });

  logoutButton?.addEventListener("click", async () => {
    logoutButton.disabled = true;
    try {
      await requestJSON("/api/v1/auth/logout", { method: "POST" });
    } finally {
      window.location.assign("/app/login");
    }
  });

  await bootShell();
}

async function bootShell() {
  applySectionMeta(shellState.section);
  renderSectionLoading("Loading member shell...");
  setMessage(document.querySelector("#shell-status"), "Loading member shell...", "");

  try {
    const profileResponse = await requestJSON("/api/v1/profile");
    if (profileResponse.status === 401) {
      window.location.assign("/app/login");
      return;
    }
    if (!profileResponse.ok) {
      renderShellBootstrapFailure(extractErrorMessage(profileResponse.payload, "Member shell bootstrap failed."));
      return;
    }

    shellState.profile = profileResponse.payload;
    setMessage(document.querySelector("#shell-status"), shellState.sectionMessage || "Member shell ready.", "success");
    await loadSection(shellState.section);
  } catch (error) {
    renderShellBootstrapFailure(error instanceof Error ? error.message : "Member shell bootstrap failed.");
  }
}

async function loadSection(section) {
  shellState.section = normalizeShellSection(section);
  highlightNav(shellState.section);
  applySectionMeta(shellState.section);
  renderSectionLoading(`Loading ${SECTION_META[shellState.section].eyebrow.toLowerCase()}...`);

  try {
    switch (shellState.section) {
      case "home":
        await loadHomeSection();
        break;
      case "workouts":
        await loadWorkoutsSection();
        break;
      case "meals":
        await loadMealsSection();
        break;
      case "tournaments":
        await loadTournamentsSection();
        break;
      case "settings":
        renderSettingsSection(shellState.profile);
        bindSettingsSection();
        break;
      default:
        window.location.assign(memberShellPath(DEFAULT_MEMBER_SHELL_SECTION));
    }
  } catch (error) {
    renderSectionError(error instanceof Error ? error.message : "Section load failed.");
  }
}

async function loadSectionPayloads(paths) {
  const responses = await Promise.all(paths.map((path) => requestJSON(path)));
  for (const response of responses) {
    if (response.status === 401) {
      window.location.assign("/app/login");
      throw new Error("Unauthorized");
    }
    if (!response.ok) {
      throw new Error(extractErrorMessage(response.payload, "Section load failed."));
    }
  }
  return responses.map((response) => response.payload);
}

async function loadHomeSection() {
  const [presence, eligibility, membership, recommendation] = await loadSectionPayloads(SECTION_API_PATHS.home);
  const recommendationCard = recommendationSummary(recommendation);
  const membershipCard = membershipSummary(membership);
  const facilities = Array.isArray(presence?.facilities) ? presence.facilities : [];
  const currentFacility = facilities.find((facility) => facility.status === "present");
  const visibleName = shellState.profile?.display_name || shellState.profile?.student_id || "Member";

  renderCards([
    {
      title: "Profile summary",
      body: [
        metricRow("Member", visibleName),
        metricRow("Visibility", shellState.profile?.visibility_mode || "unknown"),
        metricRow("Availability", shellState.profile?.availability_mode || "unknown"),
        metricRow("Email", shellState.profile?.email || "unknown"),
      ].join(""),
    },
    {
      title: "Presence summary",
      body: facilities.length
        ? [
            metricRow("Facilities tracked", String(facilities.length)),
            metricRow("Current status", currentFacility ? `Present at ${currentFacility.facility_key}` : "Not currently present"),
            metricRow("Latest streak", facilities[0]?.streak?.status || "not_started"),
          ].join("")
        : emptyStateMarkup("No linked facility visits exist yet."),
    },
    {
      title: "Lobby eligibility",
      body: [
        metricRow("Eligible", eligibility?.eligible ? "yes" : "no"),
        metricRow("Reason", eligibility?.reason || "unknown"),
      ].join(""),
    },
    {
      title: membershipCard.headline,
      body: `<p>${escapeHTML(membershipCard.detail)}</p>`,
    },
    {
      title: recommendationCard.headline,
      body: `<p>${escapeHTML(recommendationCard.detail)}</p>${recommendation?.generated_at ? `<p class="meta-copy">Generated ${escapeHTML(formatTimestamp(recommendation.generated_at))}</p>` : ""}`,
    },
    {
      title: "Schedule boundary",
      body: `<p>No member-safe schedule read exists in APOLLO yet, so schedule and booking stay out of this shell.</p>`,
    },
  ]);
}

async function loadWorkoutsSection() {
  const [workouts, templates, exercises, equipment] = await loadSectionPayloads(SECTION_API_PATHS.workouts);
  shellState.workouts = Array.isArray(workouts) ? workouts : [];
  shellState.selectedWorkoutID = selectWorkoutID(shellState.workouts, shellState.selectedWorkoutID);
  const selectedWorkout = shellState.workouts.find((workout) => workout.id === shellState.selectedWorkoutID) || null;

  renderWorkoutSection({
    workouts: shellState.workouts,
    selectedWorkout,
    templates: Array.isArray(templates) ? templates : [],
    exercises: Array.isArray(exercises) ? exercises : [],
    equipment: Array.isArray(equipment) ? equipment : [],
  });
  bindWorkoutSection();
}

async function loadMealsSection() {
  const [recommendation, mealLogs, mealTemplates] = await loadSectionPayloads(SECTION_API_PATHS.meals);
  const logs = Array.isArray(mealLogs) ? mealLogs : [];
  const templates = Array.isArray(mealTemplates) ? mealTemplates : [];

  renderCards([
    {
      title: "Nutrition recommendation",
      body: [
        metricRow("Kind", recommendation?.kind || "unknown"),
        metricRow("Goal", recommendation?.goal_key || "unknown"),
        metricRow("Calories", recommendation?.daily_calories ? `${recommendation.daily_calories.min}-${recommendation.daily_calories.max}` : "unknown"),
        metricRow("Policy", recommendation?.policy_version || "unknown"),
      ].join(""),
    },
    {
      title: "Meal logs",
      body: logs.length ? renderSimpleList(logs.slice(0, 6).map((entry) => `${entry.name} | ${entry.meal_type} | ${formatTimestamp(entry.logged_at)}`)) : emptyStateMarkup("No meal logs recorded yet."),
    },
    {
      title: "Meal templates",
      body: templates.length
        ? renderSimpleList(templates.slice(0, 6).map((entry) => `${entry.name} | ${entry.meal_type}`))
        : emptyStateMarkup("No meal templates recorded yet."),
    },
  ]);
}

async function loadTournamentsSection() {
  const [membership, preview, stats] = await loadSectionPayloads(SECTION_API_PATHS.tournaments);
  const membershipCard = membershipSummary(membership);
  const memberStats = Array.isArray(stats) ? stats : [];

  const root = document.querySelector("#section-shell");
  root.innerHTML = `
    <div class="section-grid">
      <article class="truth-card">
        <h3>${escapeHTML(membershipCard.headline)}</h3>
        <p>${escapeHTML(membershipCard.detail)}</p>
        <div class="action-row">
          <button id="join-lobby" class="primary-button" type="button"${membership?.status === "joined" ? " hidden" : ""}>Join lobby</button>
          <button id="leave-lobby" class="secondary-button" type="button"${membership?.status === "joined" ? "" : " hidden"}>Leave lobby</button>
        </div>
        <p id="membership-status" class="status-message" aria-live="polite"></p>
      </article>
      <article class="truth-card">
        <h3>Match preview</h3>
        ${renderMatchPreview(preview)}
      </article>
      <article class="truth-card">
        <h3>Competition member stats</h3>
        ${memberStats.length ? renderSimpleList(memberStats.map((entry) => `${entry.sport_key}/${entry.mode_key} | ${entry.matches_played} played | ${entry.wins}W-${entry.losses}L-${entry.draws}D`)) : emptyStateMarkup("No self-scoped competition stats exist yet.")}
      </article>
    </div>
  `;

  bindTournamentsSection();
}

function renderSettingsSection(profile) {
  const root = document.querySelector("#section-shell");
  const coaching = profile?.coaching_profile || {};
  const nutrition = profile?.nutrition_profile || {};

  root.innerHTML = `
    <form id="settings-form" class="settings-form">
      <div class="section-grid">
        <article class="truth-card">
          <h3>Identity and visibility</h3>
          <label>
            <span>Visibility</span>
            <select id="settings-visibility">
              ${renderSelectOptions(["ghost", "discoverable"], profile?.visibility_mode)}
            </select>
          </label>
          <label>
            <span>Availability</span>
            <select id="settings-availability">
              ${renderSelectOptions(["unavailable", "available_now", "with_team"], profile?.availability_mode)}
            </select>
          </label>
        </article>
        <article class="truth-card">
          <h3>Coaching profile</h3>
          <label><span>Goal key</span><input id="settings-goal-key" type="text" value="${escapeHTML(coaching.goal_key || "")}" /></label>
          <label><span>Days per week</span><input id="settings-days-per-week" type="number" min="1" max="7" value="${escapeHTML(numberValue(coaching.days_per_week))}" /></label>
          <label><span>Session minutes</span><input id="settings-session-minutes" type="number" min="1" value="${escapeHTML(numberValue(coaching.session_minutes))}" /></label>
          <label>
            <span>Experience level</span>
            <select id="settings-experience-level">
              ${renderSelectOptions(["", "beginner", "intermediate", "advanced"], coaching.experience_level || "")}
            </select>
          </label>
          <label><span>Preferred equipment keys</span><input id="settings-equipment-keys" type="text" value="${escapeHTML((coaching.preferred_equipment_keys || []).join(", "))}" /></label>
        </article>
        <article class="truth-card">
          <h3>Nutrition profile</h3>
          <label><span>Dietary restrictions</span><input id="settings-dietary-restrictions" type="text" value="${escapeHTML((nutrition.dietary_restrictions || []).join(", "))}" /></label>
          <label><span>Cuisine preferences</span><input id="settings-cuisine-preferences" type="text" value="${escapeHTML((nutrition.meal_preference?.cuisine_preferences || []).join(", "))}" /></label>
          <label>
            <span>Budget preference</span>
            <select id="settings-budget-preference">
              ${renderSelectOptions(["", "budget_constrained", "moderate", "flexible"], nutrition.budget_preference || "")}
            </select>
          </label>
          <label>
            <span>Cooking capability</span>
            <select id="settings-cooking-capability">
              ${renderSelectOptions(["", "no_kitchen", "microwave_only", "basic_kitchen", "full_kitchen"], nutrition.cooking_capability || "")}
            </select>
          </label>
        </article>
      </div>
      <div class="action-row">
        <button id="save-settings" class="primary-button" type="submit">Save settings</button>
      </div>
      <p id="settings-status" class="status-message" aria-live="polite"></p>
    </form>
  `;
}

function bindSettingsSection() {
  const form = document.querySelector("#settings-form");
  const status = document.querySelector("#settings-status");
  form?.addEventListener("submit", async (event) => {
    event.preventDefault();
    setMessage(status, "Saving settings...", "");

    try {
      const payload = buildProfilePatchPayload({
        visibilityMode: document.querySelector("#settings-visibility")?.value || "ghost",
        availabilityMode: document.querySelector("#settings-availability")?.value || "unavailable",
        goalKey: document.querySelector("#settings-goal-key")?.value || "",
        daysPerWeek: document.querySelector("#settings-days-per-week")?.value || "",
        sessionMinutes: document.querySelector("#settings-session-minutes")?.value || "",
        experienceLevel: document.querySelector("#settings-experience-level")?.value || "",
        preferredEquipmentKeys: document.querySelector("#settings-equipment-keys")?.value || "",
        dietaryRestrictions: document.querySelector("#settings-dietary-restrictions")?.value || "",
        cuisinePreferences: document.querySelector("#settings-cuisine-preferences")?.value || "",
        budgetPreference: document.querySelector("#settings-budget-preference")?.value || "",
        cookingCapability: document.querySelector("#settings-cooking-capability")?.value || "",
      });

      const result = await requestJSON("/api/v1/profile", {
        method: "PATCH",
        body: JSON.stringify(payload),
      });
      if (result.status === 401) {
        window.location.assign("/app/login");
        return;
      }
      if (!result.ok) {
        setMessage(status, extractErrorMessage(result.payload, "Settings update failed."), "error");
        return;
      }

      shellState.profile = result.payload;
      shellState.sectionMessage = "Settings saved.";
      setMessage(status, "Settings saved.", "success");
      setMessage(document.querySelector("#shell-status"), "Settings saved.", "success");
    } catch (error) {
      setMessage(status, error instanceof Error ? error.message : "Settings update failed.", "error");
    }
  });
}

function renderWorkoutSection({ workouts, selectedWorkout, templates, exercises, equipment }) {
  const root = document.querySelector("#section-shell");
  root.innerHTML = `
    <div class="section-grid section-grid-wide">
      <article class="truth-card">
        <h3>Planner substrate</h3>
        <div class="metrics">
          ${metricRow("Templates", String(templates.length))}
          ${metricRow("Exercises", String(exercises.length))}
          ${metricRow("Equipment", String(equipment.length))}
        </div>
      </article>
      <article class="truth-card truth-card-span">
        <div class="card-header">
          <div>
            <h3>Workout list</h3>
            <p class="meta-copy">Start, select, update, and finish workouts over the existing member runtime.</p>
          </div>
          <button id="create-workout" class="primary-button" type="button">Start workout</button>
        </div>
        <div class="workout-layout">
          <ol id="workout-list" class="workout-list">
            ${workouts.length ? workouts.map((workout) => renderWorkoutListItem(workout, selectedWorkout?.id)).join("") : `<li class="empty-list">${emptyStateMarkup("No workouts exist yet.")}</li>`}
          </ol>
          <section class="workout-detail">
            <div class="card-header">
              <div>
                <h4 id="workout-detail-title">${selectedWorkout ? `Workout ${escapeHTML(selectedWorkout.id.slice(0, 8))}` : "Select a workout"}</h4>
                <p id="workout-detail-state" class="meta-copy">${selectedWorkout ? escapeHTML(selectedWorkout.status) : "No workout selected"}</p>
              </div>
            </div>
            <form id="workout-editor" class="editor-shell">
              <label>
                <span>Notes</span>
                <textarea id="workout-notes" rows="4" placeholder="Workout notes"${selectedWorkout ? "" : " disabled"}>${escapeHTML(selectedWorkout?.notes || "")}</textarea>
              </label>
              <section class="exercise-section">
                <div class="card-header">
                  <div>
                    <h4>Exercise rows</h4>
                    <p class="meta-copy">These writes stay backend-authoritative.</p>
                  </div>
                  <button id="add-exercise" class="secondary-button" type="button"${selectedWorkout ? "" : " disabled"}>Add exercise</button>
                </div>
                <div id="exercise-list" class="exercise-list">
                  ${selectedWorkout ? renderExerciseRows(selectedWorkout.exercises) : emptyStateMarkup("Start or select a workout to edit draft rows.")}
                </div>
              </section>
              <div class="action-row">
                <button id="save-workout" class="primary-button" type="submit"${selectedWorkout ? "" : " disabled"}>Save draft</button>
                <button id="finish-workout" class="secondary-button" type="button"${selectedWorkout ? "" : " disabled"}>Finish workout</button>
              </div>
              <p id="workouts-status" class="status-message" aria-live="polite"></p>
            </form>
          </section>
        </div>
      </article>
    </div>
  `;
}

function bindWorkoutSection() {
  const createButton = document.querySelector("#create-workout");
  const list = document.querySelector("#workout-list");
  const editor = document.querySelector("#workout-editor");
  const addExerciseButton = document.querySelector("#add-exercise");
  const finishButton = document.querySelector("#finish-workout");
  const status = document.querySelector("#workouts-status");

  createButton?.addEventListener("click", async () => {
    setMessage(status, "Starting workout...", "");
    try {
      const result = await requestJSON("/api/v1/workouts", {
        method: "POST",
        body: JSON.stringify({ notes: null }),
      });
      if (!result.ok) {
        setMessage(status, extractErrorMessage(result.payload, "Workout create failed."), "error");
        return;
      }

      shellState.selectedWorkoutID = result.payload.id;
      shellState.sectionMessage = "Workout created.";
      await loadWorkoutsSection();
      setMessage(document.querySelector("#shell-status"), "Workout created.", "success");
    } catch (error) {
      setMessage(status, error instanceof Error ? error.message : "Workout create failed.", "error");
    }
  });

  list?.addEventListener("click", async (event) => {
    const button = event.target instanceof HTMLElement ? event.target.closest("[data-workout-id]") : null;
    if (!button) {
      return;
    }
    shellState.selectedWorkoutID = button.getAttribute("data-workout-id");
    await loadWorkoutsSection();
  });

  addExerciseButton?.addEventListener("click", () => {
    const exerciseList = document.querySelector("#exercise-list");
    if (!exerciseList) {
      return;
    }
    exerciseList.insertAdjacentHTML("beforeend", renderExerciseRows([{}]));
  });

  editor?.addEventListener("submit", async (event) => {
    event.preventDefault();
    if (!shellState.selectedWorkoutID) {
      return;
    }

    setMessage(status, "Saving workout...", "");
    try {
      const payload = buildWorkoutPayload(
        document.querySelector("#workout-notes")?.value || "",
        readExerciseRows(),
      );

      const result = await requestJSON(`/api/v1/workouts/${shellState.selectedWorkoutID}`, {
        method: "PUT",
        body: JSON.stringify(payload),
      });
      if (!result.ok) {
        setMessage(status, extractErrorMessage(result.payload, "Workout save failed."), "error");
        return;
      }

      shellState.selectedWorkoutID = result.payload.id;
      shellState.sectionMessage = "Workout saved.";
      await loadWorkoutsSection();
      setMessage(document.querySelector("#shell-status"), "Workout saved.", "success");
    } catch (error) {
      setMessage(status, error instanceof Error ? error.message : "Workout save failed.", "error");
    }
  });

  finishButton?.addEventListener("click", async () => {
    if (!shellState.selectedWorkoutID) {
      return;
    }

    setMessage(status, "Finishing workout...", "");
    try {
      const result = await requestJSON(`/api/v1/workouts/${shellState.selectedWorkoutID}/finish`, {
        method: "POST",
      });
      if (!result.ok) {
        setMessage(status, extractErrorMessage(result.payload, "Workout finish failed."), "error");
        return;
      }

      shellState.selectedWorkoutID = result.payload.id;
      shellState.sectionMessage = "Workout finished.";
      await loadWorkoutsSection();
      setMessage(document.querySelector("#shell-status"), "Workout finished.", "success");
    } catch (error) {
      setMessage(status, error instanceof Error ? error.message : "Workout finish failed.", "error");
    }
  });
}

function bindTournamentsSection() {
  const status = document.querySelector("#membership-status");
  const joinButton = document.querySelector("#join-lobby");
  const leaveButton = document.querySelector("#leave-lobby");

  joinButton?.addEventListener("click", async () => {
    setMessage(status, "Joining lobby...", "");
    try {
      const result = await requestJSON("/api/v1/lobby/membership/join", { method: "POST" });
      if (!result.ok) {
        setMessage(status, extractErrorMessage(result.payload, "Lobby join failed."), "error");
        return;
      }
      shellState.sectionMessage = "Lobby membership joined.";
      setMessage(document.querySelector("#shell-status"), "Lobby membership joined.", "success");
      await loadTournamentsSection();
      setMessage(document.querySelector("#membership-status"), "Lobby membership joined.", "success");
    } catch (error) {
      setMessage(status, error instanceof Error ? error.message : "Lobby join failed.", "error");
    }
  });

  leaveButton?.addEventListener("click", async () => {
    setMessage(status, "Leaving lobby...", "");
    try {
      const result = await requestJSON("/api/v1/lobby/membership/leave", { method: "POST" });
      if (!result.ok) {
        setMessage(status, extractErrorMessage(result.payload, "Lobby leave failed."), "error");
        return;
      }
      shellState.sectionMessage = "Lobby membership left.";
      setMessage(document.querySelector("#shell-status"), "Lobby membership left.", "success");
      await loadTournamentsSection();
      setMessage(document.querySelector("#membership-status"), "Lobby membership left.", "success");
    } catch (error) {
      setMessage(status, error instanceof Error ? error.message : "Lobby leave failed.", "error");
    }
  });
}

function renderCards(cards) {
  const root = document.querySelector("#section-shell");
  root.innerHTML = `<div class="section-grid">${cards
    .map(
      (card) => `
        <article class="truth-card">
          <h3>${escapeHTML(card.title)}</h3>
          ${card.body}
        </article>`,
    )
    .join("")}</div>`;
}

function renderShellBootstrapFailure(message) {
  setMessage(document.querySelector("#shell-status"), message, "error");
  const root = document.querySelector("#section-shell");
  root.innerHTML = `
    <article class="truth-card">
      <h3>Member shell bootstrap failed</h3>
      <p>${escapeHTML(message)}</p>
      <button id="section-retry" class="primary-button" type="button">Retry</button>
    </article>
  `;
  bindRetry();
}

function renderSectionLoading(message) {
  const root = document.querySelector("#section-shell");
  root.innerHTML = `
    <article class="truth-card">
      <h3>Loading</h3>
      <p>${escapeHTML(message)}</p>
    </article>
  `;
}

function renderSectionError(message) {
  const root = document.querySelector("#section-shell");
  root.innerHTML = `
    <article class="truth-card">
      <h3>Section load failed</h3>
      <p>${escapeHTML(message)}</p>
      <button id="section-retry" class="primary-button" type="button">Retry section</button>
    </article>
  `;
  bindRetry();
}

function bindRetry() {
  document.querySelector("#section-retry")?.addEventListener("click", async () => {
    await bootShell();
  });
}

function applySectionMeta(section) {
  const meta = SECTION_META[normalizeShellSection(section)];
  if (!meta) {
    return;
  }

  const eyebrow = document.querySelector("#section-eyebrow");
  const title = document.querySelector("#section-title");
  const copy = document.querySelector("#section-copy");
  if (eyebrow) {
    eyebrow.textContent = meta.eyebrow;
  }
  if (title) {
    title.textContent = meta.title;
  }
  if (copy) {
    copy.textContent = meta.copy;
  }
}

function highlightNav(section) {
  for (const link of document.querySelectorAll(".member-nav-link")) {
    const active = link.dataset.navSection === section;
    link.setAttribute("aria-current", active ? "page" : "false");
    link.classList.toggle("active", active);
  }
}

function renderMatchPreview(preview) {
  if (!preview || preview.candidate_count === 0) {
    return `<p>No eligible joined members are available for a deterministic match preview.</p>`;
  }

  const matches = Array.isArray(preview.matches) ? preview.matches : [];
  const unmatched = Array.isArray(preview.unmatched_labels) ? preview.unmatched_labels : [];
  return `
    <p>${escapeHTML(String(preview.candidate_count))} candidate${preview.candidate_count === 1 ? "" : "s"} in the current deterministic preview.</p>
    ${matches.length ? renderSimpleList(matches.map((match) => match.member_labels.join(" vs "))) : ""}
    ${unmatched.length ? `<p class="meta-copy">Unmatched: ${escapeHTML(unmatched.join(", "))}</p>` : ""}
  `;
}

function renderWorkoutListItem(workout, selectedWorkoutID) {
  const selected = workout.id === selectedWorkoutID;
  return `
    <li>
      <button class="list-button${selected ? " active" : ""}" type="button" data-workout-id="${escapeHTML(workout.id)}">
        <strong>${escapeHTML(workoutListLabel(workout))}</strong>
        <span>${escapeHTML(workout.status)}</span>
      </button>
    </li>
  `;
}

function renderExerciseRows(exercises) {
  const rows = Array.isArray(exercises) && exercises.length ? exercises : [{}];
  return rows
    .map(
      (exercise) => `
        <div class="exercise-row">
          <input data-exercise-field="name" type="text" placeholder="Exercise name" value="${escapeHTML(exercise.name || "")}" />
          <input data-exercise-field="sets" type="number" min="1" placeholder="Sets" value="${escapeHTML(numberValue(exercise.sets))}" />
          <input data-exercise-field="reps" type="number" min="1" placeholder="Reps" value="${escapeHTML(numberValue(exercise.reps))}" />
          <input data-exercise-field="weightKg" type="number" min="0" step="0.1" placeholder="Weight kg" value="${escapeHTML(numberValue(exercise.weight_kg))}" />
          <input data-exercise-field="rpe" type="number" min="0" max="10" step="0.1" placeholder="RPE" value="${escapeHTML(numberValue(exercise.rpe))}" />
          <input data-exercise-field="notes" type="text" placeholder="Notes" value="${escapeHTML(exercise.notes || "")}" />
        </div>
      `,
    )
    .join("");
}

function readExerciseRows() {
  return Array.from(document.querySelectorAll(".exercise-row")).map((row) => ({
    name: row.querySelector('[data-exercise-field="name"]')?.value || "",
    sets: row.querySelector('[data-exercise-field="sets"]')?.value || "",
    reps: row.querySelector('[data-exercise-field="reps"]')?.value || "",
    weightKg: row.querySelector('[data-exercise-field="weightKg"]')?.value || "",
    rpe: row.querySelector('[data-exercise-field="rpe"]')?.value || "",
    notes: row.querySelector('[data-exercise-field="notes"]')?.value || "",
  }));
}

function renderSimpleList(entries) {
  return `<ul class="plain-list">${entries.map((entry) => `<li>${escapeHTML(entry)}</li>`).join("")}</ul>`;
}

function renderSelectOptions(options, selectedValue) {
  return options
    .map((value) => {
      const label = value === "" ? "Unset" : value;
      return `<option value="${escapeHTML(value)}"${value === selectedValue ? " selected" : ""}>${escapeHTML(label)}</option>`;
    })
    .join("");
}

function emptyStateMarkup(message) {
  return `<p class="meta-copy">${escapeHTML(message)}</p>`;
}

function metricRow(label, value) {
  return `
    <div class="metric-row">
      <span>${escapeHTML(label)}</span>
      <strong>${escapeHTML(value)}</strong>
    </div>
  `;
}

function setMessage(node, message, tone) {
  if (!node) {
    return;
  }
  node.textContent = message;
  node.classList.remove("error-message", "success-message");
  if (tone === "error") {
    node.classList.add("error-message");
  }
  if (tone === "success") {
    node.classList.add("success-message");
  }
}

function normalizeOptionalString(value) {
  const normalized = String(value ?? "").trim();
  return normalized === "" ? null : normalized;
}

function parseOptionalDecimal(value) {
  const normalized = normalizeOptionalString(value);
  if (normalized == null) {
    return null;
  }

  const parsed = Number(normalized);
  return Number.isFinite(parsed) ? parsed : null;
}

function parseWholeNumber(value) {
  const normalized = normalizeOptionalString(value);
  if (normalized == null) {
    return null;
  }

  const parsed = Number.parseInt(normalized, 10);
  return Number.isInteger(parsed) ? parsed : null;
}

function parseCSV(value) {
  return String(value || "")
    .split(",")
    .map((entry) => entry.trim())
    .filter(Boolean);
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function numberValue(value) {
  return value == null ? "" : String(value);
}

if (typeof document !== "undefined" && document?.body) {
  boot().catch((error) => {
    if (document.body.dataset.apolloView === "shell") {
      renderShellBootstrapFailure(error instanceof Error ? error.message : "Member shell bootstrap failed.");
      return;
    }

    const loginStatus = document.querySelector("#login-status");
    setMessage(loginStatus, error instanceof Error ? error.message : "Bootstrap failed.", "error");
  });
}
