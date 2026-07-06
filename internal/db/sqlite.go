// Package db manages the two SQLite databases behind Yoshi Pi-hole:
// gravity.db (blocklists/domains/groups/clients) and queries.db (query log).
package db

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed migrations/gravity_schema.sql
var gravitySchema string

//go:embed migrations/queries_schema.sql
var queriesSchema string

// Open opens (creating if needed) both SQLite databases under dataDir and
// applies their schemas. Callers own the returned *sql.DB pair and must
// Close() them on shutdown.
func Open(dataDir string) (gravityDB *sql.DB, queriesDB *sql.DB, err error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("creating data dir %s: %w", dataDir, err)
	}

	gravityDB, err = openOne(filepath.Join(dataDir, "gravity.db"), gravitySchema)
	if err != nil {
		return nil, nil, fmt.Errorf("opening gravity.db: %w", err)
	}

	queriesDB, err = openOne(filepath.Join(dataDir, "queries.db"), queriesSchema)
	if err != nil {
		gravityDB.Close()
		return nil, nil, fmt.Errorf("opening queries.db: %w", err)
	}

	return gravityDB, queriesDB, nil
}

func openOne(path, schema string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", path)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	// Gravity rebuilds are large single-writer transactions and the query log
	// writer batches inserts, so a single physical connection per DB avoids
	// SQLITE_BUSY without needing a connection pool.
	sqlDB.SetMaxOpenConns(1)

	if _, err := sqlDB.Exec(schema); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("applying schema: %w", err)
	}

	return sqlDB, nil
}
