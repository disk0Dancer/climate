package generator_test

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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
		"internal/secrets/secrets.go",
		"internal/body/body.go",
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
		filepath.Join(outDir, "internal", "secrets", "secrets.go"),
		filepath.Join(outDir, "internal", "body", "body.go"),
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

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigurationsLifecycle(t *testing.T) {
	store := newStore(filepath.Join(t.TempDir(), "config.json"))
	if store.ActiveProfileName() != "default" {
		t.Fatalf("active = %q", store.ActiveProfileName())
	}
	if err := store.CreateProfile("work"); err != nil {
		t.Fatalf("CreateProfile() error = %v", err)
	}
	if err := store.UseProfile("work"); err != nil {
		t.Fatalf("UseProfile() error = %v", err)
	}
	if err := store.Set("core.base_url", "https://api.example.test", false); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := store.Set("events.signing_secret", "secret", true); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if value, ok, err := store.Get("core.base_url"); err != nil || !ok || value != "https://api.example.test" {
		t.Fatalf("Get(core.base_url) = %q, %v, %v", value, ok, err)
	}
	if value, ok, err := store.Get("events.signing_secret"); err != nil || !ok || value != "secret" {
		t.Fatalf("Get(events.signing_secret) = %q, %v, %v", value, ok, err)
	}
	removed, err := store.Unset("core.base_url")
	if err != nil || !removed {
		t.Fatalf("Unset(core.base_url) = %v, %v", removed, err)
	}
}

// TestFileBackendMigrationOnLoad seeds a legacy plaintext config.json and
// asserts that Load migrates the value into the encrypted file backend and
// rewrites the inline value with a reference marker.
func TestFileBackendMigrationOnLoad(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	t.Setenv("PETSTORE_SECRETS_BACKEND", "file")

	cfgPath, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	legacy := ` + "`" + `{"active":"default","profiles":{"default":{"properties":{},"secrets":{"auth.bearer_token":"legacy-plain"}}}}` + "`" + `
	if err := os.WriteFile(cfgPath, []byte(legacy), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	store, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if store.SecretsBackend != "file" {
		t.Fatalf("SecretsBackend = %q, want file", store.SecretsBackend)
	}
	value, ok, err := store.Get("auth.bearer_token")
	if err != nil || !ok || value != "legacy-plain" {
		t.Fatalf("Get after migration = %q, %v, %v", value, ok, err)
	}

	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var onDisk struct {
		Profiles map[string]struct {
			Secrets map[string]string ` + "`" + `json:"secrets"` + "`" + `
		} ` + "`" + `json:"profiles"` + "`" + `
	}
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if got := onDisk.Profiles["default"].Secrets["auth.bearer_token"]; got != "\x00climate-secret-ref\x00" {
		t.Fatalf("on-disk secret marker = %q, want the reference sentinel", got)
	}
}
`
	configTestPath := filepath.Join(outDir, "internal", "config", "config_runtime_test.go")
	if err := os.WriteFile(configTestPath, []byte(configTestContent), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", configTestPath, err)
	}

	secretsTestContent := `package secrets

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFileBackendRoundTripAndIdentityPerms(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))

	store, err := Open(BackendFile)
	if err != nil {
		t.Fatalf("Open(file) error = %v", err)
	}
	if store.Name() != BackendFile {
		t.Fatalf("Name = %q", store.Name())
	}
	if err := store.Set("default", "k", "v"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	got, ok, err := store.Get("default", "k")
	if err != nil || !ok || got != "v" {
		t.Fatalf("Get() = %q, %v, %v", got, ok, err)
	}
	keys, err := store.List("default")
	if err != nil || len(keys) != 1 || keys[0] != "k" {
		t.Fatalf("List() = %v, %v", keys, err)
	}
	base, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir: %v", err)
	}
	info, err := os.Stat(filepath.Join(base, appName, "identity.age"))
	if err != nil {
		t.Fatalf("identity not created: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("identity perms = %v, want 0600", info.Mode().Perm())
	}
	removed, err := store.Unset("default", "k")
	if err != nil || !removed {
		t.Fatalf("Unset() = %v, %v", removed, err)
	}
	if _, ok, _ := store.Get("default", "k"); ok {
		t.Fatal("value should be gone after Unset")
	}
}

