package storage_test

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dzaneyo/relay/internal/storage"
	_ "modernc.org/sqlite"
)

func TestLegacyAccountSchemaMigratesToNote(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = legacy.Exec(`
		PRAGMA foreign_keys=ON;
		CREATE TABLE rd_records (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, alias TEXT NOT NULL,
			category TEXT NOT NULL CHECK (category IN ('ACCOUNT','HOST','DATABASE')),
			notes TEXT NOT NULL DEFAULT '', favorite INTEGER NOT NULL DEFAULT 0,
			deleted TEXT NOT NULL DEFAULT 'N', created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		);
		CREATE UNIQUE INDEX uk_rd_records_alias ON rd_records(alias COLLATE NOCASE) WHERE deleted='N';
		CREATE TABLE rd_credentials (
			id TEXT PRIMARY KEY, record_id TEXT NOT NULL, label TEXT NOT NULL DEFAULT 'default',
			username TEXT, auth_type TEXT NOT NULL, secret_value TEXT, key_path TEXT,
			deleted TEXT NOT NULL DEFAULT 'N', created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			FOREIGN KEY(record_id) REFERENCES rd_records(id) ON DELETE CASCADE
		);
		INSERT INTO rd_records VALUES('account-1','Old account','old-account','ACCOUNT','legacy',0,'N','now','now');
		INSERT INTO rd_credentials VALUES('credential-1','account-1','default','user','PASSWORD','secret',NULL,'N','now','now');
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err = legacy.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := storage.OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var category, alias, credentialDeleted string
	if err = db.QueryRow(`SELECT category,alias FROM rd_records WHERE id='account-1'`).Scan(&category, &alias); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT deleted FROM rd_credentials WHERE id='credential-1'`).Scan(&credentialDeleted); err != nil {
		t.Fatal(err)
	}
	if category != "NOTE" || alias != "" || credentialDeleted != "Y" {
		t.Fatalf("unexpected migration result: category=%q alias=%q credentialDeleted=%q", category, alias, credentialDeleted)
	}
	if _, err = db.Exec(`INSERT INTO rd_records VALUES('note-2','Another note','','NOTE','',0,'N','now','now')`); err != nil {
		t.Fatalf("multiple NOTE rows with empty aliases must be allowed: %v", err)
	}
	var indexSQL string
	if err = db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='index' AND name='uk_rd_records_alias'`).Scan(&indexSQL); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToUpper(indexSQL), "CATEGORY <> 'NOTE'") {
		t.Fatalf("alias index does not exclude NOTE: %s", indexSQL)
	}
	rows, err := db.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("migration left a foreign key violation")
	}
}
