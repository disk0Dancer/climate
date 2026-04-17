// Package commands implements the climate CLI root command and subcommands.
package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags "-X github.com/disk0Dancer/climate/cmd/climate/commands.version=vX.Y.Z"
var version = "dev"

var rootCmd = &cobra.Command{
	Use:     "climate",
	Short:   "climate — CLI Tool Orchestrator",
	Long:    `climate generates production-ready Go CLIs from OpenAPI specifications.`,
	Version: version,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// writeJSON prints v as indented JSON to stdout.
func writeJSON(v interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintln(os.Stderr, "error encoding output:", err)
		os.Exit(1)
	}
}

// exitError prints a JSON-formatted error to stderr and exits.
func exitError(msg string, err error) {
	type errResp struct {
		Error struct {
			Message string `json:"message"`
			Detail  string `json:"detail,omitempty"`
		} `json:"error"`
	}
	resp := errResp{}
	resp.Error.Message = msg
	if err != nil {
		resp.Error.Detail = err.Error()
	}
	enc := json.NewEncoder(os.Stderr)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp)
	os.Exit(1)
}
