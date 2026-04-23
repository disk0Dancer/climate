package publish

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disk0Dancer/climate/internal/githubutil"
)

func TestSyncGitRepositoryExistingRepoPreservesRemoteFiles(t *testing.T) {
	remoteBare := filepath.Join(t.TempDir(), "remote.git")
	if err := runGit("", "init", "--bare", remoteBare); err != nil {
		t.Fatalf("init bare repo: %v", err)
	}

	seedDir := t.TempDir()
	if err := runGit(seedDir, "init"); err != nil {
		t.Fatalf("init seed repo: %v", err)
	}
	if err := runGit(seedDir, "config", "user.name", "climate"); err != nil {
		t.Fatalf("git config user.name: %v", err)
	}
	if err := runGit(seedDir, "config", "user.email", "climate@example.test"); err != nil {
		t.Fatalf("git config user.email: %v", err)
	}
	if err := os.WriteFile(filepath.Join(seedDir, "README.md"), []byte("# Existing README\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(seedDir, ".github", "workflows"), 0o755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}
	if err := os.WriteFile(filepath.Join(seedDir, ".github", "workflows", "ci.yml"), []byte("name: CI\n"), 0o644); err != nil {
		t.Fatalf("write ci workflow: %v", err)
	}
	if err := runGit(seedDir, "add", "."); err != nil {
		t.Fatalf("git add seed: %v", err)
	}
	if err := runGit(seedDir, "commit", "-m", "seed"); err != nil {
		t.Fatalf("git commit seed: %v", err)
	}
	if err := runGit(seedDir, "branch", "-M", "main"); err != nil {
		t.Fatalf("git branch -M main: %v", err)
	}
	if err := runGit(seedDir, "remote", "add", "origin", remoteBare); err != nil {
		t.Fatalf("git remote add origin: %v", err)
	}
	if err := runGit(seedDir, "push", "-u", "origin", "main"); err != nil {
		t.Fatalf("git push seed: %v", err)
	}

	sourceDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sourceDir, "cmd"), 0o755); err != nil {
		t.Fatalf("mkdir cmd: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(sourceDir, "internal", "config"), 0o755); err != nil {
		t.Fatalf("mkdir internal/config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "go.mod"), []byte("module github\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "cmd", "config.go"), []byte("package cmd\n"), 0o644); err != nil {
		t.Fatalf("write cmd/config.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "internal", "config", "config.go"), []byte("package config\n"), 0o644); err != nil {
		t.Fatalf("write internal/config/config.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "climate_meta.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write climate_meta.json: %v", err)
	}

	repo := &githubutil.Repository{
		FullName:      "disk0Dancer/github",
		SSHURL:        remoteBare,
		DefaultBranch: "main",
	}
	files, err := syncGitRepository(sourceDir, repo, "main", ProjectMetadata{
		CLIName:       "github",
		Repository:    repo.FullName,
		DefaultBranch: "main",
	}, true)
	if err != nil {
		t.Fatalf("syncGitRepository() error = %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected lifecycle files to be written during existing repo sync")
	}

	verifyDir := t.TempDir()
	if err := runGit("", "clone", "--branch", "main", remoteBare, verifyDir); err != nil {
		t.Fatalf("clone verify repo: %v", err)
	}
	readme, err := os.ReadFile(filepath.Join(verifyDir, "README.md"))
	if err != nil {
		t.Fatalf("read README after sync: %v", err)
	}
	if !strings.Contains(string(readme), "# Existing README") {
		t.Fatal("existing README should be preserved when it is not climate-managed")
	}
	if _, err := os.Stat(filepath.Join(verifyDir, "cmd", "config.go")); err != nil {
		t.Fatalf("generated cmd/config.go should be pushed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(verifyDir, "internal", "config", "config.go")); err != nil {
		t.Fatalf("generated internal/config/config.go should be pushed: %v", err)
	}
}
