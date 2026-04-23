package completion

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Writer is the output surface used when generating completion scripts.
type Writer = io.Writer

// Generator renders a completion script into the provided writer.
type Generator func(Writer) error

// Shell identifies a supported completion shell.
type Shell string

const (
	ShellBash       Shell = "bash"
	ShellZsh        Shell = "zsh"
	ShellFish       Shell = "fish"
	ShellPowerShell Shell = "powershell"
)

const (
	managedStart = "# >>> climate completion >>>"
	managedEnd   = "# <<< climate completion <<<"
)

// Paths describes where climate stores completion assets for a shell.
type Paths struct {
	Shell      string `json:"shell"`
	ScriptPath string `json:"script_path"`
	ConfigPath string `json:"config_path,omitempty"`
}

// InstallResult reports what climate wrote during completion install.
type InstallResult struct {
	Paths
	ConfigUpdated bool `json:"config_updated"`
}

// UninstallResult reports what climate removed during completion uninstall.
type UninstallResult struct {
	Paths
	ScriptRemoved bool `json:"script_removed"`
	ConfigUpdated bool `json:"config_updated"`
}

// SupportedShellNames returns the shells supported by climate completion.
func SupportedShellNames() []string {
	return []string{
		string(ShellBash),
		string(ShellZsh),
		string(ShellFish),
		string(ShellPowerShell),
	}
}

// ParseShell validates a shell name.
func ParseShell(name string) (Shell, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case string(ShellBash):
		return ShellBash, nil
	case string(ShellZsh):
		return ShellZsh, nil
	case string(ShellFish):
		return ShellFish, nil
	case "pwsh", "powershell":
		return ShellPowerShell, nil
	default:
		return "", fmt.Errorf("unsupported shell %q (supported: %s)", name, strings.Join(SupportedShellNames(), ", "))
	}
}

// DetectShell derives the active shell from a SHELL-style environment value.
func DetectShell(shellEnv string) (Shell, error) {
	if strings.TrimSpace(shellEnv) == "" {
		return "", errors.New("shell could not be detected automatically; pass --shell")
	}
	return ParseShell(filepath.Base(shellEnv))
}

// ResolveShell returns the explicitly requested shell or auto-detects it.
func ResolveShell(explicit, shellEnv, goos string) (Shell, error) {
	if strings.TrimSpace(explicit) != "" {
		return ParseShell(explicit)
	}
	if goos == "windows" && strings.TrimSpace(shellEnv) == "" {
		return ShellPowerShell, nil
	}
	return DetectShell(shellEnv)
}

// ResolvePaths returns the script and config targets for a shell.
func ResolvePaths(home string, shell Shell, goos string) (Paths, error) {
	base := filepath.Join(home, ".climate", "completions")
	switch shell {
	case ShellBash:
		return Paths{
			Shell:      string(shell),
			ScriptPath: filepath.Join(base, "climate.bash"),
			ConfigPath: resolveBashConfigPath(home, goos),
		}, nil
	case ShellZsh:
		return Paths{
			Shell:      string(shell),
			ScriptPath: filepath.Join(base, "climate.zsh"),
			ConfigPath: filepath.Join(home, ".zshrc"),
		}, nil
	case ShellFish:
		return Paths{
			Shell:      string(shell),
			ScriptPath: filepath.Join(home, ".config", "fish", "completions", "climate.fish"),
		}, nil
	case ShellPowerShell:
		return Paths{
			Shell:      string(shell),
			ScriptPath: filepath.Join(base, "climate.ps1"),
			ConfigPath: resolvePowerShellProfilePath(home, goos),
		}, nil
	default:
		return Paths{}, fmt.Errorf("unsupported shell %q", shell)
	}
}

// Install writes the completion script and managed config block.
func Install(home string, shell Shell, goos string, generate Generator) (InstallResult, error) {
	paths, err := ResolvePaths(home, shell, goos)
	if err != nil {
		return InstallResult{}, err
	}

	if err := os.MkdirAll(filepath.Dir(paths.ScriptPath), 0o755); err != nil {
		return InstallResult{}, fmt.Errorf("creating completion directory: %w", err)
	}

	file, err := os.Create(paths.ScriptPath)
	if err != nil {
		return InstallResult{}, fmt.Errorf("creating completion script: %w", err)
	}
	defer file.Close()

	if err := generate(file); err != nil {
		return InstallResult{}, fmt.Errorf("generating completion script: %w", err)
	}

	configUpdated := false
	if paths.ConfigPath != "" {
		configUpdated, err = ensureManagedBlock(paths.ConfigPath, managedBlock(paths))
		if err != nil {
			return InstallResult{}, err
		}
	}

	return InstallResult{
		Paths:         paths,
		ConfigUpdated: configUpdated,
	}, nil
}

