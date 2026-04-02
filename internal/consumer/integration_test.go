package consumer

import (
	"context"
	"testing"
	"time"

	protoevents "github.com/ixxet/ashton-proto/events"
	"github.com/nats-io/nats.go"

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

	if err := testutil.ApplySQLFiles(
		ctx,
		postgresEnv.DB,
		testutil.RepoFilePath("db", "migrations", "001_initial.up.sql"),
		testutil.RepoFilePath("db", "migrations", "002_open_visit_uniqueness.up.sql"),
		testutil.RepoFilePath("db", "migrations", "003_member_auth_and_sessions.up.sql"),
		testutil.RepoFilePath("db", "seeds", "tracer2.sql"),
	); err != nil {
		t.Fatalf("ApplySQLFiles() error = %v", err)
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
