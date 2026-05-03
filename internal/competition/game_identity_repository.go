package competition

import (
	"context"
	"time"
)

func (r *Repository) ListGameIdentityProjectionRows(ctx context.Context, input GameIdentityProjectionInput) ([]gameIdentityProjectionRowRecord, error) {
	var userID any
	if input.UserID != nil {
		userID = *input.UserID
	}

	rows, err := r.db.Query(ctx, `
WITH projection_candidates AS (
  SELECT
    p.user_id,
    p.sport_key,
    p.mode_key,
    p.facility_key,
    p.team_scope,
    p.stat_type,
    p.stat_value,
    p.computed_at
  FROM apollo.competition_analytics_projections AS p
  WHERE p.projection_version = $1
    AND p.stat_type = ANY($2::text[])
    AND ($3::uuid IS NULL OR p.user_id = $3::uuid)
    AND ($4 = '' OR p.sport_key = $4)
    AND ($5 = '' OR p.mode_key = $5)
    AND ($6 = '' OR p.facility_key = $6)
    AND p.team_scope = $7
  ORDER BY p.computed_at DESC, p.user_id ASC, p.stat_type ASC
  LIMIT $13
),
identity_rows AS (
  SELECT
    p.user_id,
    p.sport_key,
    p.mode_key,
    p.facility_key,
    p.team_scope,
    COALESCE(MAX(p.stat_value::double precision) FILTER (WHERE p.stat_type = 'matches_played'), 0) AS matches_played,
    COALESCE(MAX(p.stat_value::double precision) FILTER (WHERE p.stat_type = 'wins'), 0) AS wins,
    COALESCE(MAX(p.stat_value::double precision) FILTER (WHERE p.stat_type = 'losses'), 0) AS losses,
    COALESCE(MAX(p.stat_value::double precision) FILTER (WHERE p.stat_type = 'draws'), 0) AS draws,
    MAX(cmr.last_played) AS last_result_at,
    MAX(p.computed_at) AS computed_at
  FROM projection_candidates AS p
  LEFT JOIN apollo.competition_member_ratings AS cmr
    ON cmr.user_id = p.user_id
   AND cmr.sport_key = p.sport_key
   AND cmr.mode_key = p.mode_key
   AND cmr.rating_engine = 'legacy_elo_like'
   AND cmr.engine_version = 'legacy_elo_like.v1'
   AND cmr.policy_version = 'apollo_rating_policy_wrapper_v1'
  GROUP BY p.user_id,
           p.sport_key,
           p.mode_key,
           p.facility_key,
           p.team_scope
),
participant_contexts AS (
  SELECT *
  FROM identity_rows
  WHERE matches_played > 0
  ORDER BY
    ((matches_played * $9) + (wins * $10) + (draws * $11) + (losses * $12)) DESC,
    computed_at DESC,
    user_id ASC
  LIMIT $14
)
SELECT
  user_id,
  sport_key,
  mode_key,
  facility_key,
  team_scope,
  matches_played,
  wins,
  losses,
  draws,
  last_result_at,
  computed_at
FROM participant_contexts
ORDER BY
  ((matches_played * $9) + (wins * $10) + (draws * $11) + (losses * $12)) DESC,
  computed_at DESC,
  user_id ASC
LIMIT $8
`, competitionAnalyticsProjectionVersion, gameIdentityStatTypes, userID, input.SportKey, input.ModeKey, input.FacilityKey, input.TeamScope, int32(input.Limit), gameIdentityCPMatchesPlayedPoints, gameIdentityCPWinPoints, gameIdentityCPDrawPoints, gameIdentityCPLossPoints, int32(maxGameIdentityPublicProjectionRows), int32(maxGameIdentityParticipantContextRows))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	projections := make([]gameIdentityProjectionRowRecord, 0)
	for rows.Next() {
		var row gameIdentityProjectionRowRecord
		var lastResultAt *time.Time
		if err := rows.Scan(
			&row.UserID,
			&row.SportKey,
			&row.ModeKey,
			&row.FacilityKey,
			&row.TeamScope,
			&row.MatchesPlayed,
			&row.Wins,
			&row.Losses,
			&row.Draws,
			&lastResultAt,
			&row.ComputedAt,
		); err != nil {
			return nil, err
		}
		if lastResultAt != nil {
			utc := lastResultAt.UTC()
			row.LastResultAt = &utc
		}
		row.ComputedAt = row.ComputedAt.UTC()
		projections = append(projections, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return projections, nil
}
