package competition

import (
	"context"
	"time"
)

func (r *Repository) GetPublicCompetitionReadiness(ctx context.Context) (publicCompetitionReadinessRecord, error) {
	var record publicCompetitionReadinessRecord
	err := r.db.QueryRow(ctx, `
SELECT
  (SELECT count(*)
   FROM apollo.competition_analytics_projections
   WHERE projection_version = $1
     AND stat_type = ANY($2::text[])) AS available_leaderboards,
  (SELECT count(DISTINCT r.id)
   FROM apollo.competition_matches AS m
   INNER JOIN apollo.competition_match_results AS r
     ON r.id = m.canonical_result_id
   WHERE m.status = 'completed'
     AND r.result_status IN ('finalized', 'corrected')) AS available_canonical_results
`, competitionAnalyticsProjectionVersion, publicLeaderboardStatTypes).Scan(&record.AvailableLeaderboards, &record.AvailableCanonicalResults)
	return record, err
}

func (r *Repository) ListPublicCompetitionLeaderboard(ctx context.Context, input PublicCompetitionLeaderboardInput) ([]publicCompetitionLeaderboardRowRecord, error) {
	rows, err := r.db.Query(ctx, `
WITH leaderboard_candidates AS (
  SELECT
    p.user_id,
    p.sport_key,
    p.mode_key,
    p.facility_key,
    p.team_scope,
    p.stat_type,
    p.stat_value,
    p.sample_size,
    p.confidence,
    p.computed_at
  FROM apollo.competition_analytics_projections AS p
  WHERE p.projection_version = $1
    AND p.stat_type = $2
    AND p.team_scope = $3
    AND ($4 = '' OR p.sport_key = $4)
    AND ($5 = '' OR p.mode_key = $5)
  ORDER BY p.stat_value DESC, p.sample_size DESC, p.computed_at DESC, p.user_id ASC
  LIMIT $7
)
SELECT
  p.sport_key,
  p.mode_key,
  p.facility_key,
  p.team_scope,
  p.stat_type,
  p.stat_value::double precision,
  p.sample_size,
  p.confidence::double precision,
  MAX(cmr.last_played) AS last_result_at,
  p.computed_at
FROM leaderboard_candidates AS p
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
         p.team_scope,
         p.stat_type,
         p.stat_value,
         p.sample_size,
         p.confidence,
         p.computed_at
ORDER BY p.stat_value DESC, p.sample_size DESC, p.computed_at DESC, p.user_id ASC
LIMIT $6
`, competitionAnalyticsProjectionVersion, input.StatType, input.TeamScope, input.SportKey, input.ModeKey, int32(input.Limit), int32(maxPublicLeaderboardRowsScanned))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	leaderboard := make([]publicCompetitionLeaderboardRowRecord, 0)
	for rows.Next() {
		var row publicCompetitionLeaderboardRowRecord
		var lastResultAt *time.Time
		if err := rows.Scan(
			&row.SportKey,
			&row.ModeKey,
			&row.FacilityKey,
			&row.TeamScope,
			&row.StatType,
			&row.StatValue,
			&row.SampleSize,
			&row.Confidence,
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
		leaderboard = append(leaderboard, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return leaderboard, nil
}
