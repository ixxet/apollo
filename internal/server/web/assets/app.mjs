const recommendationHeadlines = {
  resume_in_progress_workout: "Resume your in-progress workout",
  start_first_workout: "Start your first workout",
  recovery_day: "Take a recovery day",
  repeat_last_finished_workout: "Repeat your last finished workout",
};

const recommendationReasonCopy = {
  in_progress_workout_exists: "APOLLO found one in-progress workout that still belongs to you.",
  no_finished_workouts: "APOLLO found no finished workouts for this member yet.",
  last_finished_within_recovery_window: "Your latest finished workout is still inside the 24-hour recovery window.",
  last_finished_outside_recovery_window: "Your latest finished workout is outside the 24-hour recovery window.",
};

const shellLoadFailureMessages = {
  profile: "Unable to load profile. Check your connection and refresh.",
  recommendation: "Unable to load recommendation. Check your connection and refresh.",
  workouts: "Unable to load workouts. Check your connection and refresh.",
};

export function formatTimestamp(value) {
  if (!value) {
    return "Unknown time";
  }

  return new Date(value).toLocaleString("en-CA", {
    dateStyle: "medium",
    timeStyle: "short",
  });
}

export function workoutListLabel(workout) {
  const exerciseCount = Array.isArray(workout.exercises) ? workout.exercises.length : 0;
  const suffix = exerciseCount === 1 ? "1 exercise" : `${exerciseCount} exercises`;
  return `${formatTimestamp(workout.started_at)} · ${suffix}`;
}

export function selectWorkoutID(workouts, currentID) {
  if (!Array.isArray(workouts) || workouts.length === 0) {
    return null;
  }

  const existing = workouts.find((workout) => workout.id === currentID);
  if (existing) {
    return existing.id;
  }

  return workouts[0].id;
}

export function buildWorkoutPayload(notesValue, exerciseRows) {
  return {
    notes: normalizeOptionalText(notesValue),
    exercises: exerciseRows.map((row) => ({
      name: row.name.trim(),
      sets: Number.parseInt(row.sets, 10),
      reps: Number.parseInt(row.reps, 10),
      weight_kg: parseOptionalNumber(row.weightKg),
      rpe: parseOptionalNumber(row.rpe),
      notes: normalizeOptionalText(row.notes),
    })),
  };
}

export function recommendationSummary(recommendation) {
  return {
    headline: recommendationHeadlines[recommendation.type] ?? "Recommendation",
    detail: recommendationReasonCopy[recommendation.reason] ?? "APOLLO returned a deterministic recommendation.",
  };
}

export function extractErrorMessage(payload, fallback) {
  if (payload && typeof payload === "object" && typeof payload.error === "string" && payload.error.length > 0) {
    return payload.error;
  }

  return fallback;
}

function normalizeOptionalText(value) {
  const normalized = String(value ?? "").trim();
  return normalized === "" ? null : normalized;
}

function parseOptionalNumber(value) {
  const normalized = String(value ?? "").trim();
  if (normalized === "") {
    return null;
  }

  return Number.parseFloat(normalized);
}

async function requestJSON(path, options = {}) {
  const response = await fetch(path, {
    headers: {
      "Content-Type": "application/json",
      ...(options.headers ?? {}),
    },
    ...options,
  });

  if (response.status === 204) {
    return { ok: true, payload: null, status: response.status };
  }

  let payload = null;
  try {
    payload = await response.json();
  } catch {
    payload = null;
  }

  return {
    ok: response.ok,
    payload,
    status: response.status,
  };
}

function boot() {
  if (typeof document === "undefined") {
    return;
  }

  const view = document.body.dataset.apolloView;
  if (view === "login") {
    void initLoginView();
  }
  if (view === "shell") {
    void initShellView();
  }
}

