package commands

import (
	"fmt"
	"os"

	"github.com/disk0Dancer/climate/internal/manifest"
	"github.com/disk0Dancer/climate/internal/skill"
	"github.com/disk0Dancer/climate/internal/spec"
	"github.com/disk0Dancer/climate/skills"
	"github.com/spf13/cobra"
)

var (
	skillMode    string
	skillOutPath string
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage skills for generated CLIs",
}

var skillGenerateCmd = &cobra.Command{
	Use:   "generate <cli-name>",
	Short: "Print a plain-text agent prompt for a generated CLI",
	Long: `Print a plain-text Markdown prompt describing how to use a generated CLI.

The prompt is designed to be read by an LLM agent so it can self-register
the CLI as a skill and write its own usage instructions.

Modes:
  full    — one documented command per OpenAPI operation (default)
  compact — shorter summary grouped by tag`,
	Args: cobra.ExactArgs(1),
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

		if entry.OpenAPISpec == "" {
			exitError("No OpenAPI spec recorded for this CLI; re-generate with climate generate", nil)
		}

		openAPI, err := spec.Load(entry.OpenAPISpec)
		if err != nil {
			exitError("Failed to load OpenAPI spec", err)
		}

		mode := skill.ModeFull
		if skillMode == "compact" {
			mode = skill.ModeCompact
		}

		prompt := skill.GenerateCLIPrompt(entry, openAPI, mode)

		if skillOutPath != "" {
			if writeErr := os.WriteFile(skillOutPath, []byte(prompt), 0o644); writeErr != nil {
				exitError("Failed to write prompt file", writeErr)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Skill prompt written to %s\n", skillOutPath)
			return nil
		}

		fmt.Fprint(cmd.OutOrStdout(), prompt)
		return nil
	},
}

// skillGeneratorCmd prints the embedded skills/climate.md file — the static
// Markdown skill for climate.generator itself.
var skillGeneratorCmd = &cobra.Command{
	Use:   "generator",
	Short: "Print the climate.generator skill (skills/climate.md)",
	Long: `Print the plain-text Markdown skill prompt for climate.generator.

This is the content of skills/climate.md shipped with climate.
An LLM agent can read it to learn how to use climate and self-register
it as the climate.generator skill.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprint(cmd.OutOrStdout(), skills.ClimateMD)
		return nil
	},
}

func init() {
	skillGenerateCmd.Flags().StringVar(&skillMode, "mode", "full", "Prompt verbosity: full|compact")
	skillGenerateCmd.Flags().StringVar(&skillOutPath, "out", "", "Write prompt to this file instead of stdout")
	skillCmd.AddCommand(skillGenerateCmd)
	skillCmd.AddCommand(skillGeneratorCmd)
	rootCmd.AddCommand(skillCmd)
}
