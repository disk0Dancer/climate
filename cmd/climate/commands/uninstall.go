package commands

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/disk0Dancer/climate/internal/confirm"
	"github.com/disk0Dancer/climate/internal/manifest"
	cliUninstall "github.com/disk0Dancer/climate/internal/uninstall"
	"github.com/spf13/cobra"
)

var (
	uninstallFull bool
	uninstallYes  bool
)

var (
	uninstallExecutablePath = os.Executable
	uninstallEvalSymlinks   = filepath.EvalSymlinks
	uninstallCommandRunner  = runExternalCommand
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall the climate CLI",
	Long: `Uninstall the climate executable itself.

By default only the climate CLI is removed. Use --full to also remove
generated CLIs, the manifest, and climate-managed shell completions.

The command asks for confirmation unless --yes is set.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			exitError("Failed to find home directory", err)
		}

		executablePath, err := uninstallExecutablePath()
		if err != nil {
			exitError("Failed to resolve climate executable path", err)
		}

		method, _, err := cliUninstall.DetectInstallMethod(home, executablePath, uninstallEvalSymlinks)
		if err != nil {
			exitError("Failed to detect installation method", err)
		}

		if !uninstallYes {
			confirmed, confirmErr := confirm.Ask(cmd.InOrStdin(), cmd.ErrOrStderr(), uninstallPrompt(home, method, uninstallFull))
			if confirmErr != nil {
				exitError("Failed to read confirmation", confirmErr)
			}
			if !confirmed {
				type cancelResp struct {
					Cancelled bool   `json:"cancelled"`
					Target    string `json:"target"`
					Mode      string `json:"mode"`
				}
				mode := "cli"
				if uninstallFull {
					mode = "full"
				}
				writeJSON(cancelResp{Cancelled: true, Target: "climate", Mode: mode})
				return nil
			}
		}

		result, err := cliUninstall.Self(cliUninstall.Options{
			Home:           home,
			GOOS:           runtime.GOOS,
			ExecutablePath: executablePath,
			Full:           uninstallFull,
			EvalSymlinks:   uninstallEvalSymlinks,
			RunCommand: func(name string, args ...string) error {
				return uninstallCommandRunner(cmd.ErrOrStderr(), cmd.ErrOrStderr(), name, args...)
			},
		})
		if err != nil {
			exitError("Failed to uninstall climate", err)
		}

		writeJSON(result)
		return nil
	},
}

func uninstallPrompt(home string, method cliUninstall.InstallMethod, full bool) string {
	if !full {
		return fmt.Sprintf("Uninstall climate (%s)? This removes only the climate executable.", method)
	}

	count := 0
	mf, err := manifest.LoadFrom(filepath.Join(home, ".climate", "manifest.json"))
	if err == nil {
		count = len(mf.List())
	}

	return fmt.Sprintf("Fully uninstall climate (%s)? This removes the climate executable, %d generated CLI(s), their source directories, the manifest, and climate-managed shell completions.", method, count)
}

func runExternalCommand(stdout, stderr io.Writer, name string, args ...string) error {
	command := exec.Command(name, args...)
	command.Stdout = stdout
	command.Stderr = stderr
	return command.Run()
}

func init() {
	uninstallCmd.Flags().BoolVar(&uninstallFull, "full", false, "Also remove generated CLIs, manifest, and climate-managed local state")
	uninstallCmd.Flags().BoolVar(&uninstallYes, "yes", false, "Skip the confirmation prompt")
	rootCmd.AddCommand(uninstallCmd)
}