async function initLoginView() {
  const startForm = document.querySelector("#start-verification-form");
  const verifyForm = document.querySelector("#verify-token-form");
  const startMessage = document.querySelector("#start-verification-message");
  const verifyMessage = document.querySelector("#verify-token-message");
  const tokenInput = document.querySelector("#verification-token");
  const queryToken = new URL(window.location.href).searchParams.get("token");
  if (queryToken) {
    tokenInput.value = queryToken;
  }

  startForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    setStatus(startMessage, "Starting verification…");

    const payload = {
      student_id: document.querySelector("#student-id").value.trim(),
      email: document.querySelector("#email").value.trim(),
    };

    const response = await requestJSON("/api/v1/auth/verification/start", {
      method: "POST",
      body: JSON.stringify(payload),
    });
    if (!response.ok) {
      setStatus(startMessage, extractErrorMessage(response.payload, "Failed to start verification."), "error");
      return;
    }

    setStatus(startMessage, "Verification started. Use the token from the APOLLO verification log or link.", "success");
  });

  verifyForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    setStatus(verifyMessage, "Verifying token…");

    const response = await requestJSON("/api/v1/auth/verify", {
      method: "POST",
      body: JSON.stringify({ token: tokenInput.value.trim() }),
    });
    if (!response.ok) {
      setStatus(verifyMessage, extractErrorMessage(response.payload, "Failed to verify token."), "error");
      return;
    }

    setStatus(verifyMessage, "Verified. Loading member shell…", "success");
    window.location.assign("/app");
  });
}

