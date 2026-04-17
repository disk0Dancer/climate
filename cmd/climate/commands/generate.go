package commands

import (
	"fmt"

	"github.com/disk0Dancer/climate/internal/generator"
	"github.com/disk0Dancer/climate/internal/manifest"
	"github.com/disk0Dancer/climate/internal/spec"
	"github.com/spf13/cobra"
)

var (
	generateCLIName string
	generateOutDir  string
	generateNoBuild bool
	generateForce   bool
)

var generateCmd = &cobra.Command{
	Use:   "generate [flags] <openapi_spec>",
	Short: "Generate a CLI from an OpenAPI specification",
	Long: `Generate a production-ready Go CLI from an OpenAPI 3.x specification.

The spec can be a local file path or an HTTP(S) URL.

Examples:
  climate generate https://petstore3.swagger.io/api/v3/openapi.json
  climate generate --name myapi ./openapi.yaml
  climate generate --name myapi --out-dir /tmp/myapi --no-build ./openapi.json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		specSource := args[0]

		// Load and validate spec
		rawBytes, err := spec.RawBytes(specSource)
		if err != nil {
			exitError("Failed to load spec", err)
		}

		openAPI, err := spec.Parse(specSource, rawBytes)
		if err != nil {
			exitError("Failed to parse spec", err)
		}

		opts := generator.Options{
			CLIName:    generateCLIName,
			OutDir:     generateOutDir,
			NoBuild:    generateNoBuild,
			Force:      generateForce,
			SpecSource: specSource,
		}

		result, err := generator.Generate(openAPI, rawBytes, opts)
		if err != nil {
			exitError("Generation failed", err)
		}

		// Update manifest
		mf, err := manifest.Load()
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not load manifest: %v\n", err)
		} else {
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
		}

		writeJSON(result)
		return nil
	},
}

func init() {
	generateCmd.Flags().StringVar(&generateCLIName, "name", "", "Override the generated CLI name")
	generateCmd.Flags().StringVar(&generateOutDir, "out-dir", "", "Directory for generated source code")
	generateCmd.Flags().BoolVar(&generateNoBuild, "no-build", false, "Skip building the binary")
	generateCmd.Flags().BoolVar(&generateForce, "force", false, "Overwrite existing output directory")
	rootCmd.AddCommand(generateCmd)
}
