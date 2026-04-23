package uninstall

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/disk0Dancer/climate/internal/completion"
	"github.com/disk0Dancer/climate/internal/manifest"
)

// InstallMethod describes how the climate binary was installed.
type InstallMethod string

const (
	MethodHomebrew   InstallMethod = "homebrew"
	MethodGoInstall  InstallMethod = "go-install"
	MethodStandalone InstallMethod = "standalone"
)

// RunCommand executes an external command.
type RunCommand func(name string, args ...string) error

// Options controls self-uninstall behavior.
type Options struct {
	Home           string
	GOOS           string
	ExecutablePath string
	Full           bool
	EvalSymlinks   func(string) (string, error)
	RunCommand     RunCommand
}

// GeneratedCLIResult describes cleanup work for one generated CLI.
type GeneratedCLIResult struct {
	Name          string   `json:"name"`
	BinaryPath    string   `json:"binary_path,omitempty"`
	BinaryRemoved bool     `json:"binary_removed"`
	SourceDir     string   `json:"source_dir,omitempty"`
	SourceRemoved bool     `json:"source_removed,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
}

// Result describes self-uninstall work.
type Result struct {
	InstallMethod     string                       `json:"install_method"`
	Mode              string                       `json:"mode"`
	ExecutablePath    string                       `json:"executable_path"`
	ExecutableRemoved bool                         `json:"executable_removed"`
	GeneratedCLIs     []GeneratedCLIResult         `json:"generated_clis,omitempty"`
	CompletionCleanup []completion.UninstallResult `json:"completion_cleanup,omitempty"`
	ManifestRemoved   bool                         `json:"manifest_removed"`
	Warnings          []string                     `json:"warnings,omitempty"`
}

// DetectInstallMethod derives the installation method from the executable path.
func DetectInstallMethod(home, executablePath string, evalSymlinks func(string) (string, error)) (InstallMethod, string, error) {
	resolved := executablePath
	if evalSymlinks != nil {
		realPath, err := evalSymlinks(executablePath)
		if err != nil && !os.IsNotExist(err) {
			return "", "", fmt.Errorf("resolving executable path: %w", err)
		}
		if err == nil && realPath != "" {
			resolved = realPath
		}
	}

	if isHomebrewPath(executablePath) || isHomebrewPath(resolved) {
		return MethodHomebrew, resolved, nil
	}
	if isGoInstallPath(home, executablePath) || isGoInstallPath(home, resolved) {
		return MethodGoInstall, resolved, nil
	}
	return MethodStandalone, resolved, nil
}

// RemoveGeneratedCLI deletes a generated CLI binary and optionally its sources.
func RemoveGeneratedCLI(entry manifest.CLIEntry, purgeSources bool) GeneratedCLIResult {
	result := GeneratedCLIResult{
		Name:       entry.Name,
		BinaryPath: entry.BinaryPath,
		SourceDir:  entry.SourceDir,
	}

	if entry.BinaryPath != "" {
		if err := os.Remove(entry.BinaryPath); err == nil {
			result.BinaryRemoved = true
		} else if !os.IsNotExist(err) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not remove binary %s: %v", entry.BinaryPath, err))
		}
	}

	if purgeSources && entry.SourceDir != "" {
		if err := os.RemoveAll(entry.SourceDir); err == nil {
			result.SourceRemoved = true
		} else {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not remove source dir %s: %v", entry.SourceDir, err))
		}
	}

	return result
}

// Self uninstalls the climate binary and optionally climate-managed local data.
func Self(opts Options) (Result, error) {
	method, resolvedExecutable, err := DetectInstallMethod(opts.Home, opts.ExecutablePath, opts.EvalSymlinks)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		InstallMethod:  string(method),
		Mode:           "cli",
		ExecutablePath: resolvedExecutable,
	}
	if opts.Full {
		result.Mode = "full"

		mf, loadErr := manifest.LoadFrom(filepath.Join(opts.Home, ".climate", "manifest.json"))
		if loadErr != nil {
			return Result{}, loadErr
		}

		for _, entry := range mf.List() {
			removed := RemoveGeneratedCLI(entry, true)
			result.GeneratedCLIs = append(result.GeneratedCLIs, removed)
			result.Warnings = append(result.Warnings, removed.Warnings...)
		}

		if err := os.Remove(mf.Path()); err == nil {
			result.ManifestRemoved = true
		} else if !os.IsNotExist(err) {
			return Result{}, fmt.Errorf("removing manifest: %w", err)
		}

		for _, shellName := range completion.SupportedShellNames() {
			shell, parseErr := completion.ParseShell(shellName)
			if parseErr != nil {
				return Result{}, parseErr
			}
			cleanupResult, cleanupErr := completion.Uninstall(opts.Home, shell, opts.GOOS)
			if cleanupErr != nil {
				return Result{}, cleanupErr
			}
			result.CompletionCleanup = append(result.CompletionCleanup, cleanupResult)
		}

		pruneIfEmpty(filepath.Join(opts.Home, ".climate", "completions"))
		pruneIfEmpty(filepath.Join(opts.Home, ".climate"))
	}

	switch method {
	case MethodHomebrew:
		if opts.RunCommand == nil {
			return Result{}, fmt.Errorf("homebrew uninstall requires a command runner")
		}
		if err := opts.RunCommand("brew", "uninstall", "climate"); err != nil {
			return Result{}, fmt.Errorf("running brew uninstall climate: %w", err)
		}
		result.ExecutableRemoved = true
	case MethodGoInstall, MethodStandalone:
		if err := os.Remove(resolvedExecutable); err != nil && !os.IsNotExist(err) {
			return Result{}, fmt.Errorf("removing executable %s: %w", resolvedExecutable, err)
		}
		result.ExecutableRemoved = true
	default:
		return Result{}, fmt.Errorf("unsupported install method %q", method)
	}

	return result, nil
}

func isHomebrewPath(path string) bool {
	return strings.Contains(filepath.ToSlash(path), "/Cellar/climate/")
}

func isGoInstallPath(home, path string) bool {
	cleanPath := filepath.Clean(path)
	for _, candidate := range goInstallCandidates(home) {
		if candidate == "" {
			continue
		}
		if cleanPath == filepath.Clean(candidate) {
			return true
		}
	}
	return false
}

func goInstallCandidates(home string) []string {
	candidates := []string{}
	if gobin := strings.TrimSpace(os.Getenv("GOBIN")); gobin != "" {
		candidates = append(candidates, filepath.Join(gobin, "climate"))
	}
	if gopath := strings.TrimSpace(os.Getenv("GOPATH")); gopath != "" {
		for _, part := range filepath.SplitList(gopath) {
			if strings.TrimSpace(part) == "" {
				continue
			}
			candidates = append(candidates, filepath.Join(part, "bin", "climate"))
		}
	}
	candidates = append(candidates, filepath.Join(home, "go", "bin", "climate"))
	return candidates
}

func pruneIfEmpty(path string) {
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) > 0 {
		return
	}
	_ = os.Remove(path)
}
