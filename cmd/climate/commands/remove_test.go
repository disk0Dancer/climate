package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disk0Dancer/climate/internal/manifest"
)

func TestRemoveCancelsWithoutConfirmation(t *testing.T) {
	removePurgeSources = false
	removeYes = false

	home := t.TempDir()
	t.Setenv("HOME", home)

	binaryPath := filepath.Join(home, ".climate", "bin", "petstore")
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	if err := os.WriteFile(binaryPath, []byte("bin"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	mf, err := manifest.LoadFrom(filepath.Join(home, ".climate", "manifest.json"))
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	mf.Upsert(manifest.CLIEntry{Name: "petstore", BinaryPath: binaryPath})
	if err := mf.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var stderr bytes.Buffer
	rootCmd.SetErr(&stderr)
	rootCmd.SetIn(strings.NewReader("n\n"))
	rootCmd.SetArgs([]string{"remove", "petstore"})

	raw := captureStdout(t, func() {
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	})

	var resp struct {
		Cancelled bool   `json:"cancelled"`
		Target    string `json:"target"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Cancelled {
		t.Fatal("expected cancellation response")
	}
	if _, err := os.Stat(binaryPath); err != nil {
		t.Fatalf("binary should remain after cancellation: %v", err)
	}
}

func TestRemoveDeletesGeneratedCLIWithConfirmation(t *testing.T) {
	removePurgeSources = true
	removeYes = false

	home := t.TempDir()
	t.Setenv("HOME", home)

	binaryPath := filepath.Join(home, ".climate", "bin", "petstore")
	sourceDir := filepath.Join(home, "src", "petstore")
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	if err := os.WriteFile(binaryPath, []byte("bin"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}

	mf, err := manifest.LoadFrom(filepath.Join(home, ".climate", "manifest.json"))
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	mf.Upsert(manifest.CLIEntry{Name: "petstore", BinaryPath: binaryPath, SourceDir: sourceDir})
	if err := mf.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var stderr bytes.Buffer
	rootCmd.SetErr(&stderr)
	rootCmd.SetIn(strings.NewReader("y\n"))
	rootCmd.SetArgs([]string{"remove", "--purge-sources", "petstore"})

	raw := captureStdout(t, func() {
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	})

	var resp struct {
		Removed       string `json:"removed"`
		BinaryRemoved bool   `json:"binary_removed"`
		SourceRemoved bool   `json:"source_removed"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Removed != "petstore" {
		t.Fatalf("Removed = %q, want petstore", resp.Removed)
	}
	if !resp.BinaryRemoved || !resp.SourceRemoved {
		t.Fatal("expected binary and source removal")
	}
	if _, err := os.Stat(binaryPath); !os.IsNotExist(err) {
		t.Fatalf("binary should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(sourceDir); !os.IsNotExist(err) {
		t.Fatalf("source dir should be removed, stat err = %v", err)
	}
}
