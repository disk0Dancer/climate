package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disk0Dancer/climate/internal/completion"
	"github.com/disk0Dancer/climate/internal/manifest"
)

func TestUninstallFullStandalone(t *testing.T) {
	uninstallFull = false
	uninstallYes = false

	originalExecutablePath := uninstallExecutablePath
	originalEvalSymlinks := uninstallEvalSymlinks
	originalRunner := uninstallCommandRunner
	defer func() {
		uninstallExecutablePath = originalExecutablePath
		uninstallEvalSymlinks = originalEvalSymlinks
		uninstallCommandRunner = originalRunner
	}()

	home := t.TempDir()
	t.Setenv("HOME", home)

	executable := filepath.Join(home, "bin", "climate")
	if err := os.MkdirAll(filepath.Dir(executable), 0o755); err != nil {
		t.Fatalf("mkdir executable dir: %v", err)
	}
	if err := os.WriteFile(executable, []byte("binary"), 0o755); err != nil {
		t.Fatalf("write executable: %v", err)
	}

	generatedBinary := filepath.Join(home, ".climate", "bin", "petstore")
	generatedSource := filepath.Join(home, "src", "petstore")
	if err := os.MkdirAll(filepath.Dir(generatedBinary), 0o755); err != nil {
		t.Fatalf("mkdir generated binary dir: %v", err)
	}
	if err := os.WriteFile(generatedBinary, []byte("generated"), 0o755); err != nil {
		t.Fatalf("write generated binary: %v", err)
	}
	if err := os.MkdirAll(generatedSource, 0o755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}

	mf, err := manifest.LoadFrom(filepath.Join(home, ".climate", "manifest.json"))
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	mf.Upsert(manifest.CLIEntry{Name: "petstore", BinaryPath: generatedBinary, SourceDir: generatedSource})
	if err := mf.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if _, err := completion.Install(home, completion.ShellZsh, "darwin", func(w completion.Writer) error {
		_, writeErr := w.Write([]byte("# completion script\n"))
		return writeErr
	}); err != nil {
		t.Fatalf("completion.Install() error = %v", err)
	}

	uninstallExecutablePath = func() (string, error) { return executable, nil }
	uninstallEvalSymlinks = func(path string) (string, error) { return path, nil }
	uninstallCommandRunner = func(io.Writer, io.Writer, string, ...string) error { return nil }

	var stderr bytes.Buffer
	rootCmd.SetErr(&stderr)
	rootCmd.SetIn(strings.NewReader("y\n"))
	rootCmd.SetArgs([]string{"uninstall", "--full"})

	raw := captureStdout(t, func() {
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	})

	var resp struct {
		Mode              string `json:"mode"`
		InstallMethod     string `json:"install_method"`
		ExecutableRemoved bool   `json:"executable_removed"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Mode != "full" {
		t.Fatalf("Mode = %q, want full", resp.Mode)
	}
	if resp.InstallMethod != "standalone" {
		t.Fatalf("InstallMethod = %q, want standalone", resp.InstallMethod)
	}
	if !resp.ExecutableRemoved {
		t.Fatal("executable_removed should be true")
	}
	if _, err := os.Stat(executable); !os.IsNotExist(err) {
		t.Fatalf("executable should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(generatedBinary); !os.IsNotExist(err) {
		t.Fatalf("generated binary should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(generatedSource); !os.IsNotExist(err) {
		t.Fatalf("generated source should be removed, stat err = %v", err)
	}
}