// Uninstall removes the completion script and managed config block.
func Uninstall(home string, shell Shell, goos string) (UninstallResult, error) {
	paths, err := ResolvePaths(home, shell, goos)
	if err != nil {
		return UninstallResult{}, err
	}

	scriptRemoved := false
	if err := os.Remove(paths.ScriptPath); err == nil {
		scriptRemoved = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return UninstallResult{}, fmt.Errorf("removing completion script: %w", err)
	}

	configUpdated := false
	if paths.ConfigPath != "" {
		configUpdated, err = removeManagedBlock(paths.ConfigPath)
		if err != nil {
			return UninstallResult{}, err
		}
	}

	return UninstallResult{
		Paths:         paths,
		ScriptRemoved: scriptRemoved,
		ConfigUpdated: configUpdated,
	}, nil
}

func resolveBashConfigPath(home, goos string) string {
	bashrc := filepath.Join(home, ".bashrc")
	if goos != "darwin" || fileExists(bashrc) {
		return bashrc
	}
	return filepath.Join(home, ".bash_profile")
}

func resolvePowerShellProfilePath(home, goos string) string {
	if goos == "windows" {
		return filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
	}
	return filepath.Join(home, ".config", "powershell", "Microsoft.PowerShell_profile.ps1")
}

func managedBlock(paths Paths) string {
	switch paths.Shell {
	case string(ShellPowerShell):
		return strings.Join([]string{
			managedStart,
			fmt.Sprintf("if (Test-Path \"%s\") { . \"%s\" }", paths.ScriptPath, paths.ScriptPath),
			managedEnd,
		}, "\n")
	default:
		return strings.Join([]string{
			managedStart,
			fmt.Sprintf("[ -f \"%s\" ] && . \"%s\"", paths.ScriptPath, paths.ScriptPath),
			managedEnd,
		}, "\n")
	}
}

func ensureManagedBlock(path, block string) (bool, error) {
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("reading shell config %s: %w", path, err)
	}

	updated := upsertManagedBlock(string(existing), block)
	if string(existing) == updated {
		return false, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("creating shell config directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return false, fmt.Errorf("writing shell config %s: %w", path, err)
	}
	return true, nil
}

func removeManagedBlock(path string) (bool, error) {
	existing, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reading shell config %s: %w", path, err)
	}

	updated, changed := stripManagedBlock(string(existing))
	if !changed {
		return false, nil
	}

	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return false, fmt.Errorf("writing shell config %s: %w", path, err)
	}
	return true, nil
}

func upsertManagedBlock(existing, block string) string {
	stripped, _ := stripManagedBlock(existing)
	block = strings.TrimRight(block, "\n")
	base := strings.TrimRight(stripped, "\n")
	if base == "" {
		return block + "\n"
	}
	return base + "\n\n" + block + "\n"
}

func stripManagedBlock(existing string) (string, bool) {
	start := strings.Index(existing, managedStart)
	if start == -1 {
		return normalizeContent(existing), false
	}

	endOffset := strings.Index(existing[start:], managedEnd)
	if endOffset == -1 {
		return normalizeContent(existing), false
	}

	end := start + endOffset + len(managedEnd)
	for end < len(existing) && (existing[end] == '\n' || existing[end] == '\r') {
		end++
	}

	before := strings.TrimRight(existing[:start], "\r\n")
	after := strings.TrimLeft(existing[end:], "\r\n")
	switch {
	case before == "" && after == "":
		return "", true
	case before == "":
		return normalizeContent(after), true
	case after == "":
		return normalizeContent(before), true
	default:
		return normalizeContent(before + "\n\n" + after), true
	}
}

func normalizeContent(content string) string {
	content = strings.TrimRight(content, "\r\n")
	if content == "" {
		return ""
	}
	return content + "\n"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
