package spec

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Load loads an OpenAPI specification from a file path or HTTP(S) URL.
func Load(source string) (*OpenAPI, error) {
	var data []byte
	var err error

	if isURL(source) {
		data, err = fetchURL(source)
	} else {
		data, err = os.ReadFile(source)
	}
	if err != nil {
		return nil, fmt.Errorf("loading spec: %w", err)
	}

	return Parse(source, data)
}

// Parse parses raw OpenAPI spec bytes (JSON or YAML).
func Parse(source string, data []byte) (*OpenAPI, error) {
	var spec OpenAPI

	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "{") {
		if err := json.Unmarshal(data, &spec); err != nil {
			return nil, fmt.Errorf("parsing JSON spec: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(data, &spec); err != nil {
			return nil, fmt.Errorf("parsing YAML spec: %w", err)
		}
	}

	if err := Validate(&spec); err != nil {
		return nil, err
	}

	resolveParameterRefs(&spec)

	return &spec, nil
}

// Validate performs basic structural validation of an OpenAPI spec.
func Validate(spec *OpenAPI) error {
	if spec.OpenAPI == "" {
		return &ValidationError{Message: "missing required field: openapi"}
	}
	if !strings.HasPrefix(spec.OpenAPI, "3.") {
		return &ValidationError{Message: fmt.Sprintf("unsupported OpenAPI version %q: only 3.x is supported", spec.OpenAPI)}
	}
	if spec.Info.Title == "" {
		return &ValidationError{Message: "missing required field: info.title"}
	}
	if spec.Info.Version == "" {
		return &ValidationError{Message: "missing required field: info.version"}
	}
	if len(spec.Paths) == 0 {
		return &ValidationError{Message: "spec has no paths defined"}
	}
	return nil
}

// ValidationError represents a spec validation error.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return "spec validation: " + e.Message
}

// resolveParameterRefs resolves $ref pointers in operation parameters
// against components/parameters.
func resolveParameterRefs(spec *OpenAPI) {
	if len(spec.Components.Parameters) == 0 {
		return
	}
	const prefix = "#/components/parameters/"
	for pathKey, pi := range spec.Paths {
		for method, op := range pi.Operations() {
			_ = method
			for i, p := range op.Parameters {
				if p.Ref == "" {
					continue
				}
				if !strings.HasPrefix(p.Ref, prefix) {
					continue
				}
				name := strings.TrimPrefix(p.Ref, prefix)
				if resolved, ok := spec.Components.Parameters[name]; ok {
					op.Parameters[i] = resolved
				}
			}
		}
		_ = pathKey
	}
}

// isURL reports whether s looks like an HTTP(S) URL.
func isURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https")
}

// fetchURL downloads a URL and returns its body.
func fetchURL(rawURL string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(rawURL) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", rawURL, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: HTTP %d", rawURL, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response from %s: %w", rawURL, err)
	}
	return body, nil
}
