package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletionZshWritesScriptAndTip(t *testing.T) {
	completionInstallShell = ""
	completionUninstallShell = ""

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"completion", "zsh"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if stdout.Len() == 0 {
		t.Fatal("completion script should be written to stdout")
	}
	if !strings.Contains(stdout.String(), "climate") {
		t.Fatal("completion output should mention the climate command")
	}
	if !strings.Contains(stderr.String(), "completion install --shell zsh") {
		t.Fatal("stderr should include install tip")
	}
}

func TestCompletionInstallAndUninstall(t *testing.T) {
	completionInstallShell = ""
	completionUninstallShell = ""

	home := t.TempDir()
	t.Setenv("HOME", home)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"completion", "install", "--shell", "zsh"})

	rawInstall := captureStdout(t, func() {
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("install Execute() error = %v", err)
		}
	})

	var installResp struct {
		Shell      string `json:"shell"`
		ScriptPath string `json:"script_path"`
		ConfigPath string `json:"config_path"`
	}
	if err := json.Unmarshal([]byte(rawInstall), &installResp); err != nil {
		t.Fatalf("unmarshal install response: %v", err)
	}
	if installResp.Shell != "zsh" {
		t.Fatalf("Shell = %q, want zsh", installResp.Shell)
	}
	if _, err := os.Stat(installResp.ScriptPath); err != nil {
		t.Fatalf("installed script missing: %v", err)
	}
	configBytes, err := os.ReadFile(filepath.Join(home, ".zshrc"))
	if err != nil {
		t.Fatalf("reading .zshrc: %v", err)
	}
	if !strings.Contains(string(configBytes), installResp.ScriptPath) {
		t.Fatal(".zshrc should source installed completion script")
	}

	stdout.Reset()
	stderr.Reset()
	rootCmd.SetArgs([]string{"completion", "uninstall", "--shell", "zsh"})

	rawUninstall := captureStdout(t, func() {
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("uninstall Execute() error = %v", err)
		}
	})

	var uninstallResp struct {
		ScriptRemoved bool `json:"script_removed"`
	}
	if err := json.Unmarshal([]byte(rawUninstall), &uninstallResp); err != nil {
		t.Fatalf("unmarshal uninstall response: %v", err)
	}
	if !uninstallResp.ScriptRemoved {
		t.Fatal("script_removed should be true after uninstall")
	}

	configBytes, err = os.ReadFile(filepath.Join(home, ".zshrc"))
	if err != nil {
		t.Fatalf("reading .zshrc after uninstall: %v", err)
	}
	if strings.Contains(string(configBytes), "climate completion") {
		t.Fatal(".zshrc should not contain climate-managed completion block after uninstall")
	}
}
