package publish

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disk0Dancer/climate/internal/spec"
)

func TestEnsureLifecycleFilesCreatesManagedFiles(t *testing.T) {
	dir := t.TempDir()
	files, err := EnsureLifecycleFiles(dir, ProjectMetadata{
		CLIName:       "petstore",
		Repository:    "disk0Dancer/petstore",
		DefaultBranch: "main",
		OpenAPI: &spec.OpenAPI{
			Info: spec.Info{
				Title:       "Petstore",
				Description: "Petstore CLI docs",
			},
		},
	})
	if err != nil {
		t.Fatalf("EnsureLifecycleFiles() error = %v", err)
	}
	if len(files) != 6 {
		t.Fatalf("expected 6 managed files, got %d", len(files))
	}

	readmeData, err := os.ReadFile(filepath.Join(dir, "README.md"))
	if err != nil {
		t.Fatalf("reading README: %v", err)
	}
	if !strings.Contains(string(readmeData), markdownManagedMarker) {
		t.Fatal("README should include climate marker")
	}
	if !strings.Contains(string(readmeData), "go install github.com/disk0Dancer/petstore@latest") {
		t.Fatal("README should include install snippet")
	}

	ciData, err := os.ReadFile(filepath.Join(dir, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading ci workflow: %v", err)
	}
	if !strings.Contains(string(ciData), "workflow_dispatch:") {
		t.Fatal("CI workflow should support workflow_dispatch")
	}
	if !strings.Contains(string(ciData), "Run golangci-lint") {
		t.Fatal("CI workflow should include golangci-lint")
	}

	autoFixData, err := os.ReadFile(filepath.Join(dir, ".github", "workflows", "ci-autofix.yml"))
	if err != nil {
		t.Fatalf("reading ci autofix workflow: %v", err)
	}
	if !strings.Contains(string(autoFixData), "workflow_run:") {
		t.Fatal("CI autofix workflow should trigger from workflow_run")
	}
	if !strings.Contains(string(autoFixData), "actions/workflows/ci.yml/dispatches") {
		t.Fatal("CI autofix workflow should re-dispatch CI after pushing fixes")
	}

	golangciData, err := os.ReadFile(filepath.Join(dir, ".golangci.yml"))
	if err != nil {
		t.Fatalf("reading golangci config: %v", err)
	}
	if !strings.Contains(string(golangciData), "ineffassign") {
		t.Fatal("golangci config should include baseline linters")
	}
}

func TestEnsureLifecycleFilesPreservesUserEditedFiles(t *testing.T) {
	dir := t.TempDir()
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Custom README\n"), 0o644); err != nil {
		t.Fatalf("writing custom README: %v", err)
	}

	files, err := EnsureLifecycleFiles(dir, ProjectMetadata{
		CLIName:       "petstore",
		Repository:    "disk0Dancer/petstore",
		DefaultBranch: "main",
	})
	if err != nil {
		t.Fatalf("EnsureLifecycleFiles() error = %v", err)
	}
	for _, file := range files {
		if file == "README.md" {
			t.Fatal("README should not be overwritten when it is not climate-managed")
		}
	}
}
