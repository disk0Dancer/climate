package commands

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/disk0Dancer/climate/internal/mock"
	"github.com/disk0Dancer/climate/internal/spec"
	"github.com/spf13/cobra"
)

var (
	mockPort        int
	mockLatency     int
	mockEmitURL     string
	mockEventPath   string
	mockEventMethod string
)

var mockCmd = &cobra.Command{
	Use:   "mock [flags] <openapi_spec>",
	Short: "Start a local HTTP mock server from an OpenAPI spec",
	Long: `Start a local HTTP mock server that serves synthetic responses for every
endpoint defined in an OpenAPI 3.x specification.

This is useful for local development and testing when the real service is
unavailable, produces side-effects, or you simply want to experiment with
the API surface without any credentials.

The server inspects each operation's first successful (2xx) response schema
and generates a plausible JSON value — objects with all declared properties
filled in, arrays with one example element, and scalars set to sensible
zero values.

The spec can be a local file path or an HTTP(S) URL.

Examples:
  climate mock ./openapi.yaml
  climate mock --port 9090 https://petstore3.swagger.io/api/v3/openapi.json
  climate mock --latency 200 ./orders.yaml
  climate mock --emit-url http://localhost:3001/webhook --event-path /events/order-created --event-method POST ./openapi.yaml`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		specSource := args[0]
		method := strings.ToUpper(strings.TrimSpace(mockEventMethod))
		if method == "" {
			method = http.MethodPost
		}
		emitMode := strings.TrimSpace(mockEmitURL) != ""

		if !emitMode && (cmd.Flags().Changed("event-path") || cmd.Flags().Changed("event-method")) {
			exitError("--event-path or --event-method require --emit-url", nil)
		}
		if emitMode {
			if strings.TrimSpace(mockEventPath) == "" {
				exitError("Missing required flag --event-path when using --emit-url", nil)
			}
			if !isValidHTTPMethod(method) {
				exitError(
					"Invalid --event-method value",
					fmt.Errorf("unsupported HTTP method %q (supported: GET, POST, PUT, PATCH, DELETE)", method),
				)
			}
		}

		openAPI, err := spec.Load(specSource)
		if err != nil {
			exitError("Failed to load spec", err)
		}

		if emitMode {
			payload, err := mock.GenerateEventPayload(openAPI, mockEventPath, method)
			if err != nil {
				exitError("Failed to generate event payload", err)
			}
			statusCode, err := mock.EmitEvent(mockEmitURL, method, payload)
			if err != nil {
				exitError("Failed to emit event", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"Emitted %s event from %s to %s (status: %d)\n",
				method, mockEventPath, mockEmitURL, statusCode)
			return nil
		}

		addr := fmt.Sprintf(":%d", mockPort)
		latency := time.Duration(mockLatency) * time.Millisecond
		s := mock.NewServer(openAPI, addr, latency)

		fmt.Fprintf(cmd.OutOrStdout(), "Mock server for %q listening on http://localhost%s\n",
			openAPI.Info.Title, addr)
		if mockLatency > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Artificial latency: %dms\n", mockLatency)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "\nRoutes:")
		fmt.Fprint(cmd.OutOrStdout(), s.Summary())
		fmt.Fprintln(cmd.OutOrStdout(), "\nPress Ctrl+C to stop.")

		if err := s.ListenAndServe(); err != nil {
			exitError("Mock server error", err)
		}
		return nil
	},
}

func init() {
	mockCmd.Flags().IntVar(&mockPort, "port", 8080, "TCP port to listen on")
	mockCmd.Flags().IntVar(&mockLatency, "latency", 0, "Artificial response latency in milliseconds")
	mockCmd.Flags().StringVar(&mockEmitURL, "emit-url", "", "Send one synthetic webhook/event payload to this URL and exit")
	mockCmd.Flags().StringVar(&mockEventPath, "event-path", "", "OpenAPI path to use for synthetic event payload generation (required with --emit-url)")
	mockCmd.Flags().StringVar(&mockEventMethod, "event-method", "POST", "HTTP method to use for event emission with --emit-url")
	rootCmd.AddCommand(mockCmd)
}

func isValidHTTPMethod(method string) bool {
	switch method {
	case http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete:
		return true
	default:
		return false
	}
}
