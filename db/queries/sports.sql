-- name: ListSports :many
SELECT sport_key,
       display_name,
       description,
       competition_mode,
       sides_per_match,
       participants_per_side_min,
       participants_per_side_max,
       scoring_model,
       default_match_duration_minutes,
       rules_summary
FROM apollo.sports
ORDER BY sport_key;

-- name: GetSportByKey :one
SELECT sport_key,
       display_name,
       description,
       competition_mode,
       sides_per_match,
       participants_per_side_min,
       participants_per_side_max,
       scoring_model,
       default_match_duration_minutes,
       rules_summary
FROM apollo.sports
WHERE sport_key = $1
LIMIT 1;

-- name: FacilityCatalogRefExists :one
SELECT EXISTS (
    SELECT 1
    FROM apollo.facility_catalog_refs
    WHERE facility_key = $1
);

-- name: ListFacilityCatalogRefs :many
SELECT facility_key
FROM apollo.facility_catalog_refs
ORDER BY facility_key;

-- name: ListSportFacilityCapabilities :many
SELECT c.sport_key,
       c.facility_key,
       z.zone_key
FROM apollo.sport_facility_capabilities AS c
LEFT JOIN apollo.sport_facility_capability_zones AS z
  ON z.sport_key = c.sport_key
 AND z.facility_key = c.facility_key
ORDER BY c.sport_key, c.facility_key, z.zone_key;
