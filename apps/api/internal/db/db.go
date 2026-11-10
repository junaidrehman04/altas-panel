package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open opens (and creates) a SQLite database, enables foreign keys and WAL,
// and applies the auth schema.
func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	dsn := fmt.Sprintf(
		"file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_txlock=immediate",
		filepath.ToSlash(path),
	)
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	if _, err := sqlDB.Exec(schema); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	if err := ensureAdminColumn(sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	return sqlDB, nil
}

// ensureAdminColumn adds is_admin to databases created before the first-admin
// gate and marks the earliest account as the panel admin.
func ensureAdminColumn(sqlDB *sql.DB) error {
	rows, err := sqlDB.Query(`PRAGMA table_info(users)`)
	if err != nil {
		return fmt.Errorf("inspect users: %w", err)
	}
	defer rows.Close()

	found := false
	for rows.Next() {
		var cid, notnull, pk int
		var name, colType string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notnull, &dflt, &pk); err != nil {
			return fmt.Errorf("inspect users: %w", err)
		}
		if name == "is_admin" {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("inspect users: %w", err)
	}
	if !found {
		if _, err := sqlDB.Exec(`ALTER TABLE users ADD COLUMN is_admin INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("add is_admin: %w", err)
		}
	}
	_, err = sqlDB.Exec(`
		UPDATE users
		SET is_admin = 1
		WHERE NOT EXISTS (SELECT 1 FROM users WHERE is_admin = 1)
		  AND id = (
		    SELECT id FROM users
		    ORDER BY created_at ASC, rowid ASC
		    LIMIT 1
		  )
	`)
	if err != nil {
		return fmt.Errorf("promote first admin: %w", err)
	}
	return nil
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  is_admin INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
  token_hash TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  revoked_at TEXT
);

CREATE INDEX IF NOT EXISTS sessions_user_id_idx ON sessions(user_id);

CREATE TABLE IF NOT EXISTS password_resets (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  used_at TEXT
);

CREATE TABLE IF NOT EXISTS host_metric_buckets (
  resolution TEXT NOT NULL,
  bucket_start INTEGER NOT NULL,
  cpu_sum REAL NOT NULL DEFAULT 0,
  cpu_samples INTEGER NOT NULL DEFAULT 0,
  mem_pct_sum REAL NOT NULL DEFAULT 0,
  mem_used_sum INTEGER NOT NULL DEFAULT 0,
  mem_total INTEGER NOT NULL DEFAULT 0,
  disk_pct_sum REAL NOT NULL DEFAULT 0,
  disk_used_sum INTEGER NOT NULL DEFAULT 0,
  disk_total INTEGER NOT NULL DEFAULT 0,
  disk_samples INTEGER NOT NULL DEFAULT 0,
  rx_sum REAL NOT NULL DEFAULT 0,
  tx_sum REAL NOT NULL DEFAULT 0,
  net_samples INTEGER NOT NULL DEFAULT 0,
  samples INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (resolution, bucket_start)
);

CREATE TABLE IF NOT EXISTS analytics_meta (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS analytics_buckets (
  resolution TEXT NOT NULL,
  bucket_start INTEGER NOT NULL,
  project TEXT NOT NULL,
  requests INTEGER NOT NULL DEFAULT 0,
  errors INTEGER NOT NULL DEFAULT 0,
  bytes INTEGER NOT NULL DEFAULT 0,
  b0 INTEGER NOT NULL DEFAULT 0,
  b1 INTEGER NOT NULL DEFAULT 0,
  b2 INTEGER NOT NULL DEFAULT 0,
  b3 INTEGER NOT NULL DEFAULT 0,
  b4 INTEGER NOT NULL DEFAULT 0,
  b5 INTEGER NOT NULL DEFAULT 0,
  b6 INTEGER NOT NULL DEFAULT 0,
  b7 INTEGER NOT NULL DEFAULT 0,
  b8 INTEGER NOT NULL DEFAULT 0,
  b9 INTEGER NOT NULL DEFAULT 0,
  b10 INTEGER NOT NULL DEFAULT 0,
  b11 INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (resolution, bucket_start, project)
);

CREATE TABLE IF NOT EXISTS analytics_dims (
  resolution TEXT NOT NULL,
  bucket_start INTEGER NOT NULL,
  project TEXT NOT NULL,
  kind TEXT NOT NULL,
  name TEXT NOT NULL,
  requests INTEGER NOT NULL DEFAULT 0,
  errors INTEGER NOT NULL DEFAULT 0,
  bytes INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (resolution, bucket_start, project, kind, name)
);

CREATE INDEX IF NOT EXISTS analytics_dims_kind_idx
  ON analytics_dims (resolution, kind, bucket_start);

CREATE TABLE IF NOT EXISTS analytics_visitors (
  scope TEXT NOT NULL,
  bucket_start INTEGER NOT NULL,
  project TEXT NOT NULL,
  visitor_hash TEXT NOT NULL,
  PRIMARY KEY (scope, bucket_start, project, visitor_hash)
);

CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL
);
`
