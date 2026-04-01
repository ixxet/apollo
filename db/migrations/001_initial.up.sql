CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE SCHEMA IF NOT EXISTS apollo;

CREATE TABLE apollo.users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL,
    email TEXT UNIQUE NOT NULL,
    preferences JSONB NOT NULL DEFAULT '{
        "visibility_mode": "ghost",
        "availability_mode": "unavailable",
        "coaching_profile": {}
    }'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE apollo.claimed_tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE CASCADE,
    tag_hash TEXT NOT NULL UNIQUE,
    label TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    claimed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE apollo.visits (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE CASCADE,
    facility_key TEXT NOT NULL,
    zone_key TEXT,
    source_event_id TEXT,
    arrived_at TIMESTAMPTZ NOT NULL,
    departed_at TIMESTAMPTZ,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    UNIQUE (source_event_id)
);

CREATE TABLE apollo.workouts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE CASCADE,
    logged_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    notes TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE apollo.exercises (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workout_id UUID NOT NULL REFERENCES apollo.workouts(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    sets INTEGER NOT NULL CHECK (sets > 0),
    reps INTEGER NOT NULL CHECK (reps > 0),
    weight_kg NUMERIC(6,2),
    rpe NUMERIC(3,1),
    notes TEXT
);

CREATE TABLE apollo.ares_ratings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE CASCADE,
    activity TEXT NOT NULL,
    mu NUMERIC(8,4) NOT NULL DEFAULT 25.0,
    sigma NUMERIC(8,4) NOT NULL DEFAULT 8.3333,
    games_played INTEGER NOT NULL DEFAULT 0,
    last_played TIMESTAMPTZ,
    UNIQUE (user_id, activity)
);

CREATE TABLE apollo.ares_matches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    activity TEXT NOT NULL,
    match_type TEXT NOT NULL CHECK (match_type IN ('1v1', 'team', 'ffa')),
    played_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE apollo.ares_match_players (
    match_id UUID NOT NULL REFERENCES apollo.ares_matches(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE CASCADE,
    team INTEGER,
    outcome TEXT NOT NULL CHECK (outcome IN ('win', 'loss', 'draw')),
    mu_before NUMERIC(8,4) NOT NULL,
    mu_after NUMERIC(8,4) NOT NULL,
    sigma_before NUMERIC(8,4) NOT NULL,
    sigma_after NUMERIC(8,4) NOT NULL,
    PRIMARY KEY (match_id, user_id)
);

CREATE TABLE apollo.recommendations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES apollo.users(id) ON DELETE CASCADE,
    content JSONB NOT NULL,
    model_used TEXT NOT NULL,
    latency_ms INTEGER,
    feedback TEXT CHECK (feedback IN ('helpful', 'not_helpful', 'ignored')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    feedback_at TIMESTAMPTZ
);

CREATE INDEX idx_visits_user_arrived_at
    ON apollo.visits (user_id, arrived_at DESC);

CREATE INDEX idx_workouts_user_logged_at
    ON apollo.workouts (user_id, logged_at DESC);

CREATE INDEX idx_ares_matches_activity_played_at
    ON apollo.ares_matches (activity, played_at DESC);
