CREATE TABLE apollo.facility_catalog_refs (
    facility_key TEXT PRIMARY KEY,
    CONSTRAINT facility_catalog_refs_facility_key_format
        CHECK (facility_key ~ '^[a-z0-9][a-z0-9-]*$')
);

CREATE TABLE apollo.facility_zone_refs (
    facility_key TEXT NOT NULL REFERENCES apollo.facility_catalog_refs(facility_key) ON DELETE CASCADE,
    zone_key TEXT NOT NULL,
    PRIMARY KEY (facility_key, zone_key),
    CONSTRAINT facility_zone_refs_zone_key_format
        CHECK (zone_key ~ '^[a-z0-9][a-z0-9-]*$')
);

CREATE TABLE apollo.sports (
    sport_key TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    description TEXT NOT NULL,
    competition_mode TEXT NOT NULL,
    sides_per_match INTEGER NOT NULL,
    participants_per_side_min INTEGER NOT NULL,
    participants_per_side_max INTEGER NOT NULL,
    scoring_model TEXT NOT NULL,
    default_match_duration_minutes INTEGER NOT NULL,
    rules_summary TEXT NOT NULL,
    CONSTRAINT sports_sport_key_format
        CHECK (sport_key ~ '^[a-z0-9][a-z0-9-]*$'),
    CONSTRAINT sports_display_name_required
        CHECK (btrim(display_name) <> ''),
    CONSTRAINT sports_description_required
        CHECK (btrim(description) <> ''),
    CONSTRAINT sports_competition_mode_allowed
        CHECK (competition_mode IN ('head_to_head')),
    CONSTRAINT sports_sides_per_match_range
        CHECK (sides_per_match >= 2 AND sides_per_match <= 4),
    CONSTRAINT sports_participants_per_side_min_positive
        CHECK (participants_per_side_min > 0),
    CONSTRAINT sports_participants_per_side_max_gte_min
        CHECK (participants_per_side_max >= participants_per_side_min),
    CONSTRAINT sports_scoring_model_allowed
        CHECK (scoring_model IN ('best_of_games', 'running_score')),
    CONSTRAINT sports_default_match_duration_minutes_positive
        CHECK (default_match_duration_minutes > 0),
    CONSTRAINT sports_rules_summary_required
        CHECK (btrim(rules_summary) <> '')
);

CREATE TABLE apollo.sport_facility_capabilities (
    sport_key TEXT NOT NULL REFERENCES apollo.sports(sport_key) ON DELETE CASCADE,
    facility_key TEXT NOT NULL REFERENCES apollo.facility_catalog_refs(facility_key) ON DELETE RESTRICT,
    PRIMARY KEY (sport_key, facility_key)
);

CREATE TABLE apollo.sport_facility_capability_zones (
    sport_key TEXT NOT NULL,
    facility_key TEXT NOT NULL,
    zone_key TEXT NOT NULL,
    PRIMARY KEY (sport_key, facility_key, zone_key),
    FOREIGN KEY (sport_key, facility_key)
        REFERENCES apollo.sport_facility_capabilities(sport_key, facility_key)
        ON DELETE CASCADE,
    FOREIGN KEY (facility_key, zone_key)
        REFERENCES apollo.facility_zone_refs(facility_key, zone_key)
        ON DELETE RESTRICT
);

INSERT INTO apollo.facility_catalog_refs (facility_key)
VALUES
    ('ashtonbee'),
    ('morningside');

INSERT INTO apollo.facility_zone_refs (facility_key, zone_key)
VALUES
    ('ashtonbee', 'gym-floor'),
    ('ashtonbee', 'lobby'),
    ('morningside', 'weight-room');

INSERT INTO apollo.sports (
    sport_key,
    display_name,
    description,
    competition_mode,
    sides_per_match,
    participants_per_side_min,
    participants_per_side_max,
    scoring_model,
    default_match_duration_minutes,
    rules_summary
)
VALUES
    (
        'badminton',
        'Badminton',
        'Indoor court racket sport with singles or doubles support.',
        'head_to_head',
        2,
        1,
        2,
        'best_of_games',
        45,
        'Best-of-three games to 21 points with rally scoring.'
    ),
    (
        'basketball',
        'Basketball',
        'Indoor court ball sport with two sides and running-score regulation.',
        'head_to_head',
        2,
        3,
        5,
        'running_score',
        40,
        'Two sides share one court and play to the scheduled end with running score.'
    );

INSERT INTO apollo.sport_facility_capabilities (sport_key, facility_key)
VALUES
    ('badminton', 'ashtonbee'),
    ('basketball', 'ashtonbee');

INSERT INTO apollo.sport_facility_capability_zones (sport_key, facility_key, zone_key)
VALUES
    ('badminton', 'ashtonbee', 'gym-floor'),
    ('basketball', 'ashtonbee', 'gym-floor');
