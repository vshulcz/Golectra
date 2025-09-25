package postgres

import (
	"io/fs"
	"testing"
)

func TestEmbeddedMigrations_Present(t *testing.T) {
	entries, err := fs.ReadDir(embedMigrations, "migrations")
	if err != nil {
		t.Fatalf("cannot read embedded migrations: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no embedded migrations found")
	}

	foundInit := false
	for _, e := range entries {
		if e.Name() == "0001_init.sql" {
			foundInit = true
			break
		}
	}
	if !foundInit {
		t.Logf("warning: 0001_init.sql not found among: %v", entries)
	}
}
