package storage_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/dzaneyo/relay/internal/storage"
	_ "modernc.org/sqlite"
)

func TestDatabaseRouteColumnMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-route.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE rd_ssh_routes (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			deleted TEXT NOT NULL DEFAULT 'N',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE TABLE rd_db_connections (
			record_id TEXT PRIMARY KEY,
			db_type TEXT NOT NULL,
			host TEXT NOT NULL,
			port INTEGER NOT NULL,
			database_name TEXT,
			credential_id TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}

	migrated, err := storage.OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.Close()

	rows, err := migrated.Query(`PRAGMA table_info(rd_db_connections)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err = rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			t.Fatal(err)
		}
		if name == "route_id" {
			found = true
		}
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("route_id column was not added")
	}
}
