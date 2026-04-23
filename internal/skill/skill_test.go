package skill_test

import (
	"strings"
	"testing"

	"github.com/disk0Dancer/climate/internal/manifest"
	"github.com/disk0Dancer/climate/internal/skill"
	"github.com/disk0Dancer/climate/internal/spec"
)

func sampleOpenAPI() *spec.OpenAPI {
	return &spec.OpenAPI{
		OpenAPI: "3.0.0",
		Info:    spec.Info{Title: "Petstore", Version: "1.0.0", Description: "A sample pet store API"},
		Tags:    []spec.Tag{{Name: "pets", Description: "Pet operations"}},
		Servers: []spec.Server{{URL: "https://petstore.example.com/v1"}},
		Paths: map[string]spec.PathItem{
			"/pets": {
				Get: &spec.Operation{
					OperationID: "pets_list",
					Summary:     "List all pets",
					Tags:        []string{"pets"},
					Parameters: []spec.Parameter{
						{Name: "limit", In: "query", Description: "Max results"},
					},
				},
				Post: &spec.Operation{
					OperationID: "pets_create",
					Summary:     "Create a pet",
					Tags:        []string{"pets"},
					RequestBody: &spec.RequestBody{Required: true},
				},
			},
			"/pets/{petId}": {
				Get: &spec.Operation{
					OperationID: "pets_getById",
					Summary:     "Get a pet",
					Tags:        []string{"pets"},
					Parameters: []spec.Parameter{
						{Name: "petId", In: "path", Required: true},
					},
				},
			},
		},
	}
}

func TestGenerateCLIPromptFull(t *testing.T) {
	entry := manifest.CLIEntry{
		Name:       "petstore",
		BinaryPath: "/home/user/.climate/bin/petstore",
		Version:    "1.0.0",
	}
	openAPI := sampleOpenAPI()

	prompt := skill.GenerateCLIPrompt(entry, openAPI, skill.ModeFull)

	if prompt == "" {
		t.Fatal("GenerateCLIPrompt() returned empty string")
	}
	if !strings.Contains(prompt, "# Skill: petstore") {
		t.Error("prompt should contain skill header")
	}
	if !strings.Contains(prompt, "cli.petstore") {
		t.Error("prompt should contain skill id")
	}
	// Full mode must document individual operations
	if !strings.Contains(prompt, "List all pets") {
		t.Error("full mode prompt should document List all pets operation")
	}
	if !strings.Contains(prompt, "Create a pet") {
		t.Error("full mode prompt should document Create a pet operation")
	}
	// Should contain the binary path
	if !strings.Contains(prompt, "/home/user/.climate/bin/petstore") {
		t.Error("prompt should reference the binary path")
	}
	// Should describe output format
	if !strings.Contains(prompt, "## Output format") {
		t.Error("prompt should describe output format")
	}
	// Should contain self-registration instructions
	if !strings.Contains(prompt, "## How to register") {
		t.Error("prompt should contain self-registration instructions")
	}
}

func TestGenerateCLIPromptCompact(t *testing.T) {
	entry := manifest.CLIEntry{Name: "petstore", Version: "1.0.0"}
	openAPI := sampleOpenAPI()

	prompt := skill.GenerateCLIPrompt(entry, openAPI, skill.ModeCompact)

	if !strings.Contains(prompt, "# Skill: petstore") {
		t.Error("compact prompt should have skill header")
	}
	// Compact: shows tags, not individual ops
	if !strings.Contains(prompt, "### pets") {
		t.Error("compact prompt should show tag sections")
	}
	// Compact should NOT list every individual operation summary
	if strings.Contains(prompt, "List all pets") {
		t.Error("compact prompt should not list individual operation summaries")
	}
}

func TestGenerateCLIPromptWithAuth(t *testing.T) {
	entry := manifest.CLIEntry{Name: "securedapi", Version: "1.0.0"}
	openAPI := sampleOpenAPI()
	openAPI.Components = spec.Components{
		SecuritySchemes: map[string]spec.SecurityScheme{
			"bearerAuth": {Type: "http", Scheme: "bearer"},
			"apiKey":     {Type: "apiKey", Name: "X-API-Key", In: "header"},
		},
	}

	prompt := skill.GenerateCLIPrompt(entry, openAPI, skill.ModeFull)

	if !strings.Contains(prompt, "## Authentication") {
		t.Error("prompt should contain authentication section")
	}
	if !strings.Contains(prompt, "SECUREDAPI_TOKEN") {
		t.Error("prompt should contain bearer token env var")
	}
	if !strings.Contains(prompt, "SECUREDAPI_APIKEY_API_KEY") {
		t.Error("prompt should contain API key env var")
	}
}

