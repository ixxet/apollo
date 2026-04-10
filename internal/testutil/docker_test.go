package testutil

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestDockerHostResolutionAndOverride(t *testing.T) {
	t.Run("env override wins", func(t *testing.T) {
		t.Setenv("DOCKER_HOST_NAME", "docker.internal")
		originalLookupHost := lookupHost
		lookupHost = func(string) ([]string, error) {
			t.Fatal("lookupHost should not run when DOCKER_HOST_NAME is set")
			return nil, nil
		}
		defer func() {
			lookupHost = originalLookupHost
		}()

		if host := dockerHost(); host != "docker.internal" {
			t.Fatalf("dockerHost() = %q, want docker.internal", host)
		}
	})

	t.Run("host.docker.internal when resolvable", func(t *testing.T) {
		t.Setenv("DOCKER_HOST_NAME", "")
		originalLookupHost := lookupHost
		lookupHost = func(string) ([]string, error) {
			return []string{"192.168.65.2"}, nil
		}
		defer func() {
			lookupHost = originalLookupHost
		}()

		if host := dockerHost(); host != "host.docker.internal" {
			t.Fatalf("dockerHost() = %q, want host.docker.internal", host)
		}
	})

	t.Run("localhost fallback when host.docker.internal does not resolve", func(t *testing.T) {
		t.Setenv("DOCKER_HOST_NAME", "")
		originalLookupHost := lookupHost
		lookupHost = func(string) ([]string, error) {
			return nil, errors.New("lookup failed")
		}
		defer func() {
			lookupHost = originalLookupHost
		}()

		if host := dockerHost(); host != "127.0.0.1" {
			t.Fatalf("dockerHost() = %q, want 127.0.0.1", host)
		}
	})
}

func TestWaitForPostgresClosesFailedAttempts(t *testing.T) {
	originalNewPostgresPool := newPostgresPool
	originalPingPostgresPool := pingPostgresPool
	originalClosePostgresPool := closePostgresPool
	originalSleepWithContext := sleepWithContext
	defer func() {
		newPostgresPool = originalNewPostgresPool
		pingPostgresPool = originalPingPostgresPool
		closePostgresPool = originalClosePostgresPool
		sleepWithContext = originalSleepWithContext
	}()

	attempts := []*pgxpool.Pool{{}, {}}
	newCalls := 0
	newPostgresPool = func(context.Context, string) (*pgxpool.Pool, error) {
		db := attempts[newCalls]
		newCalls++
		return db, nil
	}

	pingCalls := 0
	pingPostgresPool = func(context.Context, *pgxpool.Pool) error {
		pingCalls++
		if pingCalls == 1 {
			return errors.New("connection refused")
		}
		return nil
	}

	var closed []*pgxpool.Pool
	closePostgresPool = func(db *pgxpool.Pool) {
		closed = append(closed, db)
	}

	sleepCalls := 0
	sleepWithContext = func(context.Context, time.Duration) error {
		sleepCalls++
		return nil
	}

	db, err := waitForPostgres(context.Background(), "postgres://apollo", postgresStartupConfig{
		maxWait:     time.Second,
		retryDelay:  time.Millisecond,
		pingTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("waitForPostgres() error = %v", err)
	}
	if db != attempts[1] {
		t.Fatalf("waitForPostgres() returned unexpected pool")
	}
	if len(closed) != 1 || closed[0] != attempts[0] {
		t.Fatalf("closed pools = %v, want only first failed attempt", closed)
	}
	if sleepCalls != 1 {
		t.Fatalf("sleepCalls = %d, want 1", sleepCalls)
	}
}

func TestWaitForPostgresReturnsLastErrorAfterDeadline(t *testing.T) {
	originalNewPostgresPool := newPostgresPool
	originalPingPostgresPool := pingPostgresPool
	originalClosePostgresPool := closePostgresPool
	originalSleepWithContext := sleepWithContext
	originalNowTime := nowTime
	defer func() {
		newPostgresPool = originalNewPostgresPool
		pingPostgresPool = originalPingPostgresPool
		closePostgresPool = originalClosePostgresPool
		sleepWithContext = originalSleepWithContext
		nowTime = originalNowTime
	}()

	currentTime := time.Unix(0, 0)
	nowTime = func() time.Time {
		return currentTime
	}

	attempts := []*pgxpool.Pool{{}, {}}
	newCalls := 0
	newPostgresPool = func(context.Context, string) (*pgxpool.Pool, error) {
		db := attempts[newCalls]
		newCalls++
		return db, nil
	}

	pingPostgresPool = func(context.Context, *pgxpool.Pool) error {
		return errors.New("connection refused")
	}

	var closed []*pgxpool.Pool
	closePostgresPool = func(db *pgxpool.Pool) {
		closed = append(closed, db)
	}

	sleepWithContext = func(context.Context, time.Duration) error {
		currentTime = currentTime.Add(time.Second)
		return nil
	}

	_, err := waitForPostgres(context.Background(), "postgres://apollo", postgresStartupConfig{
		maxWait:     time.Second,
		retryDelay:  time.Second,
		pingTimeout: time.Second,
	})
	if err == nil {
		t.Fatal("waitForPostgres() error = nil, want deadline error")
	}
	if !strings.Contains(err.Error(), "postgres did not become ready within 1s") {
		t.Fatalf("waitForPostgres() error = %q, want deadline message", err.Error())
	}
	if len(closed) != 2 {
		t.Fatalf("closed pools = %d, want 2 failed attempts closed", len(closed))
	}
}
