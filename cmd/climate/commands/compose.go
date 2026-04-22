package commands

import (
	"fmt"
	"strings"

	"github.com/disk0Dancer/climate/internal/compose"
	"github.com/disk0Dancer/climate/internal/generator"
	"github.com/disk0Dancer/climate/internal/manifest"
	"github.com/spf13/cobra"
)

var (
	composeName    string
	composeOutDir  string
	composeNoBuild bool
	composeForce   bool
	composeTitle   string
	composeVersion string
	composeDesc    string
)

var composeCmd = &cobra.Command{
	Use:   "compose [flags] <spec1>:<prefix1> [<spec2>:<prefix2> ...]",
	Short: "Compose multiple OpenAPI specs into a single gateway CLI",
	Long: `Merge several OpenAPI 3.x specifications — each assigned a path prefix —
into one composite spec, then generate a CLI from the result.

This is the recommended workflow for microservice environments where each
service owns its own OpenAPI document.  The resulting CLI acts as a single
facade: one binary, one authentication model, all services.

Each positional argument has the form:

  <spec>:<prefix>

Where <spec> is a file path or URL and <prefix> is a non-empty path prefix
that starts with "/" (e.g. "/api/v1").

Examples:
  climate compose orders.yaml:/api/orders users.yaml:/api/users
  climate compose --name gateway --title "Gateway API" \
      https://orders.svc/openapi.json:/orders \
      https://users.svc/openapi.json:/users`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputs, err := parseSpecInputs(args)
		if err != nil {
			exitError("Invalid spec:prefix arguments", err)
		}

		merged, rawBytes, err := compose.MergeToBytes(inputs, compose.Options{
			Title:       composeTitle,
			Version:     composeVersion,
			Description: composeDesc,
		})
		if err != nil {
			exitError("Failed to compose specs", err)
		}

		opts := generator.Options{
			CLIName:    composeName,
			OutDir:     composeOutDir,
			NoBuild:    composeNoBuild,
			Force:      composeForce,
			SpecSource: buildSpecSourceLabel(inputs),
		}

		result, err := generator.Generate(merged, rawBytes, opts)
		if err != nil {
			exitError("Generation failed", err)
		}

		// Update manifest.
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
				OpenAPISpec: opts.SpecSource,
			})
			if saveErr := mf.Save(); saveErr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not save manifest: %v\n", saveErr)
			}
		}

		writeJSON(result)
		return nil
	},
}

// parseSpecInputs splits each "spec:prefix" argument into a SpecInput.
func parseSpecInputs(args []string) ([]compose.SpecInput, error) {
	inputs := make([]compose.SpecInput, 0, len(args))
	for _, arg := range args {
		// A URL contains "://" so we must find the colon that separates the
		// spec from the prefix carefully: the prefix always starts with "/" so
		// the last occurrence of ":/" is the boundary.
		idx := strings.LastIndex(arg, ":/")
		if idx < 0 {
			// Plain colon split (local path with no scheme).
			parts := strings.SplitN(arg, ":", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("argument %q must have the form <spec>:<prefix>", arg)
			}
			inputs = append(inputs, compose.SpecInput{Source: parts[0], Prefix: parts[1]})
			continue
		}

		// Check whether ":/" is the scheme separator ("://") or the
		// spec/prefix boundary.
		schemeIdx := strings.Index(arg, "://")
		if schemeIdx >= 0 && schemeIdx == idx {
			// The only ":/" is the scheme — there's no prefix separator.
			return nil, fmt.Errorf("argument %q must have the form <spec>:<prefix> (e.g. https://host/spec.json:/prefix)", arg)
		}

		// The last ":/" is the boundary (handles https://…:/prefix).
		source := arg[:idx]
		prefix := arg[idx+1:] // keeps the leading "/"
		inputs = append(inputs, compose.SpecInput{Source: source, Prefix: prefix})
	}
	return inputs, nil
}

// buildSpecSourceLabel returns a human-readable label listing all source specs.
func buildSpecSourceLabel(inputs []compose.SpecInput) string {
	parts := make([]string, len(inputs))
	for i, inp := range inputs {
		parts[i] = inp.Source + "@" + inp.Prefix
	}
	return "compose:[" + strings.Join(parts, ",") + "]"
}

func init() {
	composeCmd.Flags().StringVar(&composeName, "name", "", "Override the generated CLI name")
	composeCmd.Flags().StringVar(&composeOutDir, "out-dir", "", "Directory for generated source code")
	composeCmd.Flags().BoolVar(&composeNoBuild, "no-build", false, "Skip building the binary")
	composeCmd.Flags().BoolVar(&composeForce, "force", false, "Overwrite existing output directory")
	composeCmd.Flags().StringVar(&composeTitle, "title", "", "Title for the composed API (info.title)")
	composeCmd.Flags().StringVar(&composeVersion, "api-version", "1.0.0", "Version for the composed API (info.version)")
	composeCmd.Flags().StringVar(&composeDesc, "description", "", "Description for the composed API (info.description)")
	rootCmd.AddCommand(composeCmd)
}
