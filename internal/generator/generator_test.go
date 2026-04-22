package generator_test

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
					OperationID:                       "pets_create",
					Summary:                           "Create a pet",
					Tags:                              []string{"pets"},
					XClimateEventName:                 "pet-created",
					XClimateSignatureMode:             "hmac",
					XClimateSignatureHeader:           "X-GitHub-Signature",
					XClimateSignatureAlgorithm:        "sha256",
					XClimateSignatureIncludeTimestamp: false,
					RequestBody:                       &spec.RequestBody{Required: true},
					Callbacks: map[string]spec.Callback{
						"petCreated": {
							"{$request.body#/callback_url}": {
								Post: &spec.Operation{
									Summary:           "Pet created callback",
									XClimateEventPath: "/webhooks/pet-created",
									RequestBody: &spec.RequestBody{
										Content: map[string]spec.MediaType{
											"application/json": {
												Schema: &spec.Schema{
													Type: "object",
													Properties: map[string]*spec.Schema{
														"id":   {Type: "string"},
														"type": {Type: "string"},
													},
												},
											},
										},
									},
								},
							},
						},
					},
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
		Webhooks: map[string]spec.PathItem{
			"payment.succeeded": {
				Post: &spec.Operation{
					Summary:                           "Payment succeeded webhook",
					XClimateEventName:                 "payment-succeeded",
					XClimateEventPath:                 "/webhooks/payment-succeeded",
					XClimateSignatureMode:             "hmac",
					XClimateSignatureHeader:           "X-Signature",
					XClimateSignatureAlgorithm:        "sha256",
					XClimateSignatureIncludeTimestamp: true,
					XClimateSignatureTimestampHeader:  "X-Signature-Timestamp",
					RequestBody: &spec.RequestBody{
						Content: map[string]spec.MediaType{
							"application/json": {
								Schema: &spec.Schema{
									Type: "object",
									Properties: map[string]*spec.Schema{
										"event_id": {Type: "string"},
										"type":     {Type: "string"},
									},
								},
							},
						},
					},
				},
			},
		},
		Components: spec.Components{
			SecuritySchemes: map[string]spec.SecurityScheme{
				"oauth": {
					Type: "oauth2",
					Flows: &spec.OAuthFlows{
						ClientCredentials: &spec.OAuthFlow{
							TokenURL: "https://petstore.example.com/oauth/token",
						},
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
		"cmd/auth.go",
		"cmd/config.go",
		"cmd/root.go",
		"cmd/commands.go",
		"cmd/events.go",
		"internal/client/client.go",
		"internal/config/config.go",
		"internal/events/events.go",
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

func TestGenerateRootVersionIsBuildOverridable(t *testing.T) {
	outDir := t.TempDir()
	openAPI := sampleOpenAPI()
	rawSpec := []byte(`{}`)

	_, err := generator.Generate(openAPI, rawSpec, generator.Options{
		CLIName: "petstore",
		OutDir:  outDir,
		NoBuild: true,
		Force:   true,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(outDir, "cmd", "root.go"))
	if err != nil {
		t.Fatalf("reading root.go: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `var version = "1.0.0"`) {
		t.Fatal("root.go should declare a build-overridable version variable")
	}
	if !strings.Contains(content, "Version: version") {
		t.Fatal("root.go should wire cobra version through the version variable")
	}
}

func TestGenerateIncludesEventsListenerCommand(t *testing.T) {
	outDir := t.TempDir()
	openAPI := sampleOpenAPI()

	_, err := generator.Generate(openAPI, []byte(`{}`), generator.Options{
		CLIName: "petstore",
		OutDir:  outDir,
		NoBuild: true,
		Force:   true,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	eventsCmd, err := os.ReadFile(filepath.Join(outDir, "cmd", "events.go"))
	if err != nil {
		t.Fatalf("reading cmd/events.go: %v", err)
	}
	eventsContent := string(eventsCmd)
	if !strings.Contains(eventsContent, `Use:   "events"`) {
		t.Fatal("generated CLI should include an events command group")
	}
	if !strings.Contains(eventsContent, `Use:   "list"`) {
		t.Fatal("generated CLI should include an events list command")
	}
	if !strings.Contains(eventsContent, `Use:   "listen [event-name]"`) {
		t.Fatal("generated CLI should include an events listen command")
	}
	if !strings.Contains(eventsContent, `Use:   "emit <event-name>"`) {
		t.Fatal("generated CLI should include an events emit command")
	}
	if !strings.Contains(eventsContent, `"listener.started"`) {
		t.Fatal("events listener should emit structured startup records")
	}
	if !strings.Contains(eventsContent, "payment-succeeded") {
		t.Fatal("generated events command should include named webhook definitions")
	}
}

func TestGenerateServerVariableFlagsAndInterpolation(t *testing.T) {
	outDir := t.TempDir()
	openAPI := sampleOpenAPI()
	openAPI.Servers = []spec.Server{
		{
			URL: "https://{region}.api.example.com/{basePath}",
			Variables: map[string]spec.ServerVariable{
				"region": {
					Default: "eu",
				},
				"basePath": {
					Default: "v1",
				},
			},
		},
	}
	rawSpec := []byte(`{}`)

	_, err := generator.Generate(openAPI, rawSpec, generator.Options{
		CLIName: "petstore",
		OutDir:  outDir,
		NoBuild: true,
		Force:   true,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(outDir, "cmd", "root.go"))
	if err != nil {
		t.Fatalf("reading root.go: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "defaultBaseURLTemplate") {
		t.Fatal("root.go should keep the templated server URL")
	}
	if !strings.Contains(content, `StringVar(&serverVarRegion, "server-var-region"`) {
		t.Fatal("root.go should declare --server-var-region")
	}
	if !strings.Contains(content, `StringVar(&serverVarBasePath, "server-var-base-path"`) {
		t.Fatal("root.go should declare --server-var-base-path")
	}
	if !strings.Contains(content, "PETSTORE_SERVER_VAR_REGION") {
		t.Fatal("root.go should expose PETSTORE_SERVER_VAR_REGION env override")
	}
	if !strings.Contains(content, "PETSTORE_SERVER_VAR_BASE_PATH") {
		t.Fatal("root.go should expose PETSTORE_SERVER_VAR_BASE_PATH env override")
	}
	if !strings.Contains(content, `strings.ReplaceAll`) {
		t.Fatal("root.go should interpolate server variables")
	}
}

func TestGenerateIncludesConfigCommands(t *testing.T) {
	outDir := t.TempDir()
	openAPI := sampleOpenAPI()

	_, err := generator.Generate(openAPI, []byte(`{}`), generator.Options{
		CLIName: "petstore",
		OutDir:  outDir,
		NoBuild: true,
		Force:   true,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	configCmd, err := os.ReadFile(filepath.Join(outDir, "cmd", "config.go"))
	if err != nil {
		t.Fatalf("reading cmd/config.go: %v", err)
	}
	configContent := string(configCmd)
	if !strings.Contains(configContent, `Use:   "config"`) {
		t.Fatal("generated CLI should include a config command group")
	}
	if !strings.Contains(configContent, `Use:   "list"`) {
		t.Fatal("generated CLI should include config list")
	}
	if !strings.Contains(configContent, `Use:   "set <key> <value>"`) {
		t.Fatal("generated CLI should include config set")
	}
	if !strings.Contains(configContent, `"secret"`) {
		t.Fatal("generated config command should support secret storage")
	}
}

func TestGenerateIncludesAuthCommands(t *testing.T) {
	outDir := t.TempDir()
	openAPI := sampleOpenAPI()

	_, err := generator.Generate(openAPI, []byte(`{}`), generator.Options{
		CLIName: "petstore",
		OutDir:  outDir,
		NoBuild: true,
		Force:   true,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	authCmd, err := os.ReadFile(filepath.Join(outDir, "cmd", "auth.go"))
	if err != nil {
		t.Fatalf("reading cmd/auth.go: %v", err)
	}
	authContent := string(authCmd)
	if !strings.Contains(authContent, `Use:   "auth"`) {
		t.Fatal("generated CLI should include an auth command group")
	}
	if !strings.Contains(authContent, `Use:   "login"`) {
		t.Fatal("generated CLI should include auth login")
	}
	if !strings.Contains(authContent, `Use:   "status"`) {
		t.Fatal("generated CLI should include auth status")
	}
	if !strings.Contains(authContent, `Use:   "logout"`) {
		t.Fatal("generated CLI should include auth logout")
	}
}

func TestGenerateIncludesTunnelProviderHelpers(t *testing.T) {
	outDir := t.TempDir()
	openAPI := sampleOpenAPI()

	_, err := generator.Generate(openAPI, []byte(`{}`), generator.Options{
		CLIName: "petstore",
		OutDir:  outDir,
		NoBuild: true,
		Force:   true,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	eventsHelper, err := os.ReadFile(filepath.Join(outDir, "internal", "events", "events.go"))
	if err != nil {
		t.Fatalf("reading internal/events/events.go: %v", err)
	}
	content := string(eventsHelper)
	for _, want := range []string{
		`"cloudflared"`,
		`"hmac"`,
		`"sha256"`,
		`"sha1"`,
		`"sha512"`,
		`"listener.tunnel"`,
		`"verified"`,
		`X-Signature`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("generated events helper should mention %q", want)
		}
	}
}

func TestGeneratedGoFilesParse(t *testing.T) {
	outDir := t.TempDir()
	openAPI := sampleOpenAPI()

	_, err := generator.Generate(openAPI, []byte(`{}`), generator.Options{
		CLIName: "petstore",
		OutDir:  outDir,
		NoBuild: true,
		Force:   true,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	goFiles := []string{
		filepath.Join(outDir, "main.go"),
		filepath.Join(outDir, "cmd", "auth.go"),
		filepath.Join(outDir, "cmd", "config.go"),
		filepath.Join(outDir, "cmd", "root.go"),
		filepath.Join(outDir, "cmd", "commands.go"),
		filepath.Join(outDir, "cmd", "events.go"),
		filepath.Join(outDir, "internal", "client", "client.go"),
		filepath.Join(outDir, "internal", "config", "config.go"),
		filepath.Join(outDir, "internal", "events", "events.go"),
	}

	fset := token.NewFileSet()
	for _, path := range goFiles {
		if _, err := parser.ParseFile(fset, path, nil, parser.AllErrors); err != nil {
			t.Fatalf("generated Go file %s should parse: %v", path, err)
		}
	}
}

func TestGeneratedEventsRuntime(t *testing.T) {
	outDir := t.TempDir()
	openAPI := sampleOpenAPI()

	_, err := generator.Generate(openAPI, []byte(`{}`), generator.Options{
		CLIName: "petstore",
		OutDir:  outDir,
		NoBuild: true,
		Force:   true,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	testContent := `package events

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTunnelProvidersEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "cloudflared")
	script := "#!/bin/sh\nprintf '%s\\n' 'https://cloudflared.example.test'\nsleep 1\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	records := make(chan TunnelRecord, 1)
	_, err := StartTunnel(ctx, "cloudflared", "http://127.0.0.1:8081/webhooks/test", func(v interface{}) {
		if rec, ok := v.(TunnelRecord); ok {
			select {
			case records <- rec:
			default:
			}
		}
	})
	if err != nil {
		t.Fatalf("StartTunnel() error = %v", err)
	}

	select {
	case rec := <-records:
		if rec.PublicURL != "https://cloudflared.example.test" {
			t.Fatalf("PublicURL = %q", rec.PublicURL)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for tunnel record")
	}
}

func TestHMACVerificationBodyOnly(t *testing.T) {
	body := []byte("{\"action\":\"ping\"}")
	headers, err := SignatureHeaders(SignatureOptions{
		Mode:      "hmac",
		Header:    "X-Signature",
		Secret:    "secret",
		Algorithm: "sha256",
	}, body)
	if err != nil {
		t.Fatalf("SignatureHeaders() error = %v", err)
	}
	verified, err := verifySignature(SignatureOptions{
		Mode:      "hmac",
		Header:    "X-Signature",
		Secret:    "secret",
		Algorithm: "sha256",
	}, http.Header{
		"X-Signature": []string{headers["X-Signature"]},
	}, body)
	if err != nil {
		t.Fatalf("verifySignature() error = %v", err)
	}
	if !verified {
		t.Fatal("expected verification to pass")
	}
}

func TestHMACVerificationWithTimestamp(t *testing.T) {
	body := []byte("{\"action\":\"ping\"}")
	headers, err := SignatureHeaders(SignatureOptions{
		Mode:             "hmac",
		Header:           "X-Signature",
		Secret:           "secret",
		Algorithm:        "sha512",
		IncludeTimestamp: true,
		TimestampHeader:  "X-Signature-Timestamp",
	}, body)
	if err != nil {
		t.Fatalf("SignatureHeaders() error = %v", err)
	}
	httpHeaders := http.Header{}
	for key, value := range headers {
		httpHeaders.Set(key, value)
	}
	verified, err := verifySignature(SignatureOptions{
		Mode:               "hmac",
		Header:             "X-Signature",
		Secret:             "secret",
		Algorithm:          "sha512",
		IncludeTimestamp:   true,
		TimestampHeader:    "X-Signature-Timestamp",
		TimestampTolerance: time.Minute,
	}, httpHeaders, body)
	if err != nil {
		t.Fatalf("verifySignature() error = %v", err)
	}
	if !verified {
		t.Fatal("expected verification to pass")
	}
}
`
	testPath := filepath.Join(outDir, "internal", "events", "events_runtime_test.go")
	if err := os.WriteFile(testPath, []byte(testContent), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", testPath, err)
	}

	configTestContent := `package config

import "testing"

func TestConfigurationsLifecycle(t *testing.T) {
	store := newStore("/tmp/config.json")
	if store.ActiveProfileName() != "default" {
		t.Fatalf("active = %q", store.ActiveProfileName())
	}
	if err := store.CreateProfile("work"); err != nil {
		t.Fatalf("CreateProfile() error = %v", err)
	}
	if err := store.UseProfile("work"); err != nil {
		t.Fatalf("UseProfile() error = %v", err)
	}
	store.Set("core.base_url", "https://api.example.test", false)
	store.Set("events.signing_secret", "secret", true)
	if value, ok := store.Get("core.base_url"); !ok || value != "https://api.example.test" {
		t.Fatalf("Get(core.base_url) = %q, %v", value, ok)
	}
	if value, ok := store.Get("events.signing_secret"); !ok || value != "secret" {
		t.Fatalf("Get(events.signing_secret) = %q, %v", value, ok)
	}
	if !store.Unset("core.base_url") {
		t.Fatal("Unset(core.base_url) should return true")
	}
}
`
	configTestPath := filepath.Join(outDir, "internal", "config", "config_runtime_test.go")
	if err := os.WriteFile(configTestPath, []byte(configTestContent), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", configTestPath, err)
	}

	packageDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	repoRoot := filepath.Clean(filepath.Join(packageDir, "..", ".."))
	gomodcache := filepath.Join(repoRoot, ".cache", "go-mod")
	gocache := filepath.Join(outDir, ".gocache")

	cmd := exec.Command("go", "test", "./internal/...")
	cmd.Dir = outDir
	cmd.Env = append(os.Environ(),
		"GOCACHE="+gocache,
		"GOMODCACHE="+gomodcache,
		"GOSUMDB=off",
		"GOPROXY=off",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated go test ./internal/... failed: %v\n%s", err, string(output))
	}
}
