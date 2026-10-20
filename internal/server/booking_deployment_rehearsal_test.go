package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ixxet/apollo/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestBookingSchema033OldImageAndRestore(t *testing.T) {
	if os.Getenv("APOLLO_BOOKING_DEPLOY_REHEARSAL") != "1" {
		t.Skip("set APOLLO_BOOKING_DEPLOY_REHEARSAL=1 after pulling the documented amd64 rollback image")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	env := newAuthProfileServerEnv(t)
	defer closeServerEnv(t, env)
	option := insertPublicBookingHTTPResource(t, env, "restore-court", "Restore Rehearsal Court", true, true)
	databaseURL, err := url.Parse(env.db.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	run := func(input []byte, args ...string) []byte {
		t.Helper()
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Stdin = bytes.NewReader(input)
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("%s rehearsal command failed: %v", args[0], err)
		}
		return output
	}
	ids := strings.Fields(string(run(nil, "docker", "ps", "--filter", "publish="+databaseURL.Port(), "--format", "{{.ID}}")))
	if len(ids) != 1 {
		t.Fatalf("expected one disposable PostgreSQL container, got %d", len(ids))
	}
	postgres := ids[0]
	run(nil, "docker", "exec", postgres, "createdb", "-U", "apollo", "migration_rehearsal")
	migrationURL := *databaseURL
	migrationURL.Path = "/migration_rehearsal"
	migrationDB, err := pgxpool.New(ctx, migrationURL.String())
	if err != nil {
		t.Fatal(err)
	}
	defer migrationDB.Close()
	files, err := filepath.Glob(testutil.RepoFilePath("db", "migrations", "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if filepath.Base(file) >= "033" {
			continue
		}
		if err := testutil.ApplySQLFiles(ctx, migrationDB, file); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := migrationDB.Exec(ctx, `
INSERT INTO apollo.users (student_id,display_name,email) VALUES ('migration-four','Synthetic Four','four@example.invalid'),('migration-five','Synthetic Five','five@example.invalid');
INSERT INTO apollo.competition_member_ratings (user_id,sport_key,mode_key,mu,sigma,matches_played)
SELECT id,'badminton','singles',1500,350,CASE WHEN student_id='migration-four' THEN 4 ELSE 5 END FROM apollo.users;
INSERT INTO apollo.competition_rating_events (event_type,rating_engine,engine_version,policy_version,sport_key,projection_watermark,occurred_at)
VALUES ('competition.rating.policy_selected','legacy_elo_like','legacy_elo_like.v1','apollo_legacy_rating_v1','badminton','synthetic',now());`); err != nil {
		t.Fatal(err)
	}
	before := time.Now()
	if err := testutil.ApplySQLFiles(ctx, migrationDB, testutil.RepoFilePath("db", "migrations", "033_competition_rating_policy_wrapper.up.sql")); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(before)
	var provisional, ranked, events int
	if err := migrationDB.QueryRow(ctx, `SELECT count(*) FILTER (WHERE calibration_status='provisional'),count(*) FILTER (WHERE calibration_status='ranked') FROM apollo.competition_member_ratings`).Scan(&provisional, &ranked); err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.QueryRow(ctx, `SELECT count(*) FROM apollo.competition_rating_events`).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if provisional != 1 || ranked != 1 || events != 1 {
		t.Fatalf("033 synthetic backfill=%d/%d events=%d, want 1/1/1", provisional, ranked, events)
	}
	t.Logf("033 SQL delta applied to populated synthetic schema 032 in %s; four/five-match threshold and historical event retained; not production lock-time proof", elapsed)

	rollbackImage := "ghcr.io/ixxet/apollo:sha-6b27618@sha256:6c2d4ef8c67d0ea21d3b41963eb9709d0114b53bfea56c30d456cafbb677515a"
	imageDB := *databaseURL
	imageDB.Host = "host.docker.internal:" + databaseURL.Port()
	child := exec.CommandContext(ctx, "docker", "run", "--rm", "-d", "--platform", "linux/amd64", "-p", "127.0.0.1::8081", "-e", "APOLLO_DATABASE_URL", "-e", "APOLLO_SESSION_COOKIE_SECRET", "-e", "APOLLO_HTTP_ADDR=:8081", "-e", "APOLLO_NATS_URL=", rollbackImage)
	child.Env = append(os.Environ(), "APOLLO_DATABASE_URL="+imageDB.String(), "APOLLO_SESSION_COOKIE_SECRET=0123456789abcdef0123456789abcdef")
	output, err := child.Output()
	if err != nil {
		t.Fatal("could not start pinned amd64 rollback image")
	}
	container := strings.TrimSpace(string(output))
	defer func() { _ = exec.Command("docker", "rm", "-f", container).Run() }()
	endpoint := strings.TrimSpace(string(run(nil, "docker", "port", container, "8081/tcp")))
	origin := "http://" + endpoint
	client := &http.Client{Timeout: 2 * time.Second}
	ready := false
	for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); {
		response, err := client.Get(origin + "/api/v1/public/booking/options")
		if err == nil {
			body, _ := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if response.StatusCode == 200 && bytes.Contains(body, []byte(option.String())) {
				ready = true
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		t.Fatal("old pinned image could not read schema 033 public options")
	}
	start, end := publicBookingWindow(72*time.Hour, time.Hour)
	body := publicBookingRequestJSON(option, start, end, `"contact_email":"restore@example.invalid"`)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, origin+"/api/v1/public/booking/requests", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "rollback-schema-compatibility")
	request.Header.Set("X-Apollo-Public-Intake-Channel", "public_web")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != 202 {
		t.Fatalf("old image write on schema 033 returned %d", response.StatusCode)
	}
	dump := run(nil, "docker", "exec", postgres, "pg_dump", "-U", "apollo", "-Fc", "apollo")
	run(nil, "docker", "exec", postgres, "createdb", "-U", "apollo", "booking_restore")
	run(dump, "docker", "exec", "-i", postgres, "pg_restore", "--exit-on-error", "--no-owner", "-U", "apollo", "-d", "booking_restore")
	restoredURL := *databaseURL
	restoredURL.Path = "/booking_restore"
	restored, err := pgxpool.New(ctx, restoredURL.String())
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	var requests, resources int
	if err := restored.QueryRow(ctx, `SELECT (SELECT count(*) FROM apollo.booking_requests WHERE status='requested'),(SELECT count(*) FROM apollo.schedule_resources WHERE active AND bookable)`).Scan(&requests, &resources); err != nil {
		t.Fatal(err)
	}
	if requests != 1 || resources != 1 {
		t.Fatalf("restored request/resource counts=%d/%d, want 1/1", requests, resources)
	}
	t.Logf("PASS: actual pinned amd64 image reads/writes schema 033; PostgreSQL custom dump restored to separate disposable database, bytes=%d sha256=%x, requests/resources=1/1. This is not an independent live backup or safe concurrent-booking rollback.", len(dump), sha256.Sum256(dump))
}
