-- name: GetLatestFinishedWorkoutByUserID :one
SELECT w.*
FROM apollo.workouts AS w
WHERE w.user_id = $1
  AND w.status = 'finished'
ORDER BY w.finished_at DESC,
         w.started_at DESC,
         w.id DESC
LIMIT 1;
