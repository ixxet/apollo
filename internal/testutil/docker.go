package testutil

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/ory/dockertest/v3"
	dc "github.com/ory/dockertest/v3/docker"
)

type postgresStartupConfig struct {
	maxWait     time.Duration
	retryDelay  time.Duration
	pingTimeout time.Duration
}

var (
	lookupHost = net.LookupHost

	newPostgresPool  = pgxpool.New
	pingPostgresPool = func(ctx context.Context, db *pgxpool.Pool) error {
		return db.Ping(ctx)
	}
	closePostgresPool = func(db *pgxpool.Pool) {
		db.Close()
	}
	nowTime          = time.Now
	sleepWithContext = func(ctx context.Context, delay time.Duration) error {
		timer := time.NewTimer(delay)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return nil
		}
	}

	defaultPostgresStartupConfig = postgresStartupConfig{
		maxWait:     2 * time.Minute,
		retryDelay:  time.Second,
		pingTimeout: 3 * time.Second,
	}
)

type PostgresEnv struct {
	pool        *dockertest.Pool
	resource    *dockertest.Resource
	DB          *pgxpool.Pool
	DatabaseURL string
}

type NATSEnv struct {
	pool     *dockertest.Pool
	resource *dockertest.Resource
	Conn     *nats.Conn
	URL      string
}

func StartPostgres(ctx context.Context) (*PostgresEnv, error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, err
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository:   "postgres",
		Tag:          "16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		PortBindings: map[dc.Port][]dc.PortBinding{
			"5432/tcp": {{HostIP: "0.0.0.0", HostPort: ""}},
		},
		Labels: map[string]string{"io.ashton.test": "apollo-postgres"},
		Env: []string{
			"POSTGRES_USER=apollo",
			"POSTGRES_PASSWORD=apollo",
			"POSTGRES_DB=apollo",
		},
	}, func(hostConfig *dc.HostConfig) {
		// Docker Desktop can leave running containers permanently unbound with
		// PublishAllPorts alone. Request this ephemeral binding explicitly.
		hostConfig.PublishAllPorts = false
	})
	if err != nil {
		return nil, err
	}

	// Docker can return a snapshot before binding its random host port. An empty
	// DSN port would silently make pgx try the unrelated default port 5432.
	startupCtx, cancelStartup := context.WithTimeout(ctx, defaultPostgresStartupConfig.maxWait)
	defer cancelStartup()
	port, err := waitForPublishedPort(startupCtx, func(inspectCtx context.Context) (string, error) {
		current := resource.GetPort("5432/tcp")
		if parsed, err := strconv.Atoi(current); err == nil && parsed > 0 && parsed <= 65535 {
			return current, nil
		}
		if resource.Container == nil || resource.Container.ID == "" {
			return "", fmt.Errorf("Docker returned no container ID")
		}
		container, err := pool.Client.InspectContainerWithContext(resource.Container.ID, inspectCtx)
		if err != nil {
			return "", err
		}
		resource.Container = container
		if container.State.Status == "exited" || container.State.Dead {
			return "", fmt.Errorf("PostgreSQL container stopped before port binding (exit=%d, oom=%t)", container.State.ExitCode, container.State.OOMKilled)
		}
		return resource.GetPort("5432/tcp"), nil
	})
	if err != nil {
		_ = pool.Purge(resource)
		detail := ""
		if resource.Container != nil {
			state := resource.Container.State
			detail = fmt.Sprintf(" (state=%s, running=%t, exit=%d, oom=%t)", state.Status, state.Running, state.ExitCode, state.OOMKilled)
		}
		return nil, fmt.Errorf("PostgreSQL published port%s: %w", detail, err)
	}

	databaseURL := fmt.Sprintf(
		"postgres://apollo:apollo@%s:%s/apollo?sslmode=disable",
		dockerHost(),
		port,
	)

	db, err := waitForPostgres(startupCtx, databaseURL, defaultPostgresStartupConfig)
	if err != nil {
		_ = pool.Purge(resource)
		return nil, err
	}

	return &PostgresEnv{
		pool:        pool,
		resource:    resource,
		DB:          db,
		DatabaseURL: databaseURL,
	}, nil
}

func waitForPublishedPort(ctx context.Context, lookup func(context.Context) (string, error)) (string, error) {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		port, err := lookup(ctx)
		if err != nil {
			return "", err
		}
		parsed, err := strconv.Atoi(port)
		if err == nil && parsed > 0 && parsed <= 65535 {
			return port, nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
		}
	}
}

func waitForPostgres(ctx context.Context, databaseURL string, cfg postgresStartupConfig) (*pgxpool.Pool, error) {
	deadline := nowTime().Add(cfg.maxWait)
	var lastErr error

	for {
		db, err := newPostgresPool(ctx, databaseURL)
		if err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, cfg.pingTimeout)
			err = pingPostgresPool(pingCtx, db)
			cancel()
		}
		if err == nil {
			return db, nil
		}

		lastErr = err
		if db != nil {
			closePostgresPool(db)
		}

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !nowTime().Before(deadline) {
			return nil, fmt.Errorf("postgres did not become ready within %s: %w", cfg.maxWait, lastErr)
		}
		if err := sleepWithContext(ctx, cfg.retryDelay); err != nil {
			return nil, err
		}
	}
}

func (e *PostgresEnv) Close() error {
	if e.DB != nil {
		closePostgresPool(e.DB)
	}
	if e.pool != nil && e.resource != nil {
		return e.pool.Purge(e.resource)
	}
	return nil
}

func StartNATS() (*NATSEnv, error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, err
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "nats",
		Tag:        "2.10-alpine",
	})
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("nats://%s:%s", dockerHost(), resource.GetPort("4222/tcp"))

	var conn *nats.Conn
	if err := pool.Retry(func() error {
		conn, err = nats.Connect(url)
		return err
	}); err != nil {
		if conn != nil {
			conn.Close()
		}
		_ = pool.Purge(resource)
		return nil, err
	}

	return &NATSEnv{
		pool:     pool,
		resource: resource,
		Conn:     conn,
		URL:      url,
	}, nil
}

func (e *NATSEnv) Close() error {
	if e.Conn != nil {
		e.Conn.Close()
	}
	if e.pool != nil && e.resource != nil {
		return e.pool.Purge(e.resource)
	}
	return nil
}

func ApplySQLFiles(ctx context.Context, db *pgxpool.Pool, paths ...string) error {
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if _, err := db.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
	}

	return nil
}

func dockerHost() string {
	if host := os.Getenv("DOCKER_HOST_NAME"); host != "" {
		return host
	}

	if _, err := lookupHost("host.docker.internal"); err == nil {
		return "host.docker.internal"
	}

	return "127.0.0.1"
}
