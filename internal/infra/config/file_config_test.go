package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadConfigFile_ReadsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.json")
	payload := []byte(`{"ok":true}`)
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	got, err := readConfigFile(path)
	if err != nil {
		t.Fatalf("readConfigFile error: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("readConfigFile=%q want %q", string(got), string(payload))
	}
}

func TestReadConfigFile_EmptyPath(t *testing.T) {
	if _, err := readConfigFile("   "); err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestReadConfigFile_InvalidPath(t *testing.T) {
	if _, err := readConfigFile("/"); err == nil {
		t.Fatal("expected error for invalid path")
	}
}
