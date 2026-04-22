package commands

import (
	"fmt"

	"github.com/disk0Dancer/climate/internal/confirm"
	"github.com/disk0Dancer/climate/internal/manifest"
	cliUninstall "github.com/disk0Dancer/climate/internal/uninstall"
	"github.com/spf13/cobra"
)

var (
	removePurgeSources bool
	removeYes          bool
)

var removeCmd = &cobra.Command{
	Use:   "remove <cli-name>",
	Short: "Remove a generated CLI",
	Long:  `Remove a CLI binary and its manifest entry. Use --purge-sources to also delete source files. The command asks for confirmation unless --yes is set.`,
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

		if !removeYes {
			confirmed, confirmErr := confirm.Ask(cmd.InOrStdin(), cmd.ErrOrStderr(), removePrompt(entry.Name, removePurgeSources))
			if confirmErr != nil {
				exitError("Failed to read confirmation", confirmErr)
			}
			if !confirmed {
				type cancelResp struct {
					Cancelled bool   `json:"cancelled"`
					Target    string `json:"target"`
				}
				writeJSON(cancelResp{Cancelled: true, Target: cliName})
				return nil
			}
		}

		removed := cliUninstall.RemoveGeneratedCLI(entry, removePurgeSources)
		mf.Remove(cliName)
		if saveErr := mf.Save(); saveErr != nil {
			exitError("Failed to save manifest", saveErr)
		}

		type removeResp struct {
			Removed       string   `json:"removed"`
			BinaryRemoved bool     `json:"binary_removed"`
			SourceRemoved bool     `json:"source_removed,omitempty"`
			Warnings      []string `json:"warnings,omitempty"`
		}
		writeJSON(removeResp{
			Removed:       cliName,
			BinaryRemoved: removed.BinaryRemoved,
			SourceRemoved: removed.SourceRemoved,
			Warnings:      removed.Warnings,
		})
		return nil
	},
}

func removePrompt(cliName string, purgeSources bool) string {
	if purgeSources {
		return fmt.Sprintf("Remove generated CLI %q and delete its source directory?", cliName)
	}
	return fmt.Sprintf("Remove generated CLI %q?", cliName)
}

func init() {
	removeCmd.Flags().BoolVar(&removePurgeSources, "purge-sources", false, "Also delete generated source files")
	removeCmd.Flags().BoolVar(&removeYes, "yes", false, "Skip the confirmation prompt")
	rootCmd.AddCommand(removeCmd)
}
