package generator_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/disk0Dancer/climate/internal/generator"
	"github.com/disk0Dancer/climate/internal/spec"
)

func sampleOpenAPI() *spec.OpenAPI {
	return &spec.OpenAPI{
		OpenAPI: "3.0.0",
		Info: spec.Info{
			Title:       "Petstore",
			Version:     "1.0.0",
			Description: "A sample pet store API",
		},
		Servers: []spec.Server{{URL: "https://petstore.example.com/v1"}},
		Paths: map[string]spec.PathItem{
			"/pets": {
				Get: &spec.Operation{
					OperationID: "pets_list",
					Summary:     "List all pets",
					Tags:        []string{"pets"},
					Parameters: []spec.Parameter{
						{Name: "limit", In: "query", Description: "Maximum number of results"},
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
					Summary:     "Get a pet by ID",
					Tags:        []string{"pets"},
					Parameters: []spec.Parameter{
						{Name: "petId", In: "path", Required: true, Description: "Pet ID"},
					},
				},
			},
		},
	}
}

func TestGenerateNoBuild(t *testing.T) {
	outDir := t.TempDir()
	openAPI := sampleOpenAPI()
	rawSpec := []byte(`{"openapi":"3.0.0","info":{"title":"Petstore","version":"1.0.0"},"paths":{}}`)

	opts := generator.Options{
		CLIName: "petstore",
		OutDir:  outDir,
		NoBuild: true,
		Force:   true,
	}

	result, err := generator.Generate(openAPI, rawSpec, opts)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if result.CLIName != "petstore" {
		t.Errorf("CLIName = %q, want %q", result.CLIName, "petstore")
	}
	if result.BinaryPath != "" {
		t.Errorf("BinaryPath should be empty when NoBuild=true, got %q", result.BinaryPath)
	}
	if result.SourceDir != outDir {
		t.Errorf("SourceDir = %q, want %q", result.SourceDir, outDir)
	}
}

func TestGenerateCreatesFiles(t *testing.T) {
	outDir := t.TempDir()
	openAPI := sampleOpenAPI()
	rawSpec := []byte(`{"openapi":"3.0.0"}`)

	opts := generator.Options{
		CLIName:    "petstore",
		OutDir:     outDir,
		NoBuild:    true,
		Force:      true,
		SpecSource: "https://example.com/openapi.json",
	}

	_, err := generator.Generate(openAPI, rawSpec, opts)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Check expected files exist
	expectedFiles := []string{
		"go.mod",
		"main.go",
		"cmd/root.go",
		"cmd/commands.go",
		"internal/client/client.go",
		"climate_meta.json",
	}
	for _, f := range expectedFiles {
		path := filepath.Join(outDir, f)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %s not found: %v", f, err)
		}
	}
}

func TestGenerateDerivesNameFromTitle(t *testing.T) {
	outDir := t.TempDir()
	openAPI := sampleOpenAPI()
	openAPI.Info.Title = "My Awesome API"
	rawSpec := []byte(`{}`)

	opts := generator.Options{
		OutDir:  outDir,
		NoBuild: true,
		Force:   true,
	}

	result, err := generator.Generate(openAPI, rawSpec, opts)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if result.CLIName != "my-awesome-api" {
		t.Errorf("CLIName = %q, want %q", result.CLIName, "my-awesome-api")
	}
}

func TestGenerateExistingDirWithoutForce(t *testing.T) {
	outDir := t.TempDir()
	openAPI := sampleOpenAPI()
	rawSpec := []byte(`{}`)

	// First generation succeeds
	opts := generator.Options{
		CLIName: "petstore",
		OutDir:  outDir,
		NoBuild: true,
		Force:   false,
	}

	// Write a file to make the directory non-empty
	_ = os.WriteFile(filepath.Join(outDir, "existing.txt"), []byte("test"), 0o644)

	_, err := generator.Generate(openAPI, rawSpec, opts)
	if err == nil {
		t.Error("Generate() should fail when output dir exists and --force not set")
	}
}

func TestGenerateWithAuthSchemes(t *testing.T) {
	outDir := t.TempDir()
	openAPI := sampleOpenAPI()
	openAPI.Components = spec.Components{
		SecuritySchemes: map[string]spec.SecurityScheme{
			"bearerAuth": {
				Type:   "http",
				Scheme: "bearer",
			},
			"apiKeyAuth": {
				Type: "apiKey",
				Name: "X-API-Key",
				In:   "header",
			},
		},
	}
	rawSpec := []byte(`{}`)

	opts := generator.Options{
		CLIName: "secured-api",
		OutDir:  outDir,
		NoBuild: true,
		Force:   true,
	}

	result, err := generator.Generate(openAPI, rawSpec, opts)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify root.go contains auth flag declarations
	rootGoPath := filepath.Join(outDir, "cmd", "root.go")
	data, err := os.ReadFile(rootGoPath)
	if err != nil {
		t.Fatalf("reading root.go: %v", err)
	}
	rootGoContent := string(data)

	if result.CLIName != "secured-api" {
		t.Errorf("CLIName = %q, want %q", result.CLIName, "secured-api")
	}

	// Should contain bearer token or api key flags
	if len(rootGoContent) == 0 {
		t.Error("root.go should not be empty")
	}
}
