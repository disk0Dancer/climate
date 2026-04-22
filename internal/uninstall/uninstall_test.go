package uninstall_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/disk0Dancer/climate/internal/completion"
	"github.com/disk0Dancer/climate/internal/manifest"
	"github.com/disk0Dancer/climate/internal/uninstall"
)

func TestDetectInstallMethodHomebrew(t *testing.T) {
	t.Parallel()

	method, resolved, err := uninstall.DetectInstallMethod(
		t.TempDir(),
		"/opt/homebrew/bin/climate",
		func(string) (string, error) {
			return "/opt/homebrew/Cellar/climate/1.2.3/bin/climate", nil
		},
	)
	if err != nil {
		t.Fatalf("DetectInstallMethod() error = %v", err)
	}
	if method != uninstall.MethodHomebrew {
		t.Fatalf("method = %q, want %q", method, uninstall.MethodHomebrew)
	}
	if resolved != "/opt/homebrew/Cellar/climate/1.2.3/bin/climate" {
		t.Fatalf("resolved = %q", resolved)
	}
}

func TestDetectInstallMethodGoInstall(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	executable := filepath.Join(home, "go", "bin", "climate")

	method, _, err := uninstall.DetectInstallMethod(home, executable, nil)
	if err != nil {
		t.Fatalf("DetectInstallMethod() error = %v", err)
	}
	if method != uninstall.MethodGoInstall {
		t.Fatalf("method = %q, want %q", method, uninstall.MethodGoInstall)
	}
}

func TestSelfFullStandalone(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	executable := filepath.Join(home, "bin", "climate")
	if err := os.MkdirAll(filepath.Dir(executable), 0o755); err != nil {
		t.Fatalf("mkdir executable dir: %v", err)
	}
	if err := os.WriteFile(executable, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}

	entryBinary := filepath.Join(home, ".climate", "bin", "petstore")
	entrySource := filepath.Join(home, "src", "petstore")
	if err := os.MkdirAll(filepath.Dir(entryBinary), 0o755); err != nil {
		t.Fatalf("mkdir generated bin dir: %v", err)
	}
	if err := os.WriteFile(entryBinary, []byte("generated"), 0o755); err != nil {
		t.Fatalf("write generated binary: %v", err)
	}
	if err := os.MkdirAll(entrySource, 0o755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(entrySource, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	mf, err := manifest.LoadFrom(filepath.Join(home, ".climate", "manifest.json"))
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	mf.Upsert(manifest.CLIEntry{
		Name:       "petstore",
		BinaryPath: entryBinary,
		SourceDir:  entrySource,
	})
	if err := mf.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if _, err := completion.Install(home, completion.ShellZsh, "darwin", func(w completion.Writer) error {
		_, writeErr := w.Write([]byte("# completion script\n"))
		return writeErr
	}); err != nil {
		t.Fatalf("completion.Install() error = %v", err)
	}

	result, err := uninstall.Self(uninstall.Options{
		Home:           home,
		GOOS:           "darwin",
		ExecutablePath: executable,
		Full:           true,
	})
	if err != nil {
		t.Fatalf("Self() error = %v", err)
	}
	if result.Mode != "full" {
		t.Fatalf("Mode = %q, want full", result.Mode)
	}
	if !result.ExecutableRemoved {
		t.Fatal("ExecutableRemoved should be true")
	}
	if len(result.GeneratedCLIs) != 1 {
		t.Fatalf("expected 1 generated CLI cleanup result, got %d", len(result.GeneratedCLIs))
	}
	if _, err := os.Stat(executable); !os.IsNotExist(err) {
		t.Fatalf("executable should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(entryBinary); !os.IsNotExist(err) {
		t.Fatalf("generated binary should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(entrySource); !os.IsNotExist(err) {
		t.Fatalf("source dir should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".climate", "manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("manifest should be removed, stat err = %v", err)
	}
}

func TestSelfHomebrewUsesBrewUninstall(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	called := false

	result, err := uninstall.Self(uninstall.Options{
		Home:           home,
		GOOS:           "darwin",
		ExecutablePath: "/opt/homebrew/bin/climate",
		EvalSymlinks: func(string) (string, error) {
			return "/opt/homebrew/Cellar/climate/1.2.3/bin/climate", nil
		},
		RunCommand: func(name string, args ...string) error {
			called = true
			if name != "brew" {
				t.Fatalf("command name = %q, want brew", name)
			}
			if len(args) != 2 || args[0] != "uninstall" || args[1] != "climate" {
				t.Fatalf("args = %#v", args)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Self() error = %v", err)
	}
	if !called {
		t.Fatal("brew uninstall should be invoked")
	}
	if result.InstallMethod != string(uninstall.MethodHomebrew) {
		t.Fatalf("InstallMethod = %q", result.InstallMethod)
	}
}
