package commands

import (
	"fmt"
	"os"

	"github.com/disk0Dancer/climate/internal/manifest"
	"github.com/spf13/cobra"
)

var removePurgeSources bool

var removeCmd = &cobra.Command{
	Use:   "remove <cli-name>",
	Short: "Remove a generated CLI",
	Long:  `Remove a CLI binary and its manifest entry. Use --purge-sources to also delete source files.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cliName := args[0]

		mf, err := manifest.Load()
		if err != nil {
			exitError("Failed to load manifest", err)
		}

		entry, ok := mf.Get(cliName)
		if !ok {
			exitError(fmt.Sprintf("CLI %q not found in manifest", cliName), nil)
		}

		// Remove binary
		if entry.BinaryPath != "" {
			if rmErr := os.Remove(entry.BinaryPath); rmErr != nil && !os.IsNotExist(rmErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not remove binary %s: %v\n", entry.BinaryPath, rmErr)
			}
		}

		// Optionally remove sources
		if removePurgeSources && entry.SourceDir != "" {
			if rmErr := os.RemoveAll(entry.SourceDir); rmErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not remove source dir %s: %v\n", entry.SourceDir, rmErr)
			}
		}

		mf.Remove(cliName)
		if saveErr := mf.Save(); saveErr != nil {
			exitError("Failed to save manifest", saveErr)
		}

		type removeResp struct {
			Removed string `json:"removed"`
		}
		writeJSON(removeResp{Removed: cliName})
		return nil
	},
}

func init() {
	removeCmd.Flags().BoolVar(&removePurgeSources, "purge-sources", false, "Also delete generated source files")
	rootCmd.AddCommand(removeCmd)
}
