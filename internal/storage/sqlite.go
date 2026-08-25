package storage

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaFS embed.FS

func DefaultHome() (string, error) {
	if home := os.Getenv("RELAY_HOME"); home != "" {
		return home, nil
	}

	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(userHome, ".relay"), nil
}

func Open() (*sql.DB, error) {
	home, err := DefaultHome()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(home, 0o700); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(home, "relay.db")

	conn, err := OpenPath(dbPath)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(dbPath); err == nil {
		_ = os.Chmod(dbPath, 0o600)
	}
	return conn, nil
}

func OpenPath(dbPath string) (*sql.DB, error) {
	dsn := dbPath
	if dbPath != ":memory:" {
		dsn = "file:" + filepath.ToSlash(dbPath) + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	}
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// Initialization and the legacy table rebuild must stay on one connection so
	// connection-local SQLite PRAGMAs are deterministic.
	conn.SetMaxOpenConns(1)

	if _, err := conn.Exec(`
	    PRAGMA foreign_keys = ON;
		PRAGMA journal_mode = WAL;
		PRAGMA busy_timeout = 5000;
	`); err != nil {
		conn.Close()
		return nil, fmt.Errorf("configuring database: %w", err)
	}
	if err := migrateAccountRecords(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrating database: %w", err)
	}
	if err := migrateDatabaseRoute(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrating database route: %w", err)
	}

	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		conn.Close()
		return nil, err
	}

	if _, err := conn.Exec(string(schema)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("initializing database: %w", err)
	}
	return conn, nil
}

// migrateDatabaseRoute upgrades development databases created before DATABASE
// records could reference an SSH route. Fresh databases are handled by schema.sql.
func migrateDatabaseRoute(db *sql.DB) error {
	var tableName string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='rd_db_connections'`).Scan(&tableName)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}

	rows, err := db.Query(`PRAGMA table_info(rd_db_connections)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err = rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == "route_id" {
			return nil
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}

	_, err = db.Exec(`ALTER TABLE rd_db_connections ADD COLUMN route_id TEXT REFERENCES rd_ssh_routes(id) ON DELETE SET NULL`)
	return err
}

// migrateAccountRecords upgrades the one pre-NOTE development schema in place.
// SQLite cannot alter a CHECK constraint, so rd_records is rebuilt while its
// identifiers remain unchanged. Credentials are retained as soft-deleted history.
func migrateAccountRecords(db *sql.DB) error {
	var tableSQL string
	err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='rd_records'`).Scan(&tableSQL)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	upper := strings.ToUpper(tableSQL)
	if !strings.Contains(upper, "'ACCOUNT'") || strings.Contains(upper, "'NOTE'") {
		return nil
	}

	if _, err = db.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	defer func() { _, _ = db.Exec(`PRAGMA foreign_keys = ON`) }()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	rollback := func(e error) error {
		_ = tx.Rollback()
		return e
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err = tx.Exec(`
		CREATE TABLE rd_records_note_migration (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			alias TEXT NOT NULL,
			category TEXT NOT NULL CHECK (category IN ('NOTE', 'HOST', 'DATABASE')),
			notes TEXT NOT NULL DEFAULT '',
			favorite INTEGER NOT NULL DEFAULT 0 CHECK (favorite IN (0, 1)),
			deleted TEXT NOT NULL DEFAULT 'N' CHECK (deleted IN ('Y', 'N')),
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)
	`); err != nil {
		return rollback(err)
	}
	if _, err = tx.Exec(`
		INSERT INTO rd_records_note_migration(id,name,alias,category,notes,favorite,deleted,created_at,updated_at)
		SELECT id,name,CASE WHEN category='ACCOUNT' THEN '' ELSE alias END,
		       CASE WHEN category='ACCOUNT' THEN 'NOTE' ELSE category END,
		       notes,favorite,deleted,created_at,updated_at
		FROM rd_records
	`); err != nil {
		return rollback(err)
	}
	if _, err = tx.Exec(`UPDATE rd_credentials SET deleted='Y',updated_at=? WHERE deleted='N' AND record_id IN (SELECT id FROM rd_records WHERE category='ACCOUNT')`, now); err != nil {
		return rollback(err)
	}
	if _, err = tx.Exec(`DROP TABLE rd_records`); err != nil {
		return rollback(err)
	}
	if _, err = tx.Exec(`ALTER TABLE rd_records_note_migration RENAME TO rd_records`); err != nil {
		return rollback(err)
	}
	if _, err = tx.Exec(`PRAGMA user_version = 1`); err != nil {
		return rollback(err)
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if _, err = db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return err
	}
	var violations int
	rows, err := db.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		violations++
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if violations != 0 {
		return fmt.Errorf("foreign key check failed with %d violation(s)", violations)
	}
	return nil
}