func TestGenerateCLIPromptServerURL(t *testing.T) {
	entry := manifest.CLIEntry{Name: "petstore", Version: "1.0.0"}
	openAPI := sampleOpenAPI()

	prompt := skill.GenerateCLIPrompt(entry, openAPI, skill.ModeFull)

	if !strings.Contains(prompt, "https://petstore.example.com/v1") {
		t.Error("prompt should mention the default server URL")
	}
	if !strings.Contains(prompt, "PETSTORE_BASE_URL") {
		t.Error("prompt should mention the base URL env var override")
	}
}

func TestGenerateCLIPromptBodyFlags(t *testing.T) {
	entry := manifest.CLIEntry{Name: "petstore", Version: "1.0.0"}
	openAPI := sampleOpenAPI()

	prompt := skill.GenerateCLIPrompt(entry, openAPI, skill.ModeFull)

	// The create operation has a request body — prompt must document --data-json
	if !strings.Contains(prompt, "--data-json") {
		t.Error("prompt should document --data-json flag for operations with a request body")
	}
	if !strings.Contains(prompt, "--data-file") {
		t.Error("prompt should document --data-file flag for operations with a request body")
	}
}

func TestGenerateCLIPromptServerVariables(t *testing.T) {
	entry := manifest.CLIEntry{Name: "petstore", Version: "1.0.0"}
	openAPI := sampleOpenAPI()
	openAPI.Servers = []spec.Server{
		{
			URL: "https://{region}.api.example.com/{basePath}",
			Variables: map[string]spec.ServerVariable{
				"region":   {Default: "eu"},
				"basePath": {Default: "v1"},
			},
		},
	}

	prompt := skill.GenerateCLIPrompt(entry, openAPI, skill.ModeFull)

	if !strings.Contains(prompt, "--server-var-region") {
		t.Error("prompt should document --server-var-region")
	}
	if !strings.Contains(prompt, "--server-var-base-path") {
		t.Error("prompt should document --server-var-base-path")
	}
	if !strings.Contains(prompt, "PETSTORE_SERVER_VAR_REGION") {
		t.Error("prompt should mention PETSTORE_SERVER_VAR_REGION env var")
	}
	if !strings.Contains(prompt, "PETSTORE_SERVER_VAR_BASE_PATH") {
		t.Error("prompt should mention PETSTORE_SERVER_VAR_BASE_PATH env var")
	}
}

func TestGenerateCLIPromptIncludesEventsListener(t *testing.T) {
	entry := manifest.CLIEntry{Name: "petstore", Version: "1.0.0"}
	openAPI := sampleOpenAPI()
	openAPI.Components = spec.Components{
		SecuritySchemes: map[string]spec.SecurityScheme{
			"bearerAuth": {Type: "http", Scheme: "bearer"},
		},
	}

	prompt := skill.GenerateCLIPrompt(entry, openAPI, skill.ModeFull)

	if !strings.Contains(prompt, "events list") {
		t.Error("prompt should mention the generated events list command")
	}
	if !strings.Contains(prompt, "events listen") {
		t.Error("prompt should mention the generated events listener command")
	}
	if !strings.Contains(prompt, "events emit") {
		t.Error("prompt should mention the generated events emit command")
	}
	if !strings.Contains(prompt, "config list") {
		t.Error("prompt should mention generated config commands")
	}
	if !strings.Contains(prompt, "config set --secret events.signing_secret") {
		t.Error("prompt should mention secret config storage")
	}
	if !strings.Contains(prompt, "config profiles use") {
		t.Error("prompt should mention named profile management")
	}
	if !strings.Contains(prompt, "--tunnel auto") {
		t.Error("prompt should mention tunnel support for the events listener")
	}
	if !strings.Contains(prompt, "--signature-mode") {
		t.Error("prompt should mention generic HMAC signature support")
	}
	if !strings.Contains(prompt, "auth login") {
		t.Error("prompt should mention interactive auth commands")
	}
}

func TestGenerateCLIPromptRequiredParam(t *testing.T) {
	entry := manifest.CLIEntry{Name: "petstore", Version: "1.0.0"}
	openAPI := sampleOpenAPI()

	prompt := skill.GenerateCLIPrompt(entry, openAPI, skill.ModeFull)

	// petId is a required path param — should be marked required
	if !strings.Contains(prompt, "(required)") {
		t.Error("prompt should mark required parameters")
	}
}