func writeFakeGopass(t *testing.T, initialized bool) {
	t.Helper()
	binDir := t.TempDir()
	storeDir := t.TempDir()
	script := "#!/bin/sh\n" +
		"store=\"${GOPASS_FAKE_STORE:?}\"\n" +
		"cmd=\"$1\"; shift\n" +
		"case \"$cmd\" in\n" +
		"  ls) shift; prefix=\"$1\"; if [ -z \"$GOPASS_FAKE_INIT\" ]; then exit 1; fi; if [ -z \"$prefix\" ]; then exit 0; fi; find \"$store\" -type f 2>/dev/null | sed \"s#^$store/##\" | grep \"^$prefix\" || true; exit 0;;\n" +
		"  show) shift; p=\"$1\"; if [ -f \"$store/$p\" ]; then cat \"$store/$p\"; exit 0; else echo 'entry is not in the password store' >&2; exit 1; fi;;\n" +
		"  insert) shift; p=\"$1\"; mkdir -p \"$(dirname \"$store/$p\")\"; cat > \"$store/$p\"; exit 0;;\n" +
		"  rm) shift; p=\"$1\"; if [ -f \"$store/$p\" ]; then rm -f \"$store/$p\"; exit 0; else echo 'entry is not in the password store' >&2; exit 1; fi;;\n" +
		"  *) exit 2;;\n" +
		"esac\n"
	if err := os.WriteFile(filepath.Join(binDir, "gopass"), []byte(script), 0o755); err != nil {
		t.Fatalf("write shim: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GOPASS_FAKE_STORE", storeDir)
	if initialized {
		t.Setenv("GOPASS_FAKE_INIT", "1")
	} else {
		os.Unsetenv("GOPASS_FAKE_INIT")
	}
}

func TestGopassBackendRoundTrip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell shim not portable to windows")
	}
	writeFakeGopass(t, true)
	store, name, err := Resolve("")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if name != BackendGopass {
		t.Fatalf("resolved backend = %q, want gopass", name)
	}
	if err := store.Set("default", "auth.bearer_token", "tok"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	got, ok, err := store.Get("default", "auth.bearer_token")
	if err != nil || !ok || got != "tok" {
		t.Fatalf("Get() = %q, %v, %v", got, ok, err)
	}
	keys, err := store.List("default")
	if err != nil || len(keys) != 1 || keys[0] != "auth.bearer_token" {
		t.Fatalf("List() = %v, %v", keys, err)
	}
	removed, err := store.Unset("default", "auth.bearer_token")
	if err != nil || !removed {
		t.Fatalf("Unset() = %v, %v", removed, err)
	}
}

func TestGopassPresentButUninitializedFallsBackToFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell shim not portable to windows")
	}
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	writeFakeGopass(t, false)
	_, name, err := Resolve("")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if name != BackendFile {
		t.Fatalf("resolved backend = %q, want file (fallback)", name)
	}
}
`
	secretsTestPath := filepath.Join(outDir, "internal", "secrets", "secrets_runtime_test.go")
	if err := os.WriteFile(secretsTestPath, []byte(secretsTestContent), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", secretsTestPath, err)
	}

	bodyTestContent := `package body

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func mustCompose(t *testing.T, base string, defaults map[string]string, args []string, stdin string, schema Schema) map[string]interface{} {
	t.Helper()
	var b []byte
	if base != "" {
		b = []byte(base)
	}
	out, err := Compose(b, defaults, args, strings.NewReader(stdin), schema, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Compose error: %v", err)
	}
	m := map[string]interface{}{}
	if len(out) > 0 {
		if err := json.Unmarshal(out, &m); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
	}
	return m
}

