package commands

import (
	"fmt"

	"github.com/disk0Dancer/climate/internal/generator"
	"github.com/disk0Dancer/climate/internal/manifest"
	"github.com/disk0Dancer/climate/internal/spec"
	"github.com/spf13/cobra"
)

var upgradeOpenAPI string

var upgradeCmd = &cobra.Command{
	Use:   "upgrade <cli-name>",
	Short: "Re-generate a CLI from an updated spec",
	Long:  `Re-generate a previously created CLI, optionally using a new OpenAPI spec.`,
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

		specSource := upgradeOpenAPI
		if specSource == "" {
			specSource = entry.OpenAPISpec
		}
		if specSource == "" {
			exitError("No OpenAPI spec available; provide --openapi <spec>", nil)
		}

		rawBytes, err := spec.RawBytes(specSource)
		if err != nil {
			exitError("Failed to load spec", err)
		}

		openAPI, err := spec.Parse(specSource, rawBytes)
		if err != nil {
			exitError("Failed to parse spec", err)
		}

		opts := generator.Options{
			CLIName:    cliName,
			OutDir:     entry.SourceDir,
			NoBuild:    false,
			Force:      true, // overwrite existing sources
			SpecSource: specSource,
		}

		result, err := generator.Generate(openAPI, rawBytes, opts)
		if err != nil {
			exitError("Generation failed", err)
		}

		mf.Upsert(manifest.CLIEntry{
			Name:        result.CLIName,
			BinaryPath:  result.BinaryPath,
			SourceDir:   result.SourceDir,
			Version:     result.Version,
			OpenAPIHash: result.OpenAPIHash,
			OpenAPISpec: specSource,
		})
		if saveErr := mf.Save(); saveErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not save manifest: %v\n", saveErr)
		}

		writeJSON(result)
		return nil
	},
}

func init() {
	upgradeCmd.Flags().StringVar(&upgradeOpenAPI, "openapi", "", "Override the OpenAPI spec source")
	rootCmd.AddCommand(upgradeCmd)
}
