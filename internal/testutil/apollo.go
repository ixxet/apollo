package testutil

import (
	"context"
	"path/filepath"
	"runtime"

	"github.com/jackc/pgx/v5/pgxpool"
)

func RepoFilePath(parts ...string) string {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("runtime.Caller() = false")
	}

	allParts := []string{filepath.Dir(currentFile), "..", ".."}
	allParts = append(allParts, parts...)

	return filepath.Join(allParts...)
}

func ApplyApolloSchema(ctx context.Context, db *pgxpool.Pool) error {
	return ApplySQLFiles(
		ctx,
		db,
		RepoFilePath("db", "migrations", "001_initial.up.sql"),
		RepoFilePath("db", "migrations", "002_open_visit_uniqueness.up.sql"),
		RepoFilePath("db", "migrations", "003_member_auth_and_sessions.up.sql"),
	)
}
