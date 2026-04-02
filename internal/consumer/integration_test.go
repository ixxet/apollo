package consumer

import (
	"context"
	"testing"
	"time"

	protoevents "github.com/ixxet/ashton-proto/events"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"

	"github.com/ixxet/apollo/internal/store"
	"github.com/ixxet/apollo/internal/testutil"
	"github.com/ixxet/apollo/internal/visits"
)

func TestIdentifiedPresenceIntegrationCreatesOneVisitWithoutWorkout(t *testing.T) {
	ctx := context.Background()
	postgresEnv, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres() error = %v", err)
	}
	defer func() {
		if err := postgresEnv.Close(); err != nil {
			t.Fatalf("postgresEnv.Close() error = %v", err)
		}
	}()

	if err := testutil.ApplyApolloSchema(ctx, postgresEnv.DB); err != nil {
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}
	if err := testutil.ApplySQLFiles(ctx, postgresEnv.DB, testutil.RepoFilePath("db", "seeds", "tracer2.sql")); err != nil {
		t.Fatalf("ApplySQLFiles(seed) error = %v", err)
	}

	natsEnv, err := testutil.StartNATS()
	if err != nil {
		t.Fatalf("StartNATS() error = %v", err)
	}
	defer func() {
		if err := natsEnv.Close(); err != nil {
			t.Fatalf("natsEnv.Close() error = %v", err)
		}
	}()

	repository := visits.NewRepository(postgresEnv.DB)
	service := visits.NewService(repository)
	handler := NewIdentifiedPresenceHandler(service)
	if _, err := natsEnv.Conn.Subscribe(protoevents.SubjectIdentifiedPresenceArrived, func(msg *nats.Msg) {
		_, _ = handler.HandleMessage(context.Background(), msg.Data)
	}); err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	if err := natsEnv.Conn.Publish(protoevents.SubjectIdentifiedPresenceArrived, protoevents.ValidIdentifiedPresenceArrivedFixture()); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if err := natsEnv.Conn.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		rows, err := repository.ListByStudentID(ctx, "tracer2-student-001")
		if err != nil {
			t.Fatalf("ListByStudentID() error = %v", err)
		}
		if len(rows) == 1 {
			var workoutCount int
			if err := postgresEnv.DB.QueryRow(ctx, "SELECT count(*) FROM apollo.workouts").Scan(&workoutCount); err != nil {
				t.Fatalf("QueryRow(workouts) error = %v", err)
			}
			if workoutCount != 0 {
				t.Fatalf("workoutCount = %d, want 0", workoutCount)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("len(rows) = %d, want 1", len(rows))
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestIdentifiedDepartureIntegrationClosesVisitWithoutMutatingWorkoutClaimedTagOrPreferencesState(t *testing.T) {
	ctx := context.Background()
	postgresEnv, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres() error = %v", err)
	}
	defer func() {
		if err := postgresEnv.Close(); err != nil {
			t.Fatalf("postgresEnv.Close() error = %v", err)
		}
	}()

	if err := testutil.ApplyApolloSchema(ctx, postgresEnv.DB); err != nil {
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}
	if err := testutil.ApplySQLFiles(ctx, postgresEnv.DB, testutil.RepoFilePath("db", "seeds", "tracer2.sql")); err != nil {
		t.Fatalf("ApplySQLFiles(seed) error = %v", err)
	}

	natsEnv, err := testutil.StartNATS()
	if err != nil {
		t.Fatalf("StartNATS() error = %v", err)
	}
	defer func() {
		if err := natsEnv.Close(); err != nil {
			t.Fatalf("natsEnv.Close() error = %v", err)
		}
	}()

	repository := visits.NewRepository(postgresEnv.DB)
	service := visits.NewService(repository)
	arrivalHandler := NewIdentifiedPresenceHandler(service)
	departureHandler := NewIdentifiedDepartureHandler(service)
	if _, err := natsEnv.Conn.Subscribe(protoevents.SubjectIdentifiedPresenceArrived, func(msg *nats.Msg) {
		_, _ = arrivalHandler.HandleMessage(context.Background(), msg.Data)
	}); err != nil {
		t.Fatalf("Subscribe(arrived) error = %v", err)
	}
	if _, err := natsEnv.Conn.Subscribe(protoevents.SubjectIdentifiedPresenceDeparted, func(msg *nats.Msg) {
		_, _ = departureHandler.HandleMessage(context.Background(), msg.Data)
	}); err != nil {
		t.Fatalf("Subscribe(departed) error = %v", err)
	}

	if err := natsEnv.Conn.Publish(protoevents.SubjectIdentifiedPresenceArrived, protoevents.ValidIdentifiedPresenceArrivedFixture()); err != nil {
		t.Fatalf("Publish(arrived) error = %v", err)
	}
	if err := natsEnv.Conn.Flush(); err != nil {
		t.Fatalf("Flush(arrived) error = %v", err)
	}

	arrivedVisit := waitForVisitCount(t, ctx, repository, "tracer2-student-001", 1)
	if arrivedVisit.DepartedAt.Valid {
		t.Fatalf("arrivedVisit.DepartedAt.Valid = %t, want false before departure", arrivedVisit.DepartedAt.Valid)
	}

	user := lookupIntegrationUser(t, ctx, repository, "tag_tracer2_001")
	insertClaimedTag(t, ctx, postgresEnv.DB, user.ID, "tag_tracer5_001", "departure tracer tag")
	preferencesBefore := readPreferencesJSON(t, ctx, postgresEnv.DB, user.ID)
	workoutsBefore := countTableRows(t, ctx, postgresEnv.DB, "apollo.workouts")
	claimedTagsBefore := countUserRows(t, ctx, postgresEnv.DB, "apollo.claimed_tags", user.ID)

	if err := natsEnv.Conn.Publish(protoevents.SubjectIdentifiedPresenceDeparted, protoevents.ValidIdentifiedPresenceDepartedFixture()); err != nil {
		t.Fatalf("Publish(departed) error = %v", err)
	}
	if err := natsEnv.Conn.Flush(); err != nil {
		t.Fatalf("Flush(departed) error = %v", err)
	}

	closedVisit := waitForClosedVisit(t, ctx, repository, "tracer2-student-001")
	if closedVisit.DepartureSourceEventID == nil || *closedVisit.DepartureSourceEventID != "mock-out-001" {
		t.Fatalf("closedVisit.DepartureSourceEventID = %#v, want mock-out-001", closedVisit.DepartureSourceEventID)
	}

	if workoutsAfter := countTableRows(t, ctx, postgresEnv.DB, "apollo.workouts"); workoutsAfter != workoutsBefore {
		t.Fatalf("workout count changed from %d to %d after departure close", workoutsBefore, workoutsAfter)
	}
	if claimedTagsAfter := countUserRows(t, ctx, postgresEnv.DB, "apollo.claimed_tags", user.ID); claimedTagsAfter != claimedTagsBefore {
		t.Fatalf("claimed tag count changed from %d to %d after departure close", claimedTagsBefore, claimedTagsAfter)
	}
	if preferencesAfter := readPreferencesJSON(t, ctx, postgresEnv.DB, user.ID); preferencesAfter != preferencesBefore {
		t.Fatalf("preferences changed from %s to %s after departure close", preferencesBefore, preferencesAfter)
	}
}

func TestIdentifiedDepartureReplayIsIdempotentEndToEnd(t *testing.T) {
	ctx := context.Background()
	postgresEnv, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres() error = %v", err)
	}
	defer func() {
		if err := postgresEnv.Close(); err != nil {
			t.Fatalf("postgresEnv.Close() error = %v", err)
		}
	}()

	if err := testutil.ApplyApolloSchema(ctx, postgresEnv.DB); err != nil {
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}
	if err := testutil.ApplySQLFiles(ctx, postgresEnv.DB, testutil.RepoFilePath("db", "seeds", "tracer2.sql")); err != nil {
		t.Fatalf("ApplySQLFiles(seed) error = %v", err)
	}

	natsEnv, err := testutil.StartNATS()
	if err != nil {
		t.Fatalf("StartNATS() error = %v", err)
	}
	defer func() {
		if err := natsEnv.Close(); err != nil {
			t.Fatalf("natsEnv.Close() error = %v", err)
		}
	}()

	repository := visits.NewRepository(postgresEnv.DB)
	service := visits.NewService(repository)
	user := lookupIntegrationUser(t, ctx, repository, "tag_tracer2_001")
	insertClaimedTag(t, ctx, postgresEnv.DB, user.ID, "tag_tracer5_001", "departure tracer tag")
	arrivalResults := make(chan visits.Result, 1)
	departureResults := make(chan visits.Result, 2)
	if _, err := natsEnv.Conn.Subscribe(protoevents.SubjectIdentifiedPresenceArrived, func(msg *nats.Msg) {
		result, _ := NewIdentifiedPresenceHandler(service).HandleMessage(context.Background(), msg.Data)
		arrivalResults <- result
	}); err != nil {
		t.Fatalf("Subscribe(arrived) error = %v", err)
	}
	if _, err := natsEnv.Conn.Subscribe(protoevents.SubjectIdentifiedPresenceDeparted, func(msg *nats.Msg) {
		result, _ := NewIdentifiedDepartureHandler(service).HandleMessage(context.Background(), msg.Data)
		departureResults <- result
	}); err != nil {
		t.Fatalf("Subscribe(departed) error = %v", err)
	}

	if err := natsEnv.Conn.Publish(protoevents.SubjectIdentifiedPresenceArrived, protoevents.ValidIdentifiedPresenceArrivedFixture()); err != nil {
		t.Fatalf("Publish(arrived) error = %v", err)
	}
	if err := natsEnv.Conn.Flush(); err != nil {
		t.Fatalf("Flush(arrived) error = %v", err)
	}
	assertOutcome(t, arrivalResults, visits.OutcomeCreated)

	if err := natsEnv.Conn.Publish(protoevents.SubjectIdentifiedPresenceDeparted, protoevents.ValidIdentifiedPresenceDepartedFixture()); err != nil {
		t.Fatalf("Publish(first departed) error = %v", err)
	}
	if err := natsEnv.Conn.Publish(protoevents.SubjectIdentifiedPresenceDeparted, protoevents.ValidIdentifiedPresenceDepartedFixture()); err != nil {
		t.Fatalf("Publish(second departed) error = %v", err)
	}
	if err := natsEnv.Conn.Flush(); err != nil {
		t.Fatalf("Flush(departed replay) error = %v", err)
	}

	assertOutcome(t, departureResults, visits.OutcomeClosed)
	assertOutcome(t, departureResults, visits.OutcomeDuplicate)

	closedVisit := waitForClosedVisit(t, ctx, repository, "tracer2-student-001")
	if closedVisit.DepartureSourceEventID == nil || *closedVisit.DepartureSourceEventID != "mock-out-001" {
		t.Fatalf("closedVisit.DepartureSourceEventID = %#v, want mock-out-001", closedVisit.DepartureSourceEventID)
	}
	rows, err := repository.ListByStudentID(ctx, "tracer2-student-001")
	if err != nil {
		t.Fatalf("ListByStudentID() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1 after replay", len(rows))
	}
}