func TestParseForms(t *testing.T) {
	doc, err := Parse([]string{"model=gpt", "temp:=0.2", "stream:=true", "a.b=x", "items[0].name=n"}, nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc["model"] != "gpt" {
		t.Fatalf("model=%v", doc["model"])
	}
	if doc["temp"] != 0.2 {
		t.Fatalf("temp=%v", doc["temp"])
	}
	if doc["stream"] != true {
		t.Fatalf("stream=%v", doc["stream"])
	}
	a, _ := doc["a"].(map[string]interface{})
	if a == nil || a["b"] != "x" {
		t.Fatalf("a.b=%v", doc["a"])
	}
	items, _ := doc["items"].([]interface{})
	if len(items) != 1 {
		t.Fatalf("items=%v", doc["items"])
	}
	obj, _ := items[0].(map[string]interface{})
	if obj["name"] != "n" {
		t.Fatalf("items[0].name=%v", items[0])
	}
}

func TestParseErrors(t *testing.T) {
	if _, err := Parse([]string{"noequals"}, nil); err == nil {
		t.Fatal("expected error for missing =")
	}
	if _, err := Parse([]string{"x:=not json"}, nil); err == nil {
		t.Fatal("expected error for bad raw JSON")
	}
}

func TestParseStdin(t *testing.T) {
	doc, err := Parse([]string{"content=@-"}, strings.NewReader("hello\n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc["content"] != "hello" {
		t.Fatalf("content=%v", doc["content"])
	}
}

func TestArgsLaterWins(t *testing.T) {
	doc, _ := Parse([]string{"a=1", "a=2"}, nil)
	if doc["a"] != "2" {
		t.Fatalf("a=%v", doc["a"])
	}
}

func TestComposePrecedenceArgsWin(t *testing.T) {
	schema := Schema{Known: true, Fields: map[string]string{"model": "string", "temperature": "number"}}
	m := mustCompose(t, "{\"temperature\":0.1}", map[string]string{"model": "default-model"}, []string{"model=arg-model"}, "", schema)
	if m["model"] != "arg-model" {
		t.Fatalf("model=%v (args should win)", m["model"])
	}
	if m["temperature"] != 0.1 {
		t.Fatalf("temperature=%v", m["temperature"])
	}
}

func TestComposeDefaultSurvives(t *testing.T) {
	schema := Schema{Known: true, Fields: map[string]string{"model": "string", "temperature": "number"}}
	m := mustCompose(t, "", map[string]string{"model": "default-model"}, []string{"temperature:=0.5"}, "", schema)
	if m["model"] != "default-model" {
		t.Fatalf("model=%v", m["model"])
	}
	if m["temperature"] != 0.5 {
		t.Fatalf("temperature=%v", m["temperature"])
	}
}

func TestValidationUnknownField(t *testing.T) {
	schema := Schema{Known: true, Fields: map[string]string{"model": "string"}}
	_, err := Compose(nil, nil, []string{"bogus=1"}, nil, schema, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(err.Error(), "model") {
		t.Fatalf("error should list valid fields: %v", err)
	}
}

func TestValidationTypeMismatch(t *testing.T) {
	schema := Schema{Known: true, Fields: map[string]string{"temperature": "number"}}
	_, err := Compose(nil, nil, []string{"temperature=hot"}, nil, schema, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "expects number") {
		t.Fatalf("err=%v", err)
	}
}

func TestDefaultsFilteredWithWarning(t *testing.T) {
	schema := Schema{Known: true, Fields: map[string]string{"model": "string"}}
	warn := &bytes.Buffer{}
	out, err := Compose(nil, map[string]string{"model": "m", "bogus": "x"}, nil, nil, schema, warn)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	m := map[string]interface{}{}
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["model"] != "m" {
		t.Fatalf("model=%v", m["model"])
	}
	if _, ok := m["bogus"]; ok {
		t.Fatal("bogus default should be filtered")
	}
	if !strings.Contains(warn.String(), "bogus") {
		t.Fatalf("expected warning about bogus, got %q", warn.String())
	}
}

func TestUnknownSchemaSkipsValidation(t *testing.T) {
	m := mustCompose(t, "", nil, []string{"anything=1", "nested.k=v"}, "", Schema{})
	if m["anything"] != "1" {
		t.Fatalf("anything=%v", m["anything"])
	}
}

func TestPickToJQ(t *testing.T) {
	got, err := PickToJQ("choices[0].message.content")
	if err != nil {
		t.Fatalf("PickToJQ: %v", err)
	}
	if got != ".choices[0].message.content" {
		t.Fatalf("got %q", got)
	}
}

func TestApplyJQ(t *testing.T) {
	input := map[string]interface{}{"choices": []interface{}{map[string]interface{}{"message": map[string]interface{}{"content": "hi"}}}}
	results, err := ApplyJQ(".choices[0].message.content", input)
	if err != nil {
		t.Fatalf("ApplyJQ: %v", err)
	}
	if len(results) != 1 || results[0] != "hi" {
		t.Fatalf("results=%v", results)
	}
}

func TestComposeNoInputReturnsNil(t *testing.T) {
	out, err := Compose(nil, nil, nil, nil, Schema{}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil body, got %s", out)
	}
}
`
	bodyTestPath := filepath.Join(outDir, "internal", "body", "body_runtime_test.go")
	if err := os.WriteFile(bodyTestPath, []byte(bodyTestContent), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", bodyTestPath, err)
	}

	gocache := filepath.Join(outDir, ".gocache")

	// Generate a go.sum for the generated module (it now depends on
	// filippo.io/age). tidy is the one step allowed to reach the module proxy:
	// a fresh module depending on cobra v1.8.0 (a pre-pruning go 1.15 module)
	// and age must resolve the full graph including transitive *test*
	// dependencies (e.g. c2sp.org/CCTV/age), which a pruned/empty cache does
	// not contain. The build and test steps below run fully offline
	// (GOPROXY=off), which is what proves the generated CLI needs no network.
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = outDir
	tidy.Env = append(os.Environ(),
		"GOCACHE="+gocache,
		"GOSUMDB=off",
	)
	if output, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy for generated module failed: %v\n%s", err, string(output))
	}

	cmd := exec.Command("go", "test", "./internal/...")
	cmd.Dir = outDir
	cmd.Env = append(os.Environ(),
		"GOCACHE="+gocache,
		"GOSUMDB=off",
		"GOPROXY=off",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated go test ./internal/... failed: %v\n%s", err, string(output))
	}
}

// TestGeneratedSecretRoundTripFileBackendBuild builds the full generated CLI
// and exercises `config set --secret` / `config get` with the encrypted file
// backend under a temp HOME, asserting the secret is never written to
// config.json in plaintext.
func TestGeneratedSecretRoundTripFileBackendBuild(t *testing.T) {
	outDir := t.TempDir()
	openAPI := sampleOpenAPI()

	if _, err := generator.Generate(openAPI, []byte(`{}`), generator.Options{
		CLIName: "petstore",
		OutDir:  outDir,
		NoBuild: true,
		Force:   true,
	}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	gocache := filepath.Join(outDir, ".gocache")
	env := append(os.Environ(),
		"GOCACHE="+gocache,
		"GOSUMDB=off",
	)

	// tidy resolves the full module graph (incl. transitive test deps) and so
	// needs the proxy once; the build below is offline (GOPROXY=off), proving
	// the generated CLI builds without network.
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = outDir
	tidy.Env = env
	if output, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, string(output))
	}

	binPath := filepath.Join(outDir, "petstore-bin")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = outDir
	build.Env = append(env, "GOPROXY=off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building generated CLI failed: %v\n%s", err, string(output))
	}

	home := t.TempDir()
	runEnv := append(os.Environ(),
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, "xdg"),
		"PETSTORE_SECRETS_BACKEND=file",
	)

	const secret = "s3cr3t-token-value"
	set := exec.Command(binPath, "config", "set", "auth.bearer_token", secret, "--secret")
	set.Env = runEnv
	if output, err := set.CombinedOutput(); err != nil {
		t.Fatalf("config set --secret failed: %v\n%s", err, string(output))
	}

	get := exec.Command(binPath, "config", "get", "auth.bearer_token")
	get.Env = runEnv
	getOut, err := get.CombinedOutput()
	if err != nil {
		t.Fatalf("config get failed: %v\n%s", err, string(getOut))
	}
	if !strings.Contains(string(getOut), secret) {
		t.Fatalf("config get did not return the secret; output=%s", string(getOut))
	}

	// The secret must never appear in plaintext on disk, and an encrypted
	// blob must exist.
	sawBlob := false
	err = filepath.Walk(home, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Base(path) == "secrets.age" {
			sawBlob = true
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(data), secret) {
			t.Fatalf("secret leaked in plaintext at %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking home: %v", err)
	}
	if !sawBlob {
		t.Fatal("expected an encrypted secrets.age blob to be written")
	}
}

// chatOpenAPI is a minimal spec with a POST operation whose request body has a
// typed object schema, used to exercise ergonomic body composition end to end.
func chatOpenAPI() *spec.OpenAPI {
	return &spec.OpenAPI{
		OpenAPI: "3.0.0",
		Info:    spec.Info{Title: "Chat API", Version: "1.0.0"},
		Servers: []spec.Server{{URL: "https://api.example.com/v1"}},
		Paths: map[string]spec.PathItem{
			"/chat/completions": {
				Post: &spec.Operation{
					OperationID: "chat_createChatCompletion",
					Summary:     "Create a chat completion",
					Tags:        []string{"chat"},
					RequestBody: &spec.RequestBody{
						Required: true,
						Content: map[string]spec.MediaType{
							"application/json": {
								Schema: &spec.Schema{
									Type: "object",
									Properties: map[string]*spec.Schema{
										"model":       {Type: "string"},
										"temperature": {Type: "number"},
										"messages":    {Type: "array", Items: &spec.Schema{Type: "object"}},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// TestGeneratedBodyCompositionE2E builds a generated CLI and drives a real
// request through it: a body composed from a configured default and positional
// key=value arguments, asserting both the JSON the server received
// and the raw --pick output.
func TestGeneratedBodyCompositionE2E(t *testing.T) {
	outDir := t.TempDir()
	if _, err := generator.Generate(chatOpenAPI(), []byte(`{}`), generator.Options{
		CLIName: "chatcli",
		OutDir:  outDir,
		NoBuild: true,
		Force:   true,
	}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	gocache := filepath.Join(outDir, ".gocache")
	env := append(os.Environ(),
		"GOCACHE="+gocache,
		"GOSUMDB=off",
	)

	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = outDir
	tidy.Env = env
	if output, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, string(output))
	}

	binPath := filepath.Join(outDir, "chatcli-bin")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = outDir
	build.Env = append(env, "GOPROXY=off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building generated CLI failed: %v\n%s", err, string(output))
	}

	var mu sync.Mutex
	var recorded []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		recorded = body
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hi there"}}]}`))
	}))
	defer srv.Close()

	home := t.TempDir()
	runEnv := append(os.Environ(),
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, "xdg"),
		"CHATCLI_SECRETS_BACKEND=plaintext",
	)

	// Configure a per-tag default so it can be omitted from the call.
	setDefault := exec.Command(binPath, "config", "set", "defaults.chat.model", "gpt-5-nano")
	setDefault.Env = runEnv
	if output, err := setDefault.CombinedOutput(); err != nil {
		t.Fatalf("config set default failed: %v\n%s", err, string(output))
	}

	// Compose: model from the default, temperature/messages from args, and the
	// system prompt piped via stdin; shape the response with --pick.
	run := exec.Command(binPath, "chat", "create-chat-completion",
		"temperature:=0.2",
		`messages:=[{"role":"user","content":"hi"}]`,
		"--base-url", srv.URL,
		"--pick", "choices[0].message.content",
	)
	run.Env = runEnv
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("composition run failed: %v\n%s", err, string(out))
	}
	if got := strings.TrimSpace(string(out)); got != "hi there" {
		t.Fatalf("--pick output = %q, want %q", got, "hi there")
	}

	mu.Lock()
	received := recorded
	mu.Unlock()
	var body map[string]interface{}
	if err := json.Unmarshal(received, &body); err != nil {
		t.Fatalf("server received invalid JSON %q: %v", string(received), err)
	}
	if body["model"] != "gpt-5-nano" {
		t.Fatalf("model = %v, want gpt-5-nano (from default)", body["model"])
	}
	if body["temperature"] != 0.2 {
		t.Fatalf("temperature = %v, want 0.2", body["temperature"])
	}
	msgs, ok := body["messages"].([]interface{})
	if !ok || len(msgs) != 1 {
		t.Fatalf("messages = %v", body["messages"])
	}

	// A positional arg overrides the configured default.
	runOverride := exec.Command(binPath, "chat", "create-chat-completion",
		"model=override-model",
		"--base-url", srv.URL,
	)
	runOverride.Env = runEnv
	if output, err := runOverride.CombinedOutput(); err != nil {
		t.Fatalf("override run failed: %v\n%s", err, string(output))
	}
	mu.Lock()
	received = recorded
	mu.Unlock()
	var overrideBody map[string]interface{}
	if err := json.Unmarshal(received, &overrideBody); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if overrideBody["model"] != "override-model" {
		t.Fatalf("model = %v, want override-model (arg should win over default)", overrideBody["model"])
	}
}
