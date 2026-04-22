package commands

import (
	"fmt"
	"os"
	"runtime"

	cliCompletion "github.com/disk0Dancer/climate/internal/completion"
	"github.com/spf13/cobra"
)

var (
	completionInstallShell   string
	completionUninstallShell string
)

var completionCmd = &cobra.Command{
	Use:   "completion",
	Short: "Generate and manage shell completions",
	Long: `Generate shell completion scripts for climate or install them into your local shell setup.

Examples:
  climate completion zsh
  climate completion install --shell zsh
  climate completion uninstall --shell zsh`,
}

var completionInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install shell completions into the local shell config",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			exitError("Failed to find home directory", err)
		}

		shell, err := cliCompletion.ResolveShell(completionInstallShell, os.Getenv("SHELL"), runtime.GOOS)
		if err != nil {
			exitError("Failed to determine shell", err)
		}

		result, err := cliCompletion.Install(home, shell, runtime.GOOS, func(w cliCompletion.Writer) error {
			return generateCompletionScript(cmd.Root(), shell, w)
		})
		if err != nil {
			exitError("Failed to install shell completions", err)
		}

		writeJSON(result)
		return nil
	},
}

var completionUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove climate-managed shell completions",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			exitError("Failed to find home directory", err)
		}

		shell, err := cliCompletion.ResolveShell(completionUninstallShell, os.Getenv("SHELL"), runtime.GOOS)
		if err != nil {
			exitError("Failed to determine shell", err)
		}

		result, err := cliCompletion.Uninstall(home, shell, runtime.GOOS)
		if err != nil {
			exitError("Failed to uninstall shell completions", err)
		}

		writeJSON(result)
		return nil
	},
}

func newCompletionScriptCmd(shell cliCompletion.Shell) *cobra.Command {
	return &cobra.Command{
		Use:   string(shell),
		Short: fmt.Sprintf("Print the %s completion script", shell),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := generateCompletionScript(cmd.Root(), shell, cmd.OutOrStdout()); err != nil {
				exitError("Failed to generate completion script", err)
			}
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Tip: run `climate completion install --shell %s` to wire this into your local shell config.\n", shell)
			return nil
		},
	}
}

func generateCompletionScript(root *cobra.Command, shell cliCompletion.Shell, out cliCompletion.Writer) error {
	switch shell {
	case cliCompletion.ShellBash:
		return root.GenBashCompletionV2(out, true)
	case cliCompletion.ShellZsh:
		return root.GenZshCompletion(out)
	case cliCompletion.ShellFish:
		return root.GenFishCompletion(out, true)
	case cliCompletion.ShellPowerShell:
		return root.GenPowerShellCompletionWithDesc(out)
	default:
		return fmt.Errorf("unsupported shell %q", shell)
	}
}

func completeSupportedShells(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return cliCompletion.SupportedShellNames(), cobra.ShellCompDirectiveNoFileComp
}

func init() {
	completionInstallCmd.Flags().StringVar(&completionInstallShell, "shell", "", "Shell to install completions for")
	completionUninstallCmd.Flags().StringVar(&completionUninstallShell, "shell", "", "Shell to uninstall completions for")
	_ = completionInstallCmd.RegisterFlagCompletionFunc("shell", completeSupportedShells)
	_ = completionUninstallCmd.RegisterFlagCompletionFunc("shell", completeSupportedShells)

	completionCmd.AddCommand(newCompletionScriptCmd(cliCompletion.ShellBash))
	completionCmd.AddCommand(newCompletionScriptCmd(cliCompletion.ShellZsh))
	completionCmd.AddCommand(newCompletionScriptCmd(cliCompletion.ShellFish))
	completionCmd.AddCommand(newCompletionScriptCmd(cliCompletion.ShellPowerShell))
	completionCmd.AddCommand(completionInstallCmd)
	completionCmd.AddCommand(completionUninstallCmd)
	rootCmd.AddCommand(completionCmd)
}