func TestIdentifiedPresenceLifecycleDoesNotFinishExistingInProgressWorkout(t *testing.T) {
	ctx := context.Background()
	postgresEnv, err := testutil.StartPostgres(ctx)
	if err != nil {
		t.Fatalf("StartPostgres() error = %v", err)
	}
	defer func() {
		if err := postgresEnv.Close(); err != nil {
			t.Fatalf("postgresEnv.Close() error = %v", err)
		}
	}()

	if err := testutil.ApplyApolloSchema(ctx, postgresEnv.DB); err != nil {
		t.Fatalf("ApplyApolloSchema() error = %v", err)
	}
	if err := testutil.ApplySQLFiles(ctx, postgresEnv.DB, testutil.RepoFilePath("db", "seeds", "tracer2.sql")); err != nil {
		t.Fatalf("ApplySQLFiles(seed) error = %v", err)
	}

	natsEnv, err := testutil.StartNATS()
	if err != nil {
		t.Fatalf("StartNATS() error = %v", err)
	}
	defer func() {
		if err := natsEnv.Close(); err != nil {
			t.Fatalf("natsEnv.Close() error = %v", err)
		}
	}()

	repository := visits.NewRepository(postgresEnv.DB)
	service := visits.NewService(repository)
	arrivalHandler := NewIdentifiedPresenceHandler(service)
	departureHandler := NewIdentifiedDepartureHandler(service)
	if _, err := natsEnv.Conn.Subscribe(protoevents.SubjectIdentifiedPresenceArrived, func(msg *nats.Msg) {
		_, _ = arrivalHandler.HandleMessage(context.Background(), msg.Data)
	}); err != nil {
		t.Fatalf("Subscribe(arrived) error = %v", err)
	}
	if _, err := natsEnv.Conn.Subscribe(protoevents.SubjectIdentifiedPresenceDeparted, func(msg *nats.Msg) {
		_, _ = departureHandler.HandleMessage(context.Background(), msg.Data)
	}); err != nil {
		t.Fatalf("Subscribe(departed) error = %v", err)
	}

	user := lookupIntegrationUser(t, ctx, repository, "tag_tracer2_001")
	insertClaimedTag(t, ctx, postgresEnv.DB, user.ID, "tag_tracer5_001", "departure tracer tag")
	var workoutID string
	if err := postgresEnv.DB.QueryRow(ctx, `
INSERT INTO apollo.workouts (user_id, notes, metadata)
VALUES ($1, $2, $3::jsonb)
RETURNING id::text
`, user.ID, "in progress runtime workout", `{"source":"runtime"}`).Scan(&workoutID); err != nil {
		t.Fatalf("QueryRow(insert workout) error = %v", err)
	}

	if err := natsEnv.Conn.Publish(protoevents.SubjectIdentifiedPresenceArrived, protoevents.ValidIdentifiedPresenceArrivedFixture()); err != nil {
		t.Fatalf("Publish(arrived) error = %v", err)
	}
	if err := natsEnv.Conn.Flush(); err != nil {
		t.Fatalf("Flush(arrived) error = %v", err)
	}
	waitForVisitCount(t, ctx, repository, "tracer2-student-001", 1)
	assertWorkoutState(t, ctx, postgresEnv.DB, workoutID, "in_progress", false)

	if err := natsEnv.Conn.Publish(protoevents.SubjectIdentifiedPresenceDeparted, protoevents.ValidIdentifiedPresenceDepartedFixture()); err != nil {
		t.Fatalf("Publish(departed) error = %v", err)
	}
	if err := natsEnv.Conn.Flush(); err != nil {
		t.Fatalf("Flush(departed) error = %v", err)
	}
	waitForClosedVisit(t, ctx, repository, "tracer2-student-001")
	assertWorkoutState(t, ctx, postgresEnv.DB, workoutID, "in_progress", false)
}

