package migrations

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const ensureSchemaMigrationsSQL = `
CREATE SCHEMA IF NOT EXISTS apollo;

CREATE TABLE IF NOT EXISTS apollo.schema_migrations (
    name TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

//go:embed *.up.sql
var migrationFiles embed.FS

type Migration struct {
	Name string
	SQL  string
}

func List() ([]Migration, error) {
	names, err := fs.Glob(migrationFiles, "*.up.sql")
	if err != nil {
		return nil, fmt.Errorf("list migration files: %w", err)
	}

	sort.Strings(names)

	migrations := make([]Migration, 0, len(names))
	for _, name := range names {
		content, err := migrationFiles.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", name, err)
		}

		migrations = append(migrations, Migration{
			Name: name,
			SQL:  strings.TrimSpace(string(content)),
		})
	}

	return migrations, nil
}

func ApplyAll(ctx context.Context, db *pgxpool.Pool) error {
	if _, err := db.Exec(ctx, ensureSchemaMigrationsSQL); err != nil {
		return fmt.Errorf("ensure schema migrations table: %w", err)
	}

	applied, err := appliedSet(ctx, db)
	if err != nil {
		return err
	}

	migrations, err := List()
	if err != nil {
		return err
	}

	for _, migration := range migrations {
		if _, ok := applied[migration.Name]; ok {
			continue
		}

		if err := applyOne(ctx, db, migration); err != nil {
			return fmt.Errorf("apply %s: %w", migration.Name, err)
		}
	}

	return nil
}

func appliedSet(ctx context.Context, db *pgxpool.Pool) (map[string]struct{}, error) {
	rows, err := db.Query(ctx, `SELECT name FROM apollo.schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]struct{})
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}

		applied[name] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}

	return applied, nil
}

func applyOne(ctx context.Context, db *pgxpool.Pool, migration Migration) (err error) {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if err == nil {
			return
		}

		_ = tx.Rollback(ctx)
	}()

	if _, err = tx.Exec(ctx, migration.SQL); err != nil {
		return fmt.Errorf("execute migration SQL: %w", err)
	}

	if _, err = tx.Exec(ctx, `INSERT INTO apollo.schema_migrations (name) VALUES ($1)`, migration.Name); err != nil {
		return fmt.Errorf("record applied migration: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
