package store

import "testing"

func TestMigrationFilesDiscoverNumberedSQLInOrder(t *testing.T) {
	files, err := migrationFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 6 || files[0] != "migrations/001_initial.sql" || files[len(files)-1] != "migrations/006_privacy_allowlist.sql" {
		t.Fatalf("unexpected migrations: %#v", files)
	}
}
