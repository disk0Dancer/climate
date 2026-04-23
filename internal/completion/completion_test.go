package completion_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disk0Dancer/climate/internal/completion"
)

func TestDetectShell(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		value   string
		want    completion.Shell
		wantErr bool
	}{
		{name: "bash", value: "/bin/bash", want: completion.ShellBash},
		{name: "zsh", value: "/bin/zsh", want: completion.ShellZsh},
		{name: "fish", value: "/opt/homebrew/bin/fish", want: completion.ShellFish},
		{name: "pwsh", value: "/usr/local/bin/pwsh", want: completion.ShellPowerShell},
		{name: "empty", value: "", wantErr: true},
		{name: "unsupported", value: "/bin/tcsh", wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := completion.DetectShell(tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatal("DetectShell() error = nil, want non-nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("DetectShell() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("DetectShell() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolvePaths(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	bashrc := filepath.Join(home, ".bashrc")
	if err := os.WriteFile(bashrc, []byte("# bash\n"), 0o644); err != nil {
		t.Fatalf("writing .bashrc: %v", err)
	}

	cases := []struct {
		name           string
		shell          completion.Shell
		goos           string
		wantScriptPath string
		wantConfigPath string
	}{
		{
			name:           "bash",
			shell:          completion.ShellBash,
			goos:           "linux",
			wantScriptPath: filepath.Join(home, ".climate", "completions", "climate.bash"),
			wantConfigPath: bashrc,
		},
		{
			name:           "zsh",
			shell:          completion.ShellZsh,
			goos:           "darwin",
			wantScriptPath: filepath.Join(home, ".climate", "completions", "climate.zsh"),
			wantConfigPath: filepath.Join(home, ".zshrc"),
		},
		{
			name:           "fish",
			shell:          completion.ShellFish,
			goos:           "darwin",
			wantScriptPath: filepath.Join(home, ".config", "fish", "completions", "climate.fish"),
			wantConfigPath: "",
		},
		{
			name:           "powershell",
			shell:          completion.ShellPowerShell,
			goos:           "linux",
			wantScriptPath: filepath.Join(home, ".climate", "completions", "climate.ps1"),
			wantConfigPath: filepath.Join(home, ".config", "powershell", "Microsoft.PowerShell_profile.ps1"),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := completion.ResolvePaths(home, tc.shell, tc.goos)
			if err != nil {
				t.Fatalf("ResolvePaths() error = %v", err)
			}
			if got.ScriptPath != tc.wantScriptPath {
				t.Fatalf("ScriptPath = %q, want %q", got.ScriptPath, tc.wantScriptPath)
			}
			if got.ConfigPath != tc.wantConfigPath {
				t.Fatalf("ConfigPath = %q, want %q", got.ConfigPath, tc.wantConfigPath)
			}
		})
	}
}

func TestResolvePathsUsesBashProfileOnDarwinWithoutBashrc(t *testing.T) {
	t.Parallel()

	home := t.TempDir()

	got, err := completion.ResolvePaths(home, completion.ShellBash, "darwin")
	if err != nil {
		t.Fatalf("ResolvePaths() error = %v", err)
	}
	want := filepath.Join(home, ".bash_profile")
	if got.ConfigPath != want {
		t.Fatalf("ConfigPath = %q, want %q", got.ConfigPath, want)
	}
}

func TestInstallAddsManagedBlock(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	configPath := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(configPath, []byte("export PATH=\"$HOME/bin:$PATH\"\n"), 0o644); err != nil {
		t.Fatalf("writing .zshrc: %v", err)
	}

	result, err := completion.Install(home, completion.ShellZsh, "darwin", func(w completion.Writer) error {
		_, writeErr := w.Write([]byte("# completion script\n"))
		return writeErr
	})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if result.ScriptPath == "" {
		t.Fatal("ScriptPath should not be empty")
	}

	scriptBytes, err := os.ReadFile(result.ScriptPath)
	if err != nil {
		t.Fatalf("reading script: %v", err)
	}
	if string(scriptBytes) != "# completion script\n" {
		t.Fatalf("script content = %q", string(scriptBytes))
	}

	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	config := string(configBytes)
	if !strings.Contains(config, "# >>> climate completion >>>") {
		t.Fatal("managed block start marker missing")
	}
	if strings.Count(config, "# >>> climate completion >>>") != 1 {
		t.Fatal("managed block should be added exactly once")
	}
	if !strings.Contains(config, result.ScriptPath) {
		t.Fatal("config should source the generated script path")
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	t.Parallel()

	home := t.TempDir()

	for i := 0; i < 2; i++ {
		_, err := completion.Install(home, completion.ShellZsh, "darwin", func(w completion.Writer) error {
			_, writeErr := w.Write([]byte("# completion script\n"))
			return writeErr
		})
		if err != nil {
			t.Fatalf("Install() run %d error = %v", i+1, err)
		}
	}

	configBytes, err := os.ReadFile(filepath.Join(home, ".zshrc"))
	if err != nil {
		t.Fatalf("reading .zshrc: %v", err)
	}
	if strings.Count(string(configBytes), "# >>> climate completion >>>") != 1 {
		t.Fatal("managed block should remain singular after repeated install")
	}
}

func TestUninstallRemovesManagedAssets(t *testing.T) {
	t.Parallel()

	home := t.TempDir()

	installResult, err := completion.Install(home, completion.ShellZsh, "darwin", func(w completion.Writer) error {
		_, writeErr := w.Write([]byte("# completion script\n"))
		return writeErr
	})
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	uninstallResult, err := completion.Uninstall(home, completion.ShellZsh, "darwin")
	if err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if !uninstallResult.ScriptRemoved {
		t.Fatal("ScriptRemoved should be true after uninstall")
	}

	if _, err := os.Stat(installResult.ScriptPath); !os.IsNotExist(err) {
		t.Fatalf("script should be removed, stat err = %v", err)
	}

	configBytes, err := os.ReadFile(filepath.Join(home, ".zshrc"))
	if err != nil {
		t.Fatalf("reading .zshrc: %v", err)
	}
	config := string(configBytes)
	if strings.Contains(config, "# >>> climate completion >>>") {
		t.Fatal("managed block should be removed from config")
	}
}

func TestUninstallIsSafeWhenNothingExists(t *testing.T) {
	t.Parallel()

	home := t.TempDir()

	result, err := completion.Uninstall(home, completion.ShellFish, "darwin")
	if err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if result.ScriptRemoved {
		t.Fatal("ScriptRemoved should be false when nothing was installed")
	}
	if result.ConfigUpdated {
		t.Fatal("ConfigUpdated should be false for fish with no config file")
	}
}