async function initShellView() {
  const state = {
    workouts: [],
    selectedWorkoutID: null,
    selectedWorkout: null,
  };

  const profileSummary = document.querySelector("#profile-summary");
  const profileStatus = document.querySelector("#profile-status");
  const recommendationCard = document.querySelector("#recommendation-card");
  const recommendationStatus = document.querySelector("#recommendation-status");
  const workoutsList = document.querySelector("#workout-list");
  const workoutsStatus = document.querySelector("#workouts-status");
  const workoutTitle = document.querySelector("#workout-detail-title");
  const workoutState = document.querySelector("#workout-detail-state");
  const workoutNotes = document.querySelector("#workout-notes");
  const exerciseList = document.querySelector("#exercise-list");
  const workoutError = document.querySelector("#workout-error");
  const saveWorkoutButton = document.querySelector("#save-workout");
  const finishWorkoutButton = document.querySelector("#finish-workout");

  document.querySelector("#refresh-shell").addEventListener("click", () => {
    void guardedRefreshShell();
  });
  document.querySelector("#logout-shell").addEventListener("click", async () => {
    await requestJSON("/api/v1/auth/logout", { method: "POST", body: "{}" });
    window.location.assign("/app/login");
  });
  document.querySelector("#create-workout").addEventListener("click", async () => {
    clearWorkoutError();
    setStatus(workoutsStatus, "Starting workout…");
    const response = await requestJSON("/api/v1/workouts", {
      method: "POST",
      body: JSON.stringify({}),
    });
    if (!response.ok) {
      setStatus(workoutsStatus, extractErrorMessage(response.payload, "Failed to start workout."), "error");
      return;
    }

    setStatus(workoutsStatus, "Workout started.", "success");
    await guardedRefreshShell(response.payload.id);
  });
  document.querySelector("#add-exercise").addEventListener("click", () => {
    renderExerciseRows([...(state.selectedWorkout?.exercises ?? []), blankExercise()]);
  });
  document.querySelector("#workout-editor").addEventListener("submit", async (event) => {
    event.preventDefault();
    await saveWorkout();
  });
  finishWorkoutButton.addEventListener("click", async () => {
    if (!state.selectedWorkoutID) {
      return;
    }
    clearWorkoutError();
    toggleWorkoutActions(true);
    const response = await requestJSON(`/api/v1/workouts/${state.selectedWorkoutID}/finish`, {
      method: "POST",
      body: JSON.stringify({}),
    });
    if (!response.ok) {
      setStatus(workoutError, extractErrorMessage(response.payload, "Failed to finish workout."), "error");
      toggleWorkoutActions(false);
      return;
    }

    setStatus(workoutError, "Workout finished.", "success");
    await guardedRefreshShell(state.selectedWorkoutID);
  });

  workoutsList.addEventListener("click", (event) => {
    const button = event.target.closest("button[data-workout-id]");
    if (!button) {
      return;
    }

    void loadWorkoutDetail(button.dataset.workoutId);
  });

  await guardedRefreshShell();

  async function guardedRefreshShell(preferredWorkoutID = state.selectedWorkoutID) {
    try {
      await refreshShell(preferredWorkoutID);
    } catch {
      renderShellLoadFailure();
    }
  }

  async function refreshShell(preferredWorkoutID = state.selectedWorkoutID) {
    clearWorkoutError();
    setStatus(profileStatus, "Loading profile…");
    setStatus(recommendationStatus, "Loading recommendation…");
    setStatus(workoutsStatus, "Loading workouts…");

    const [profileResponse, workoutsResponse, recommendationResponse] = await Promise.all([
      requestJSON("/api/v1/profile"),
      requestJSON("/api/v1/workouts"),
      requestJSON("/api/v1/recommendations/workout"),
    ]);

    if (profileResponse.status === 401 || workoutsResponse.status === 401 || recommendationResponse.status === 401) {
      window.location.assign("/app/login");
      return;
    }

    if (profileResponse.ok) {
      renderProfile(profileResponse.payload);
      setStatus(profileStatus, "Profile loaded.", "success");
    } else {
      profileSummary.innerHTML = "";
      setStatus(profileStatus, extractErrorMessage(profileResponse.payload, "Failed to load profile."), "error");
    }

    if (recommendationResponse.ok) {
      renderRecommendation(recommendationResponse.payload);
      setStatus(recommendationStatus, "Recommendation loaded.", "success");
    } else {
      recommendationCard.innerHTML = `<p class="empty-state">${escapeHTML(extractErrorMessage(recommendationResponse.payload, "Failed to load recommendation."))}</p>`;
      setStatus(recommendationStatus, extractErrorMessage(recommendationResponse.payload, "Failed to load recommendation."), "error");
    }

    if (workoutsResponse.ok) {
      state.workouts = workoutsResponse.payload;
      state.selectedWorkoutID = selectWorkoutID(state.workouts, preferredWorkoutID);
      renderWorkoutsList(state.workouts, state.selectedWorkoutID);
      setStatus(workoutsStatus, `${state.workouts.length} workouts loaded.`, "success");
      if (state.selectedWorkoutID) {
        await loadWorkoutDetail(state.selectedWorkoutID);
      } else {
        renderEmptyWorkoutDetail();
      }
    } else {
      state.workouts = [];
      state.selectedWorkoutID = null;
      workoutsList.innerHTML = `<li class="empty-state">${escapeHTML(extractErrorMessage(workoutsResponse.payload, "Failed to load workouts."))}</li>`;
      setStatus(workoutsStatus, extractErrorMessage(workoutsResponse.payload, "Failed to load workouts."), "error");
      renderEmptyWorkoutDetail();
    }
  }

  function renderShellLoadFailure() {
    state.workouts = [];
    state.selectedWorkoutID = null;
    state.selectedWorkout = null;

    profileSummary.innerHTML = "";
    recommendationCard.innerHTML = `<p class="empty-state">${escapeHTML(shellLoadFailureMessages.recommendation)}</p>`;
    workoutsList.innerHTML = `<li class="empty-state">${escapeHTML(shellLoadFailureMessages.workouts)}</li>`;
    renderEmptyWorkoutDetail();

    setStatus(profileStatus, shellLoadFailureMessages.profile, "error");
    setStatus(recommendationStatus, shellLoadFailureMessages.recommendation, "error");
    setStatus(workoutsStatus, shellLoadFailureMessages.workouts, "error");
  }

  async function loadWorkoutDetail(workoutID) {
    state.selectedWorkoutID = workoutID;
    renderWorkoutsList(state.workouts, state.selectedWorkoutID);
    workoutTitle.textContent = "Loading workout…";
    workoutState.textContent = "Loading";

    const response = await requestJSON(`/api/v1/workouts/${workoutID}`);
    if (!response.ok) {
      renderEmptyWorkoutDetail();
      setStatus(workoutError, extractErrorMessage(response.payload, "Failed to load workout detail."), "error");
      return;
    }

    state.selectedWorkout = response.payload;
    renderWorkoutDetail(response.payload);
  }

  async function saveWorkout() {
    if (!state.selectedWorkoutID) {
      return;
    }

    clearWorkoutError();
    toggleWorkoutActions(true);
    const payload = buildWorkoutPayload(
      workoutNotes.value,
      Array.from(exerciseList.querySelectorAll(".exercise-row")).map((row) => ({
        name: row.querySelector("[data-field='name']").value,
        sets: row.querySelector("[data-field='sets']").value,
        reps: row.querySelector("[data-field='reps']").value,
        weightKg: row.querySelector("[data-field='weight_kg']").value,
        rpe: row.querySelector("[data-field='rpe']").value,
        notes: row.querySelector("[data-field='notes']").value,
      })),
    );

    const response = await requestJSON(`/api/v1/workouts/${state.selectedWorkoutID}`, {
      method: "PUT",
      body: JSON.stringify(payload),
    });
    if (!response.ok) {
      setStatus(workoutError, extractErrorMessage(response.payload, "Failed to save workout."), "error");
      toggleWorkoutActions(false);
      return;
    }

    setStatus(workoutError, "Workout saved.", "success");
    await guardedRefreshShell(state.selectedWorkoutID);
  }

  function renderProfile(profile) {
    const entries = [
      ["Display name", profile.display_name],
      ["Student ID", profile.student_id],
      ["Email", profile.email],
      ["Email verified", profile.email_verified ? "Yes" : "No"],
      ["Visibility", profile.visibility_mode],
      ["Availability", profile.availability_mode],
    ];
    profileSummary.innerHTML = entries
      .map(([label, value]) => `<div><dt>${escapeHTML(label)}</dt><dd>${escapeHTML(String(value ?? "—"))}</dd></div>`)
      .join("");
  }

  function renderRecommendation(recommendation) {
    const summary = recommendationSummary(recommendation);
    const evidence = recommendation.evidence ?? {};
    const evidenceLines = [
      evidence.in_progress_started_at ? `In progress since ${formatTimestamp(evidence.in_progress_started_at)}` : null,
      evidence.last_finished_at ? `Last finished at ${formatTimestamp(evidence.last_finished_at)}` : null,
      Number.isFinite(evidence.recovery_window_hours) ? `Recovery window ${evidence.recovery_window_hours}h` : null,
    ].filter(Boolean);

    recommendationCard.innerHTML = `
      <p class="headline">${escapeHTML(summary.headline)}</p>
      <p>${escapeHTML(summary.detail)}</p>
      <div class="meta">
        <span class="pill">${escapeHTML(recommendation.type)}</span>
        <span class="pill">${escapeHTML(recommendation.reason)}</span>
        <span class="pill">Generated ${escapeHTML(formatTimestamp(recommendation.generated_at))}</span>
      </div>
      ${evidenceLines.length === 0 ? '<p class="section-copy">No additional evidence for this recommendation.</p>' : `<ul>${evidenceLines.map((line) => `<li>${escapeHTML(line)}</li>`).join("")}</ul>`}
      ${recommendation.workout_id ? `<button type="button" id="open-recommended-workout" class="secondary-button">Open referenced workout</button>` : ""}
    `;

    const openButton = recommendationCard.querySelector("#open-recommended-workout");
    if (openButton) {
      openButton.addEventListener("click", () => {
        void loadWorkoutDetail(recommendation.workout_id);
      });
    }
  }

  function renderWorkoutsList(workouts, selectedWorkoutID) {
    if (!Array.isArray(workouts) || workouts.length === 0) {
      workoutsList.innerHTML = `<li class="empty-state">No workouts yet. Start one from this shell.</li>`;
      return;
    }

    workoutsList.innerHTML = workouts
      .map((workout) => `
        <li>
          <button class="workout-list-button${workout.id === selectedWorkoutID ? " is-selected" : ""}" data-workout-id="${escapeHTML(workout.id)}" type="button">
            <span class="label">${escapeHTML(workout.status === "in_progress" ? "In-progress workout" : "Finished workout")}</span>
            <span class="meta">${escapeHTML(workoutListLabel(workout))}</span>
            <span class="meta">${escapeHTML(workout.notes ?? "No notes")}</span>
          </button>
        </li>
      `)
      .join("");
  }

  function renderWorkoutDetail(workout) {
    workoutTitle.textContent = workout.status === "in_progress" ? "In-progress workout" : "Finished workout";
    workoutState.textContent = workout.status === "in_progress" ? "Editable draft" : "Finished";
    workoutNotes.value = workout.notes ?? "";
    renderExerciseRows(Array.isArray(workout.exercises) && workout.exercises.length > 0 ? workout.exercises : [blankExercise()]);
    toggleWorkoutActions(workout.status !== "in_progress");
  }

  function renderEmptyWorkoutDetail() {
    state.selectedWorkout = null;
    workoutTitle.textContent = "Select a workout";
    workoutState.textContent = "No workout selected";
    workoutNotes.value = "";
    exerciseList.innerHTML = `<p class="empty-state">Choose a workout from the list or start a new one.</p>`;
    toggleWorkoutActions(true);
  }

  function renderExerciseRows(exercises) {
    exerciseList.innerHTML = exercises
      .map((exercise, index) => `
        <div class="exercise-row" data-position="${index}">
          ${inputField("Name", "name", exercise.name ?? "", "text")}
          ${inputField("Sets", "sets", String(exercise.sets ?? 0), "number")}
          ${inputField("Reps", "reps", String(exercise.reps ?? 0), "number")}
          ${inputField("Weight (kg)", "weight_kg", exercise.weight_kg ?? "", "number", "0.1")}
          ${inputField("RPE", "rpe", exercise.rpe ?? "", "number", "0.1")}
          ${inputField("Notes", "notes", exercise.notes ?? "", "text")}
          <button class="ghost-button" type="button" data-remove-position="${index}">Remove</button>
        </div>
      `)
      .join("");

    exerciseList.querySelectorAll("[data-remove-position]").forEach((button) => {
      button.addEventListener("click", () => {
        const remaining = Array.from(exerciseList.querySelectorAll(".exercise-row"))
          .filter((row) => row !== button.closest(".exercise-row"))
          .map((row) => ({
            name: row.querySelector("[data-field='name']").value,
            sets: row.querySelector("[data-field='sets']").value,
            reps: row.querySelector("[data-field='reps']").value,
            weight_kg: row.querySelector("[data-field='weight_kg']").value,
            rpe: row.querySelector("[data-field='rpe']").value,
            notes: row.querySelector("[data-field='notes']").value,
          }));
        renderExerciseRows(remaining.length > 0 ? remaining : [blankExercise()]);
      });
    });
  }

  function clearWorkoutError() {
    setStatus(workoutError, "");
  }

  function toggleWorkoutActions(disabled) {
    saveWorkoutButton.disabled = disabled;
    finishWorkoutButton.disabled = disabled;
  }
}

function inputField(label, field, value, type, step = "1") {
  const extra = type === "number" ? ` step="${step}"` : "";
  return `
    <label>
      <span>${escapeHTML(label)}</span>
      <input data-field="${escapeHTML(field)}" type="${escapeHTML(type)}" value="${escapeHTML(String(value ?? ""))}"${extra} />
    </label>
  `;
}

function blankExercise() {
  return {
    name: "",
    sets: 0,
    reps: 0,
    weight_kg: "",
    rpe: "",
    notes: "",
  };
}

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll("\"", "&quot;");
}

function setStatus(element, message, tone = "") {
  if (!element) {
    return;
  }

  element.textContent = message;
  element.classList.remove("error-message", "success-message");
  if (tone === "error") {
    element.classList.add("error-message");
  }
  if (tone === "success") {
    element.classList.add("success-message");
  }
}

boot();
