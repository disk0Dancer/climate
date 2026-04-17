package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/disk0Dancer/climate/internal/manifest"
	"github.com/disk0Dancer/climate/internal/publish"
	"github.com/disk0Dancer/climate/internal/spec"
	"github.com/spf13/cobra"
)

var (
	publishOwner         string
	publishRepo          string
	publishDescription   string
	publishHomepage      string
	publishVisibility    string
	publishDefaultBranch string
	publishGitHubToken   string
	publishReuseExisting bool
)

var publishCmd = &cobra.Command{
	Use:   "publish <cli-name>",
	Short: "Publish a generated CLI into a GitHub repository",
	Long: `Create or reuse a GitHub repository for a generated CLI, bootstrap
repository lifecycle files, initialize git, and push the source directory.

Authentication is read from --github-token, GITHUB_TOKEN, or GH_TOKEN.`,
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
		if entry.SourceDir == "" {
			exitError("No source directory recorded for this CLI", nil)
		}
		if entry.OpenAPISpec == "" {
			exitError("No OpenAPI spec recorded for this CLI; re-generate it before publishing", nil)
		}

		rawSpec, err := spec.RawBytes(entry.OpenAPISpec)
		if err != nil {
			exitError("Failed to load spec", err)
		}
		openAPI, err := spec.Parse(entry.OpenAPISpec, rawSpec)
		if err != nil {
			exitError("Failed to parse spec", err)
		}

		visibility := strings.ToLower(strings.TrimSpace(publishVisibility))
		switch visibility {
		case "", "public":
			visibility = "public"
		case "private":
		default:
			exitError("Visibility must be public or private", nil)
		}

		result, err := publish.Publish(context.Background(), entry, openAPI, publish.Options{
			Token:         publishGitHubToken,
			Owner:         publishOwner,
			RepoName:      publishRepo,
			Description:   publishDescription,
			Homepage:      publishHomepage,
			Visibility:    visibility,
			DefaultBranch: publishDefaultBranch,
			ReuseExisting: publishReuseExisting,
		})
		if err != nil {
			exitError("Publish failed", err)
		}

		mf.Upsert(publish.PublishedManifestEntry(entry, result))
		if saveErr := mf.Save(); saveErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not save manifest: %v\n", saveErr)
		}

		writeJSON(result)
		return nil
	},
}

func init() {
	publishCmd.Flags().StringVar(&publishOwner, "owner", "", "GitHub owner or organization; defaults to the authenticated user")
	publishCmd.Flags().StringVar(&publishRepo, "repo", "", "Override the repository name; defaults to the CLI name")
	publishCmd.Flags().StringVar(&publishDescription, "description", "", "Override the GitHub repository description")
	publishCmd.Flags().StringVar(&publishHomepage, "homepage", "", "Repository homepage URL")
	publishCmd.Flags().StringVar(&publishVisibility, "visibility", "public", "Repository visibility: public|private")
	publishCmd.Flags().StringVar(&publishDefaultBranch, "default-branch", "main", "Default branch name to create and push")
	publishCmd.Flags().StringVar(&publishGitHubToken, "github-token", "", "GitHub token; falls back to GITHUB_TOKEN or GH_TOKEN")
	publishCmd.Flags().BoolVar(&publishReuseExisting, "reuse-existing", true, "Reuse an existing GitHub repository when it already exists")
	rootCmd.AddCommand(publishCmd)
}
