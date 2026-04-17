package manifest_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/disk0Dancer/climate/internal/manifest"
)

func TestManifestLoadSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	mf, err := manifest.LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}

	if mf.List() == nil {
		// nil slice is fine, but let's check length
	}
	if len(mf.List()) != 0 {
		t.Errorf("expected empty manifest, got %d entries", len(mf.List()))
	}

	// Upsert an entry
	mf.Upsert(manifest.CLIEntry{
		Name:        "myapi",
		BinaryPath:  "/home/user/.climate/bin/myapi",
		SourceDir:   "/home/user/.climate/src/myapi",
		Version:     "1.0.0",
		OpenAPIHash: "abc123",
		OpenAPISpec: "https://example.com/openapi.yaml",
	})

	if err := mf.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Reload and verify
	mf2, err := manifest.LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom() after save error = %v", err)
	}

	entries := mf2.List()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Name != "myapi" {
		t.Errorf("Name = %q, want %q", entry.Name, "myapi")
	}
	if entry.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", entry.Version, "1.0.0")
	}
	if entry.OpenAPIHash != "abc123" {
		t.Errorf("OpenAPIHash = %q, want %q", entry.OpenAPIHash, "abc123")
	}
	if entry.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if entry.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestManifestUpsertUpdates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	mf, _ := manifest.LoadFrom(path)

	// Insert
	mf.Upsert(manifest.CLIEntry{
		Name:    "myapi",
		Version: "1.0.0",
	})
	first := mf.List()[0]
	createdAt := first.CreatedAt

	// Small sleep to ensure timestamps differ
	time.Sleep(2 * time.Millisecond)

	// Update
	mf.Upsert(manifest.CLIEntry{
		Name:    "myapi",
		Version: "2.0.0",
	})

	entries := mf.List()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after upsert, got %d", len(entries))
	}

	updated := entries[0]
	if updated.Version != "2.0.0" {
		t.Errorf("Version after update = %q, want %q", updated.Version, "2.0.0")
	}
	if !updated.CreatedAt.Equal(createdAt) {
		t.Error("CreatedAt should not change on update")
	}
	if !updated.UpdatedAt.After(createdAt) {
		t.Error("UpdatedAt should be after CreatedAt")
	}
}

func TestManifestGet(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	mf, _ := manifest.LoadFrom(path)
	mf.Upsert(manifest.CLIEntry{Name: "myapi", Version: "1.0.0"})

	entry, ok := mf.Get("myapi")
	if !ok {
		t.Fatal("Get() should return true for existing entry")
	}
	if entry.Name != "myapi" {
		t.Errorf("Name = %q, want %q", entry.Name, "myapi")
	}

	_, ok = mf.Get("nonexistent")
	if ok {
		t.Error("Get() should return false for missing entry")
	}
}

func TestManifestRemove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	mf, _ := manifest.LoadFrom(path)
	mf.Upsert(manifest.CLIEntry{Name: "myapi"})
	mf.Upsert(manifest.CLIEntry{Name: "otherapi"})

	removed := mf.Remove("myapi")
	if !removed {
		t.Error("Remove() should return true for existing entry")
	}

	entries := mf.List()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after remove, got %d", len(entries))
	}
	if entries[0].Name != "otherapi" {
		t.Errorf("remaining entry = %q, want %q", entries[0].Name, "otherapi")
	}

	// Remove nonexistent
	removed = mf.Remove("nonexistent")
	if removed {
		t.Error("Remove() should return false for missing entry")
	}
}

func TestManifestPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	mf, err := manifest.LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}

	if mf.Path() != path {
		t.Errorf("Path() = %q, want %q", mf.Path(), path)
	}
}

func TestManifestCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	nestedPath := filepath.Join(dir, "a", "b", "c", "manifest.json")

	mf, _ := manifest.LoadFrom(nestedPath)
	mf.Upsert(manifest.CLIEntry{Name: "test"})

	if err := mf.Save(); err != nil {
		t.Fatalf("Save() to nested path error = %v", err)
	}

	if _, err := os.Stat(nestedPath); err != nil {
		t.Errorf("manifest file not created: %v", err)
	}
}
