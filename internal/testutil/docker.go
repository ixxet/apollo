package testutil

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/ory/dockertest/v3"
)

var lookupHost = net.LookupHost

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
		Repository: "postgres",
		Tag:        "16-alpine",
		Env: []string{
			"POSTGRES_USER=apollo",
			"POSTGRES_PASSWORD=apollo",
			"POSTGRES_DB=apollo",
		},
	})
	if err != nil {
		return nil, err
	}

	databaseURL := fmt.Sprintf(
		"postgres://apollo:apollo@%s:%s/apollo?sslmode=disable",
		dockerHost(),
		resource.GetPort("5432/tcp"),
	)

	var db *pgxpool.Pool
	if err := pool.Retry(func() error {
		db, err = pgxpool.New(ctx, databaseURL)
		if err != nil {
			return err
		}

		return db.Ping(ctx)
	}); err != nil {
		if db != nil {
			db.Close()
		}
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

func (e *PostgresEnv) Close() error {
	if e.DB != nil {
		e.DB.Close()
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