func waitForVisitCount(t *testing.T, ctx context.Context, repository *visits.Repository, studentID string, want int) store.ApolloVisit {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		rows, err := repository.ListByStudentID(ctx, studentID)
		if err != nil {
			t.Fatalf("ListByStudentID() error = %v", err)
		}
		if len(rows) == want {
			return rows[0]
		}
		if time.Now().After(deadline) {
			t.Fatalf("len(rows) = %d, want %d", len(rows), want)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func waitForClosedVisit(t *testing.T, ctx context.Context, repository *visits.Repository, studentID string) store.ApolloVisit {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for {
		rows, err := repository.ListByStudentID(ctx, studentID)
		if err != nil {
			t.Fatalf("ListByStudentID() error = %v", err)
		}
		if len(rows) == 1 && rows[0].DepartedAt.Valid {
			return rows[0]
		}
		if time.Now().After(deadline) {
			t.Fatalf("visit did not close, rows = %#v", rows)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func lookupIntegrationUser(t *testing.T, ctx context.Context, repository *visits.Repository, tagHash string) *store.ApolloUser {
	t.Helper()

	user, err := repository.FindActiveUserByTagHash(ctx, tagHash)
	if err != nil {
		t.Fatalf("FindActiveUserByTagHash() error = %v", err)
	}
	if user == nil {
		t.Fatalf("FindActiveUserByTagHash(%q) returned nil user", tagHash)
	}

	return user
}

func readPreferencesJSON(t *testing.T, ctx context.Context, db *pgxpool.Pool, userID any) string {
	t.Helper()

	var preferences string
	if err := db.QueryRow(ctx, "SELECT preferences::text FROM apollo.users WHERE id = $1", userID).Scan(&preferences); err != nil {
		t.Fatalf("QueryRow(preferences) error = %v", err)
	}

	return preferences
}

func countTableRows(t *testing.T, ctx context.Context, db *pgxpool.Pool, table string) int {
	t.Helper()

	var count int
	query := "SELECT count(*) FROM " + table
	if err := db.QueryRow(ctx, query).Scan(&count); err != nil {
		t.Fatalf("QueryRow(%s) error = %v", table, err)
	}

	return count
}

func countUserRows(t *testing.T, ctx context.Context, db *pgxpool.Pool, table string, userID any) int {
	t.Helper()

	var count int
	query := "SELECT count(*) FROM " + table + " WHERE user_id = $1"
	if err := db.QueryRow(ctx, query, userID).Scan(&count); err != nil {
		t.Fatalf("QueryRow(%s by user) error = %v", table, err)
	}

	return count
}

func assertOutcome(t *testing.T, results <-chan visits.Result, want visits.Outcome) {
	t.Helper()

	select {
	case result := <-results:
		if result.Outcome != want {
			t.Fatalf("result.Outcome = %q, want %q", result.Outcome, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for outcome %q", want)
	}
}

func insertClaimedTag(t *testing.T, ctx context.Context, db *pgxpool.Pool, userID any, tagHash string, label string) {
	t.Helper()

	if _, err := db.Exec(ctx, "INSERT INTO apollo.claimed_tags (user_id, tag_hash, label, is_active) VALUES ($1, $2, $3, TRUE)", userID, tagHash, label); err != nil {
		t.Fatalf("Exec(insert claimed tag %q) error = %v", tagHash, err)
	}
}

func assertWorkoutState(t *testing.T, ctx context.Context, db *pgxpool.Pool, workoutID string, wantStatus string, wantFinished bool) {
	t.Helper()

	var status string
	var finishedAt *time.Time
	if err := db.QueryRow(ctx, "SELECT status, finished_at FROM apollo.workouts WHERE id = $1", workoutID).Scan(&status, &finishedAt); err != nil {
		t.Fatalf("QueryRow(workout state) error = %v", err)
	}
	if status != wantStatus {
		t.Fatalf("status = %q, want %q", status, wantStatus)
	}
	if (finishedAt != nil) != wantFinished {
		t.Fatalf("finishedAt presence = %t, want %t", finishedAt != nil, wantFinished)
	}
}
