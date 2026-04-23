// Package generator produces Go CLI source code from an OpenAPI specification.
package generator

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"
	"unicode"

	"github.com/disk0Dancer/climate/internal/auth"
	"github.com/disk0Dancer/climate/internal/mock"
	"github.com/disk0Dancer/climate/internal/spec"
)

// Version is the current climate version.
const Version = "0.1.0"

//go:embed templates/*
var templateFS embed.FS

// Options configures the code generator.
type Options struct {
	CLIName    string
	OutDir     string
	NoBuild    bool
	Force      bool
	SpecSource string
}

// Result holds information about a generated CLI.
type Result struct {
	CLIName     string `json:"cli_name"`
	BinaryPath  string `json:"binary_path"`
	SourceDir   string `json:"source_dir"`
	Version     string `json:"version"`
	OpenAPIHash string `json:"openapi_hash"`
}

// Meta is written alongside generated sources.
type Meta struct {
	CLIName        string `json:"cli_name"`
	OpenAPIHash    string `json:"openapi_hash"`
	GeneratedAt    string `json:"generated_at"`
	ClimateVersion string `json:"climate_version"`
	SpecSource     string `json:"spec_source,omitempty"`
}

type eventDefinition struct {
	Name                      string
	DisplayName               string
	Source                    string
	Expression                string
	DefaultPath               string
	Methods                   []string
	DefaultMethod             string
	Summary                   string
	Description               string
	SampleJSON                string
	SignatureMode             string
	SignatureHeader           string
	SignatureAlgorithm        string
	SignatureIncludeTimestamp bool
	SignatureTimestampHeader  string
}

type authSchemeDefinition struct {
	Name                 string
	ConfigKey            string
	Type                 string
	AuthorizationURL     string
	TokenURL             string
	HasClientCredentials bool
	HasPasswordFlow      bool
	HasAuthorizationCode bool
	HasImplicitFlow      bool
}

func renderTemplate(name string, data interface{}) (string, error) {
	tmpl, err := template.ParseFS(templateFS, "templates/"+name)
	if err != nil {
		return "", fmt.Errorf("parse template %s: %w", name, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}
	return buf.String(), nil
}

// Generate generates a Go CLI project from an OpenAPI spec and optionally builds it.
func Generate(openAPI *spec.OpenAPI, rawSpec []byte, opts Options) (*Result, error) {
	if opts.CLIName == "" {
		opts.CLIName = spec.NormalizeName(openAPI.Info.Title)
	}

	outDir := opts.OutDir
	if outDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("finding home directory: %w", err)
		}
		outDir = filepath.Join(home, ".climate", "src", opts.CLIName)
	}

	if !opts.Force {
		if _, err := os.Stat(outDir); err == nil {
			return nil, fmt.Errorf("output directory %s already exists; use --force to overwrite", outDir)
		}
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating output directory: %w", err)
	}

	hash := spec.HashBytes(rawSpec)

	if err := generateFiles(openAPI, opts.CLIName, outDir, hash, opts.SpecSource); err != nil {
		return nil, err
	}

	binaryPath := ""
	if !opts.NoBuild {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("finding home directory: %w", err)
		}
		binDir := filepath.Join(home, ".climate", "bin")
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			return nil, fmt.Errorf("creating bin directory: %w", err)
		}
		binaryPath = filepath.Join(binDir, opts.CLIName)

		// Run go mod tidy first, then build
		if err := runGoCmd(outDir, "go", "mod", "tidy"); err != nil {
			return nil, fmt.Errorf("go mod tidy: %w", err)
		}
		if err := buildBinary(outDir, binaryPath); err != nil {
			return nil, err
		}
	}

	return &Result{
		CLIName:     opts.CLIName,
		BinaryPath:  binaryPath,
		SourceDir:   outDir,
		Version:     openAPI.Info.Version,
		OpenAPIHash: hash,
	}, nil
}

// generateFiles writes all Go source files for the CLI project.
func generateFiles(openAPI *spec.OpenAPI, cliName, outDir, hash, specSource string) error {
	schemes := auth.ParseSchemes(openAPI)
	eventDefs, err := extractEventDefinitions(openAPI)
	if err != nil {
		return err
	}
	authDefs := extractAuthSchemeDefinitions(schemes)

	// Write go.mod
	if err := writeFile(filepath.Join(outDir, "go.mod"), goModContent(cliName)); err != nil {
		return err
	}

	// Write main.go
	mainContent, err := mainGoContent(cliName)
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(outDir, "main.go"), mainContent); err != nil {
		return err
	}

	// Write cmd/root.go
	cmdDir := filepath.Join(outDir, "cmd")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		return err
	}
	rootContent, err := rootGoContent(openAPI, cliName, schemes)
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(cmdDir, "root.go"), rootContent); err != nil {
		return err
	}

	// Write cmd/commands.go
	cmdContent, err := commandsGoContent(openAPI, cliName)
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(cmdDir, "commands.go"), cmdContent); err != nil {
		return err
	}

	// Write cmd/events.go
	eventsContent, err := eventsGoContent(cliName, eventDefs)
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(cmdDir, "events.go"), eventsContent); err != nil {
		return err
	}

	// Write cmd/config.go
	configCmdContent, err := configGoContent(cliName)
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(cmdDir, "config.go"), configCmdContent); err != nil {
		return err
	}

	// Write cmd/auth.go when auth schemes exist.
	if len(authDefs) > 0 {
		authCmdContent, err := authGoContent(cliName, authDefs)
		if err != nil {
			return err
		}
		if err := writeFile(filepath.Join(cmdDir, "auth.go"), authCmdContent); err != nil {
			return err
		}
	}

	// Write internal/client/client.go
	clientDir := filepath.Join(outDir, "internal", "client")
	if err := os.MkdirAll(clientDir, 0o755); err != nil {
		return err
	}
	clientContent, err := clientGoContent(openAPI)
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(clientDir, "client.go"), clientContent); err != nil {
		return err
	}

	// Write internal/config/config.go
	configDir := filepath.Join(outDir, "internal", "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}
	configContent, err := internalConfigGoContent(cliName)
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(configDir, "config.go"), configContent); err != nil {
		return err
	}

	// Write internal/events/events.go
	eventsDir := filepath.Join(outDir, "internal", "events")
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		return err
	}
	internalEventsContent, err := internalEventsGoContent()
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(eventsDir, "events.go"), internalEventsContent); err != nil {
		return err
	}

	// Write climate_meta.json
	meta := Meta{
		CLIName:        cliName,
		OpenAPIHash:    hash,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		ClimateVersion: Version,
		SpecSource:     specSource,
	}
	metaJSON, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(outDir, "climate_meta.json"), string(metaJSON))
}

func extractEventDefinitions(openAPI *spec.OpenAPI) ([]eventDefinition, error) {
	if openAPI == nil {
		return nil, fmt.Errorf("openapi spec is nil")
	}

	defs := []eventDefinition{}
	usedNames := map[string]int{}

	webhookNames := make([]string, 0, len(openAPI.Webhooks))
	for name := range openAPI.Webhooks {
		webhookNames = append(webhookNames, name)
	}
	sort.Strings(webhookNames)

	for _, name := range webhookNames {
		def, err := buildEventDefinition(openAPI, spec.NormalizeName(name), name, "webhook", "", "/webhooks/"+spec.NormalizeName(name), openAPI.Webhooks[name], openAPI, openAPI.Webhooks[name], openAPI.Webhooks[name].Post)
		if err != nil {
			return nil, err
		}
		def.Name = dedupeEventName(def.Name, usedNames)
		defs = append(defs, def)
	}

	pathKeys := make([]string, 0, len(openAPI.Paths))
	for path := range openAPI.Paths {
		pathKeys = append(pathKeys, path)
	}
	sort.Strings(pathKeys)

	for _, path := range pathKeys {
		item := openAPI.Paths[path]
		methods := make([]string, 0, len(item.Operations()))
		for method := range item.Operations() {
			methods = append(methods, method)
		}
		sort.Strings(methods)

		for _, method := range methods {
			op := item.Operations()[method]
			if op == nil || len(op.Callbacks) == 0 {
				continue
			}

			callbackNames := make([]string, 0, len(op.Callbacks))
			for callbackName := range op.Callbacks {
				callbackNames = append(callbackNames, callbackName)
			}
			sort.Strings(callbackNames)

			for _, callbackName := range callbackNames {
				callback := op.Callbacks[callbackName]
				expressions := make([]string, 0, len(callback))
				for expression := range callback {
					expressions = append(expressions, expression)
				}
				sort.Strings(expressions)
				for _, expression := range expressions {
					prefix := callbackEventPrefix(op, method, path)
					baseName := spec.NormalizeName(prefix + "-" + callbackName)
					defaultPath := callbackDefaultPath(callbackName, expression)
					callbackItem := callback[expression]
					def, err := buildEventDefinition(openAPI, baseName, callbackName, "callback", expression, defaultPath, callbackItem, openAPI, callbackItem, callbackItem.Post)
					if err != nil {
						return nil, err
					}
					def.Name = dedupeEventName(def.Name, usedNames)
					defs = append(defs, def)
				}
			}
		}
	}

	return defs, nil
}

func extractAuthSchemeDefinitions(schemes []auth.Scheme) []authSchemeDefinition {
	defs := make([]authSchemeDefinition, 0, len(schemes))
	for _, scheme := range schemes {
		def := authSchemeDefinition{
			Name:      scheme.Name,
			ConfigKey: spec.NormalizeName(scheme.Name),
			Type:      string(scheme.Type),
		}
		if scheme.Spec.Flows != nil {
			if flow := scheme.Spec.Flows.ClientCredentials; flow != nil {
				def.HasClientCredentials = true
				if def.TokenURL == "" {
					def.TokenURL = flow.TokenURL
				}
			}
			if flow := scheme.Spec.Flows.Password; flow != nil {
				def.HasPasswordFlow = true
				if def.TokenURL == "" {
					def.TokenURL = flow.TokenURL
				}
			}
			if flow := scheme.Spec.Flows.AuthorizationCode; flow != nil {
				def.HasAuthorizationCode = true
				if def.AuthorizationURL == "" {
					def.AuthorizationURL = flow.AuthorizationURL
				}
				if def.TokenURL == "" {
					def.TokenURL = flow.TokenURL
				}
			}
			if flow := scheme.Spec.Flows.Implicit; flow != nil {
				def.HasImplicitFlow = true
				if def.AuthorizationURL == "" {
					def.AuthorizationURL = flow.AuthorizationURL
				}
			}
		}
		defs = append(defs, def)
	}
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name < defs[j].Name
	})
	return defs
}

func buildEventDefinition(openAPI *spec.OpenAPI, name, displayName, source, expression, defaultPath string, item spec.PathItem, root *spec.OpenAPI, pathItem spec.PathItem, op *spec.Operation) (eventDefinition, error) {
	methods := make([]string, 0, len(item.Operations()))
	for method := range item.Operations() {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	if len(methods) == 0 {
		return eventDefinition{}, fmt.Errorf("%s %q has no operations", source, displayName)
	}

	defaultMethod := methods[0]
	op = item.Operations()[defaultMethod]
	summary := displayName
	if op != nil && op.Summary != "" {
		summary = op.Summary
	}
	description := ""
	if op != nil {
		description = op.Description
	}

	sampleJSON := "{}"
	if op != nil {
		payload, err := mock.GeneratePayloadForOperation(openAPI, op)
		if err == nil {
			if data, marshalErr := json.Marshal(payload); marshalErr == nil {
				sampleJSON = string(data)
			}
		}
	}

	metadata := resolveEventMetadata(root, pathItem, op)

	return eventDefinition{
		Name:                      firstNonEmpty(metadata.EventName, name),
		DisplayName:               displayName,
		Source:                    source,
		Expression:                expression,
		DefaultPath:               firstNonEmpty(metadata.EventPath, defaultPath),
		Methods:                   methods,
		DefaultMethod:             defaultMethod,
		Summary:                   summary,
		Description:               description,
		SampleJSON:                sampleJSON,
		SignatureMode:             metadata.SignatureMode,
		SignatureHeader:           metadata.SignatureHeader,
		SignatureAlgorithm:        metadata.SignatureAlgorithm,
		SignatureIncludeTimestamp: metadata.SignatureIncludeTimestamp,
		SignatureTimestampHeader:  metadata.SignatureTimestampHeader,
	}, nil
}

type eventMetadata struct {
	EventName                 string
	EventPath                 string
	SignatureMode             string
	SignatureHeader           string
	SignatureAlgorithm        string
	SignatureIncludeTimestamp bool
	SignatureTimestampHeader  string
}

func resolveEventMetadata(root *spec.OpenAPI, pathItem spec.PathItem, op *spec.Operation) eventMetadata {
	meta := eventMetadata{}
	if root != nil {
		meta.EventName = root.XClimateEventName
		meta.EventPath = root.XClimateEventPath
		meta.SignatureMode = root.XClimateSignatureMode
		meta.SignatureHeader = root.XClimateSignatureHeader
		meta.SignatureAlgorithm = root.XClimateSignatureAlgorithm
		meta.SignatureIncludeTimestamp = root.XClimateSignatureIncludeTimestamp
		meta.SignatureTimestampHeader = root.XClimateSignatureTimestampHeader
	}
	if pathItem.XClimateEventName != "" {
		meta.EventName = pathItem.XClimateEventName
	}
	if pathItem.XClimateEventPath != "" {
		meta.EventPath = pathItem.XClimateEventPath
	}
	if pathItem.XClimateSignatureMode != "" {
		meta.SignatureMode = pathItem.XClimateSignatureMode
	}
	if pathItem.XClimateSignatureHeader != "" {
		meta.SignatureHeader = pathItem.XClimateSignatureHeader
	}
	if pathItem.XClimateSignatureAlgorithm != "" {
		meta.SignatureAlgorithm = pathItem.XClimateSignatureAlgorithm
	}
	if pathItem.XClimateSignatureIncludeTimestamp {
		meta.SignatureIncludeTimestamp = true
	}
	if pathItem.XClimateSignatureTimestampHeader != "" {
		meta.SignatureTimestampHeader = pathItem.XClimateSignatureTimestampHeader
	}
	if op != nil {
		if op.XClimateEventName != "" {
			meta.EventName = op.XClimateEventName
		}
		if op.XClimateEventPath != "" {
			meta.EventPath = op.XClimateEventPath
		}
		if op.XClimateSignatureMode != "" {
			meta.SignatureMode = op.XClimateSignatureMode
		}
		if op.XClimateSignatureHeader != "" {
			meta.SignatureHeader = op.XClimateSignatureHeader
		}
		if op.XClimateSignatureAlgorithm != "" {
			meta.SignatureAlgorithm = op.XClimateSignatureAlgorithm
		}
		if op.XClimateSignatureIncludeTimestamp {
			meta.SignatureIncludeTimestamp = true
		}
		if op.XClimateSignatureTimestampHeader != "" {
			meta.SignatureTimestampHeader = op.XClimateSignatureTimestampHeader
		}
	}
	return meta
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func callbackEventPrefix(op *spec.Operation, method, path string) string {
	if op != nil && op.OperationID != "" {
		return spec.NormalizeName(op.OperationID)
	}
	return spec.NormalizeName(strings.ToLower(method) + "-" + strings.Trim(path, "/"))
}

func callbackDefaultPath(callbackName, expression string) string {
	if expression != "" {
		if parsed, ok := staticCallbackPath(expression); ok {
			return parsed
		}
	}
	return "/callbacks/" + spec.NormalizeName(callbackName)
}

func staticCallbackPath(expression string) (string, bool) {
	trimmed := strings.TrimSpace(expression)
	if trimmed == "" {
		return "", false
	}
	if strings.HasPrefix(trimmed, "/") {
		return trimmed, true
	}
	if strings.Contains(trimmed, "://") {
		parts := strings.SplitN(trimmed, "://", 2)
		if len(parts) == 2 {
			rest := parts[1]
			if slash := strings.Index(rest, "/"); slash >= 0 {
				return rest[slash:], true
			}
		}
	}
	return "", false
}

func dedupeEventName(name string, used map[string]int) string {
	if used[name] == 0 {
		used[name] = 1
		return name
	}
	used[name]++
	return fmt.Sprintf("%s-%d", name, used[name])
}

func buildBinary(sourceDir, outputPath string) error {
	return runGoCmd(sourceDir, "go", "build", "-o", outputPath, ".")
}

func runGoCmd(dir string, args ...string) error {
	cmd := exec.Command(args[0], args[1:]...) //nolint:gosec
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w\n%s", strings.Join(args, " "), err, string(out))
	}
	return nil
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

// goModContent returns the go.mod content for a generated CLI.
func goModContent(cliName string) string {
	return fmt.Sprintf(`module %s

go 1.21

require github.com/spf13/cobra v1.8.0

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
)
`, cliName)
}

// mainGoContent returns the main.go content for a generated CLI.
func mainGoContent(cliName string) (string, error) {
	return renderTemplate("main.go.tmpl", struct {
		CLIName string
	}{
		CLIName: cliName,
	})
}

// rootGoContent returns the cmd/root.go content.
func rootGoContent(openAPI *spec.OpenAPI, cliName string, schemes []auth.Scheme) (string, error) {
	cliUpper := strings.ToUpper(strings.ReplaceAll(cliName, "-", "_"))

	var authVarDecls strings.Builder
	var authFlagInits strings.Builder
	var authHeadersBody strings.Builder
	var authQueryBody strings.Builder
	var serverVarDecls strings.Builder
	var serverVarFlagInits strings.Builder
	var serverVarResolveBody strings.Builder

	seenVars := map[string]bool{}

	// Track which imports we actually need
	needsBase64 := false
	needsNetHTTP := false
	needsIOUtil := false
	needsStrings := false

	for _, scheme := range schemes {
		switch scheme.Type {

		case auth.SchemeAPIKey:
			varName := safeIdent(camelCase(scheme.Name) + "APIKey")
			envVar := cliUpper + "_" + strings.ToUpper(strings.ReplaceAll(scheme.Name, "-", "_")) + "_API_KEY"
			configKey := "auth.api_keys." + spec.NormalizeName(scheme.Name)
			if !seenVars[varName] {
				seenVars[varName] = true
				flagName := kebabCase(scheme.Name) + "-key"
				_, _ = fmt.Fprintf(&authVarDecls, "\t%s string\n", varName)
				_, _ = fmt.Fprintf(&authFlagInits,
					"\trootCmd.PersistentFlags().StringVar(&%s, %q, \"\", %q)\n",
					varName, flagName, "API key for "+scheme.Name,
				)
				keyExpr := fmt.Sprintf(`
	if %s == "" {
		%s = getConfigValue(%q)
	}
	if %s == "" {
		%s = os.Getenv(%q)
	}`, varName, varName, configKey, varName, varName, envVar)
				switch scheme.Spec.In {
				case "header":
					authHeadersBody.WriteString(keyExpr)
					_, _ = fmt.Fprintf(&authHeadersBody, `
	if %s != "" {
		headers[%q] = %s
	}
`, varName, scheme.Spec.Name, varName)
				case "query":
					authQueryBody.WriteString(keyExpr)
					_, _ = fmt.Fprintf(&authQueryBody, `
	if %s != "" {
		params[%q] = %s
	}
`, varName, scheme.Spec.Name, varName)
				case "cookie":
					authHeadersBody.WriteString(keyExpr)
					_, _ = fmt.Fprintf(&authHeadersBody, `
	if %s != "" {
		if existing, ok := headers["Cookie"]; ok {
			headers["Cookie"] = existing + "; " + %q + "=" + %s
		} else {
			headers["Cookie"] = %q + "=" + %s
		}
	}
`, varName, scheme.Spec.Name, varName, scheme.Spec.Name, varName)
				}
			}

		case auth.SchemeHTTPBearer:
			if !seenVars["bearerToken"] {
				seenVars["bearerToken"] = true
				envVar := cliUpper + "_TOKEN"
				authVarDecls.WriteString("\tbearerToken string\n")
				authFlagInits.WriteString("\trootCmd.PersistentFlags().StringVar(&bearerToken, \"token\", \"\", \"Bearer token for authentication\")\n")
				_, _ = fmt.Fprintf(&authHeadersBody, `
	{
		tok := bearerToken
		if tok == "" {
			tok = getConfigValue("auth.bearer_token")
		}
		if tok == "" {
			tok = os.Getenv(%q)
		}
		if tok != "" {
			headers["Authorization"] = "Bearer " + tok
		}
	}
`, envVar)
			}

		case auth.SchemeHTTPBasic:
			if !seenVars["username"] {
				seenVars["username"] = true
				needsBase64 = true
				authVarDecls.WriteString("\tusername string\n")
				authVarDecls.WriteString("\tpassword string\n")
				authFlagInits.WriteString("\trootCmd.PersistentFlags().StringVar(&username, \"username\", \"\", \"Username for basic auth\")\n")
				authFlagInits.WriteString("\trootCmd.PersistentFlags().StringVar(&password, \"password\", \"\", \"Password for basic auth\")\n")
				_, _ = fmt.Fprintf(&authHeadersBody, `
	{
		u := username
		if u == "" {
			u = getConfigValue("auth.basic_username")
		}
		if u == "" {
			u = os.Getenv(%q)
		}
		p := password
		if p == "" {
			p = getConfigValue("auth.basic_password")
		}
		if p == "" {
			p = os.Getenv(%q)
		}
		if u != "" {
			headers["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(u+":"+p))
		}
	}
`, cliUpper+"_USERNAME", cliUpper+"_PASSWORD")
			}

		case auth.SchemeOAuth2:
			if !seenVars["clientID"] {
				seenVars["clientID"] = true
				needsNetHTTP = true
				needsIOUtil = true

				// Collect tokenURLs from all flows
				tokenURL := ""
				if scheme.Spec.Flows != nil {
					if scheme.Spec.Flows.ClientCredentials != nil {
						tokenURL = scheme.Spec.Flows.ClientCredentials.TokenURL
					} else if scheme.Spec.Flows.Password != nil {
						tokenURL = scheme.Spec.Flows.Password.TokenURL
					}
				}

				authVarDecls.WriteString("\toauth2Token  string\n")
				authVarDecls.WriteString("\tclientID     string\n")
				authVarDecls.WriteString("\tclientSecret string\n")
				authFlagInits.WriteString("\trootCmd.PersistentFlags().StringVar(&oauth2Token, \"token\", \"\", \"OAuth2 access token (overrides client credentials flow)\")\n")
				authFlagInits.WriteString("\trootCmd.PersistentFlags().StringVar(&clientID, \"client-id\", \"\", \"OAuth2 client ID\")\n")
				authFlagInits.WriteString("\trootCmd.PersistentFlags().StringVar(&clientSecret, \"client-secret\", \"\", \"OAuth2 client secret\")\n")
				_, _ = fmt.Fprintf(&authHeadersBody, `
	{
		tok := oauth2Token
		if tok == "" {
			tok = getConfigValue("auth.oauth2_token")
		}
		if tok == "" {
			tok = os.Getenv(%q)
		}
		if tok == "" {
			cid := clientID
			if cid == "" {
				cid = getConfigValue("auth.oauth2_client_id")
			}
			if cid == "" {
				cid = os.Getenv(%q)
			}
			csec := clientSecret
			if csec == "" {
				csec = getConfigValue("auth.oauth2_client_secret")
			}
			if csec == "" {
				csec = os.Getenv(%q)
			}
			if cid != "" && csec != "" {
				if t, err := fetchOAuth2Token(%q, cid, csec); err == nil {
					tok = t
				}
			}
		}
		if tok != "" {
			headers["Authorization"] = "Bearer " + tok
		}
	}
`,
					cliUpper+"_TOKEN",
					cliUpper+"_CLIENT_ID",
					cliUpper+"_CLIENT_SECRET",
					tokenURL,
				)
			}
		}
	}

	baseURL := ""
	serverVariables := map[string]spec.ServerVariable{}
	if len(openAPI.Servers) > 0 {
		baseURL = openAPI.Servers[0].URL
		serverVariables = openAPI.Servers[0].Variables
	}

	serverVarKeys := make([]string, 0, len(serverVariables))
	for name := range serverVariables {
		serverVarKeys = append(serverVarKeys, name)
	}
	sort.Strings(serverVarKeys)
	for _, name := range serverVarKeys {
		sv := serverVariables[name]
		varName := safeIdent("serverVar" + toPascal(name))
		flagName := "server-var-" + kebabCase(name)
		envVar := spec.ServerVariableEnvName(cliUpper, name)
		desc := sv.Description
		if desc == "" {
			desc = "Override server URL variable {" + name + "}"
		}
		serverVarDecls.WriteString(fmt.Sprintf("\t%s string\n", varName))
		serverVarFlagInits.WriteString(fmt.Sprintf(
			"\trootCmd.PersistentFlags().StringVar(&%s, %q, \"\", %q)\n",
			varName, flagName, desc,
		))
		serverVarResolveBody.WriteString(fmt.Sprintf(`
	{
		v := %s
		if v == "" {
			v = os.Getenv(%q)
		}
		if v == "" {
			v = %q
		}
		u = strings.ReplaceAll(u, %q, v)
	}
`, varName, envVar, sv.Default, "{"+name+"}"))
	}
	if len(serverVarKeys) > 0 {
		needsStrings = true
	}

	// Build the import list
	var imports strings.Builder
	imports.WriteString("\t\"encoding/json\"\n")
	imports.WriteString("\t\"fmt\"\n")
	if needsIOUtil || needsNetHTTP {
		imports.WriteString("\t\"io\"\n")
	}
	if needsNetHTTP {
		imports.WriteString("\t\"net/http\"\n")
		imports.WriteString("\t\"net/url\"\n")
	}
	if needsNetHTTP || needsStrings {
		imports.WriteString("\t\"strings\"\n")
	}
	imports.WriteString("\t\"os\"\n")
	if needsBase64 {
		imports.WriteString("\t\"encoding/base64\"\n")
	}
	imports.WriteString("\n\t\"github.com/spf13/cobra\"\n")

	description := openAPI.Info.Description
	if description == "" {
		description = openAPI.Info.Title + " CLI"
	}

	defaultBaseURLResolver := "\treturn defaultBaseURLTemplate\n"
	if len(serverVarKeys) > 0 {
		defaultBaseURLResolver = "	u := defaultBaseURLTemplate\n" + serverVarResolveBody.String() + "\treturn u\n"
	}

	// OAuth2 helper function — only emitted when needed
	oauth2Helper := ""
	if needsNetHTTP {
		oauth2Helper = `
// fetchOAuth2Token obtains an access token via the client_credentials flow.
func fetchOAuth2Token(tokenURL, clientID, clientSecret string) (string, error) {
	if tokenURL == "" {
		return "", fmt.Errorf("tokenUrl is empty")
	}
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	resp, err := http.PostForm(tokenURL, data)
	if err != nil {
		return "", fmt.Errorf("fetching token: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading token response: %w", err)
	}
	var result struct {
		AccessToken string ` + "`json:\"access_token\"`" + `
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing token response: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("empty access_token in response")
	}
	return result.AccessToken, nil
}
`
	}

	// Determine if base64 import is needed only
	base64Import := ""
	if needsBase64 && !needsNetHTTP {
		// base64 already added to imports above
		_ = needsBase64
	}
	_ = base64Import

	return renderTemplate("root.go.tmpl", struct {
		Imports                string
		AuthVarDecls           string
		ServerVarDecls         string
		DefaultBaseURLTemplate string
		Version                string
		CLIName                string
		Description            string
		AuthFlagInits          string
		ServerVarFlagInits     string
		BaseURLEnv             string
		ResolveDefaultBaseURL  string
		AuthHeaders            string
		AuthQuery              string
		OAuth2Helper           string
	}{
		Imports:                imports.String(),
		AuthVarDecls:           authVarDecls.String(),
		ServerVarDecls:         serverVarDecls.String(),
		DefaultBaseURLTemplate: baseURL,
		Version:                openAPI.Info.Version,
		CLIName:                cliName,
		Description:            description,
		AuthFlagInits:          authFlagInits.String(),
		ServerVarFlagInits:     serverVarFlagInits.String(),
		BaseURLEnv:             cliUpper + "_BASE_URL",
		ResolveDefaultBaseURL:  defaultBaseURLResolver,
		AuthHeaders:            authHeadersBody.String(),
		AuthQuery:              authQueryBody.String(),
		OAuth2Helper:           oauth2Helper,
	})
}

// commandsGoContent generates the cobra subcommands for all operations.
func commandsGoContent(openAPI *spec.OpenAPI, cliName string) (string, error) {
	type opInfo struct {
		Tag        string
		SubCmdName string
		Method     string
		Path       string
		Summary    string
		Parameters []spec.Parameter
		HasBody    bool
	}

	tagOps := map[string][]opInfo{}
	tagOrder := []string{}
	seenTags := map[string]bool{}

	for path, item := range openAPI.Paths {
		for method, op := range item.Operations() {
			tag := "default"
			if len(op.Tags) > 0 {
				tag = op.Tags[0]
			}
			if !seenTags[tag] {
				seenTags[tag] = true
				tagOrder = append(tagOrder, tag)
			}
			subCmd := operationSubCommand(op, method, path)
			oi := opInfo{
				Tag:        tag,
				SubCmdName: subCmd,
				Method:     method,
				Path:       path,
				Summary:    op.Summary,
				Parameters: op.Parameters,
				HasBody:    op.RequestBody != nil,
			}
			tagOps[tag] = append(tagOps[tag], oi)
		}
	}

	// Sort tags and operations for deterministic output across runs.
	sort.Strings(tagOrder)
	for tag := range tagOps {
		ops := tagOps[tag]
		sort.Slice(ops, func(i, j int) bool {
			if ops[i].SubCmdName != ops[j].SubCmdName {
				return ops[i].SubCmdName < ops[j].SubCmdName
			}
			if ops[i].Method != ops[j].Method {
				return ops[i].Method < ops[j].Method
			}
			return ops[i].Path < ops[j].Path
		})
		// Resolve duplicate subcommand names within a tag by appending the HTTP method.
		seen := map[string]int{}
		for _, op := range ops {
			seen[op.SubCmdName]++
		}
		for k := range ops {
			if seen[ops[k].SubCmdName] > 1 {
				ops[k].SubCmdName = ops[k].SubCmdName + "-" + strings.ToLower(ops[k].Method)
			}
		}
		tagOps[tag] = ops
	}

	var sb strings.Builder
	sb.WriteString("package cmd\n\n")
	sb.WriteString("import (\n")
	sb.WriteString("\t\"fmt\"\n")
	sb.WriteString("\t\"os\"\n")
	sb.WriteString("\t\"strings\"\n\n")
	_, _ = fmt.Fprintf(&sb, "\t\"%s/internal/client\"\n", cliName)
	sb.WriteString("\t\"github.com/spf13/cobra\"\n")
	sb.WriteString(")\n\n")

	// Declare all flag variables at package level to avoid redeclaration
	sb.WriteString("var (\n")
	sb.WriteString("\t// flag variable declarations for all subcommands\n")

	varCounter := map[string]int{}

	// Map from (tag, subCmd, paramName) -> varName
	varMap := map[string]string{}

	for _, tag := range tagOrder {
		ops := tagOps[tag]
		for _, op := range ops {
			opKey := tag + "_" + op.SubCmdName + "_" + op.Method
			for _, p := range op.Parameters {
				key := opKey + "_" + p.Name
				base := camelCase(p.Name)
				varCounter[base]++
				varName := fmt.Sprintf("%s%s%s%d", camelCase(tag), toPascal(op.SubCmdName), toPascal(p.Name), varCounter[base])
				varMap[key] = varName
				sb.WriteString(fmt.Sprintf("\t%s string\n", varName))
			}
			if op.HasBody {
				dataJSONKey := opKey + "__dataJSON"
				dataFileKey := opKey + "__dataFile"
				dataJSONVar := fmt.Sprintf("%s%sDataJSON", camelCase(tag), toPascal(op.SubCmdName))
				dataFileVar := fmt.Sprintf("%s%sDataFile", camelCase(tag), toPascal(op.SubCmdName))
				varMap[dataJSONKey] = dataJSONVar
				varMap[dataFileKey] = dataFileVar
				sb.WriteString(fmt.Sprintf("\t%s string\n", dataJSONVar))
				sb.WriteString(fmt.Sprintf("\t%s string\n", dataFileVar))
			}
		}
	}
	sb.WriteString(")\n\n")

	// Build init function
	sb.WriteString("func init() {\n")

	for _, tag := range tagOrder {
		ops := tagOps[tag]
		tagCmdVar := "cmd" + toPascal(tag)
		sb.WriteString(fmt.Sprintf("\t%s := &cobra.Command{\n", tagCmdVar))
		sb.WriteString(fmt.Sprintf("\t\tUse:   %q,\n", toKebab(tag)))
		sb.WriteString(fmt.Sprintf("\t\tShort: %q,\n", "Operations for "+tag))
		sb.WriteString("\t}\n")
		sb.WriteString(fmt.Sprintf("\trootCmd.AddCommand(%s)\n\n", tagCmdVar))

		for _, op := range ops {
			opKey := tag + "_" + op.SubCmdName + "_" + op.Method
			subCmdVar := "cmd" + toPascal(tag) + toPascal(op.SubCmdName) + toPascal(op.Method)

			short := op.Summary
			if short == "" {
				short = fmt.Sprintf("%s %s", op.Method, op.Path)
			}

			sb.WriteString(fmt.Sprintf("\t%s := &cobra.Command{\n", subCmdVar))
			sb.WriteString(fmt.Sprintf("\t\tUse:   %q,\n", op.SubCmdName))
			sb.WriteString(fmt.Sprintf("\t\tShort: %q,\n", short))
			sb.WriteString("\t\tRunE: func(cmd *cobra.Command, args []string) error {\n")

			// Build path with replacements
			sb.WriteString(fmt.Sprintf("\t\t\tpath := %q\n", op.Path))
			for _, p := range op.Parameters {
				if p.In == "path" {
					key := opKey + "_" + p.Name
					varName := varMap[key]
					sb.WriteString(fmt.Sprintf("\t\t\tif %s == \"\" {\n", varName))
					sb.WriteString(fmt.Sprintf("\t\t\t\texitWithError(0, \"CliError\", %q, nil)\n",
						"Missing required parameter: --"+kebabCase(p.Name)))
					sb.WriteString("\t\t\t}\n")
					sb.WriteString(fmt.Sprintf("\t\t\tpath = strings.ReplaceAll(path, %q, %s)\n", "{"+p.Name+"}", varName))
				}
			}

			// Build query params
			sb.WriteString("\t\t\tqueryParams := map[string]string{}\n")
			for _, p := range op.Parameters {
				if p.In == "query" {
					key := opKey + "_" + p.Name
					varName := varMap[key]
					sb.WriteString(fmt.Sprintf("\t\t\tif %s != \"\" {\n", varName))
					sb.WriteString(fmt.Sprintf("\t\t\t\tqueryParams[%q] = %s\n", p.Name, varName))
					sb.WriteString("\t\t\t}\n")
				}
			}

			// Body handling
			bodyArg := "nil"
			if op.HasBody {
				dataJSONKey := opKey + "__dataJSON"
				dataFileKey := opKey + "__dataFile"
				dataJSONVar := varMap[dataJSONKey]
				dataFileVar := varMap[dataFileKey]
				bodyArg = "bodyData"
				sb.WriteString("\t\t\tvar bodyData []byte\n")
				sb.WriteString(fmt.Sprintf("\t\t\tif %s != \"\" {\n", dataJSONVar))
				sb.WriteString(fmt.Sprintf("\t\t\t\tbodyData = []byte(%s)\n", dataJSONVar))
				sb.WriteString(fmt.Sprintf("\t\t\t} else if %s != \"\" {\n", dataFileVar))
				sb.WriteString("\t\t\t\tvar readErr error\n")
				sb.WriteString(fmt.Sprintf("\t\t\t\tbodyData, readErr = os.ReadFile(%s)\n", dataFileVar))
				sb.WriteString("\t\t\t\tif readErr != nil {\n")
				sb.WriteString("\t\t\t\t\treturn fmt.Errorf(\"reading data file: %w\", readErr)\n")
				sb.WriteString("\t\t\t\t}\n")
				sb.WriteString("\t\t\t}\n")
			}

			// Build per-request headers from header parameters
			hasHeaderParams := false
			for _, p := range op.Parameters {
				if p.In == "header" {
					hasHeaderParams = true
					break
				}
			}
			if hasHeaderParams {
				sb.WriteString("\t\t\treqHeaders := map[string]string{}\n")
				for _, p := range op.Parameters {
					if p.In == "header" {
						key := opKey + "_" + p.Name
						varName := varMap[key]
						sb.WriteString(fmt.Sprintf("\t\t\tif %s != \"\" {\n", varName))
						sb.WriteString(fmt.Sprintf("\t\t\t\treqHeaders[%q] = %s\n", p.Name, varName))
						sb.WriteString("\t\t\t}\n")
					}
				}
			}

			// Merge auth query params (e.g. apiKey in:query) into per-request params
			sb.WriteString("\t\t\tfor k, v := range getAuthQueryParams() {\n")
			sb.WriteString("\t\t\t\tqueryParams[k] = v\n")
			sb.WriteString("\t\t\t}\n")
			sb.WriteString("\t\t\tc := client.NewClient(getBaseURL(), getAuthHeaders())\n")
			if hasHeaderParams {
				sb.WriteString(fmt.Sprintf("\t\t\tresp, err := c.Do(%q, path, queryParams, %s, reqHeaders)\n", op.Method, bodyArg))
			} else {
				sb.WriteString(fmt.Sprintf("\t\t\tresp, err := c.Do(%q, path, queryParams, %s)\n", op.Method, bodyArg))
			}
			sb.WriteString("\t\t\tif err != nil {\n")
			sb.WriteString("\t\t\t\texitWithError(0, \"CliError\", err.Error(), nil)\n")
			sb.WriteString("\t\t\t}\n")
			sb.WriteString("\t\t\tif resp.StatusCode >= 400 {\n")
			sb.WriteString("\t\t\t\texitWithError(resp.StatusCode, \"HTTPError\", resp.Body, resp.Raw)\n")
			sb.WriteString("\t\t\t}\n")
			sb.WriteString("\t\t\twriteOutput(resp.Raw)\n")
			sb.WriteString("\t\t\treturn nil\n")
			sb.WriteString("\t\t},\n")
			sb.WriteString("\t}\n")

			// Add flags
			for _, p := range op.Parameters {
				key := opKey + "_" + p.Name
				varName := varMap[key]
				sb.WriteString(fmt.Sprintf("\t%s.Flags().StringVar(&%s, %q, \"\", %q)\n",
					subCmdVar, varName, kebabCase(p.Name), p.Description))
			}
			if op.HasBody {
				dataJSONKey := opKey + "__dataJSON"
				dataFileKey := opKey + "__dataFile"
				sb.WriteString(fmt.Sprintf("\t%s.Flags().StringVar(&%s, \"data-json\", \"\", \"JSON body data\")\n",
					subCmdVar, varMap[dataJSONKey]))
				sb.WriteString(fmt.Sprintf("\t%s.Flags().StringVar(&%s, \"data-file\", \"\", \"Path to JSON file\")\n",
					subCmdVar, varMap[dataFileKey]))
			}

			sb.WriteString(fmt.Sprintf("\t%s.AddCommand(%s)\n\n", tagCmdVar, subCmdVar))
		}
	}

	sb.WriteString("}\n")

	return sb.String(), nil
}

// eventsGoContent generates the cobra commands for event handling.
func eventsGoContent(cliName string, defs []eventDefinition) (string, error) {
	return renderTemplate("events.go.tmpl", struct {
		CLIName          string
		EventDefinitions string
		WebhookSecretEnv string
	}{
		CLIName:          cliName,
		EventDefinitions: eventDefinitionsLiteral(defs),
		WebhookSecretEnv: strings.ToUpper(strings.ReplaceAll(cliName, "-", "_")) + "_WEBHOOK_SECRET",
	})
}

func eventDefinitionsLiteral(defs []eventDefinition) string {
	var sb strings.Builder
	for _, def := range defs {
		methods := make([]string, 0, len(def.Methods))
		for _, method := range def.Methods {
			methods = append(methods, fmt.Sprintf("%q", method))
		}
		_, _ = fmt.Fprintf(&sb, "\t{\n")
		_, _ = fmt.Fprintf(&sb, "\t\tName: %q,\n", def.Name)
		_, _ = fmt.Fprintf(&sb, "\t\tDisplayName: %q,\n", def.DisplayName)
		_, _ = fmt.Fprintf(&sb, "\t\tSource: %q,\n", def.Source)
		_, _ = fmt.Fprintf(&sb, "\t\tExpression: %q,\n", def.Expression)
		_, _ = fmt.Fprintf(&sb, "\t\tDefaultPath: %q,\n", def.DefaultPath)
		_, _ = fmt.Fprintf(&sb, "\t\tMethods: []string{%s},\n", strings.Join(methods, ", "))
		_, _ = fmt.Fprintf(&sb, "\t\tDefaultMethod: %q,\n", def.DefaultMethod)
		_, _ = fmt.Fprintf(&sb, "\t\tSummary: %q,\n", def.Summary)
		_, _ = fmt.Fprintf(&sb, "\t\tDescription: %q,\n", def.Description)
		_, _ = fmt.Fprintf(&sb, "\t\tSampleJSON: %q,\n", def.SampleJSON)
		_, _ = fmt.Fprintf(&sb, "\t\tSignatureMode: %q,\n", def.SignatureMode)
		_, _ = fmt.Fprintf(&sb, "\t\tSignatureHeader: %q,\n", def.SignatureHeader)
		_, _ = fmt.Fprintf(&sb, "\t\tSignatureAlgorithm: %q,\n", def.SignatureAlgorithm)
		_, _ = fmt.Fprintf(&sb, "\t\tSignatureIncludeTimestamp: %t,\n", def.SignatureIncludeTimestamp)
		_, _ = fmt.Fprintf(&sb, "\t\tSignatureTimestampHeader: %q,\n", def.SignatureTimestampHeader)
		_, _ = fmt.Fprintf(&sb, "\t},\n")
	}
	return sb.String()
}

// internalEventsGoContent generates helpers for local event listening.
func internalEventsGoContent() (string, error) {
	return renderTemplate("internal_events.go.tmpl", nil)
}

func configGoContent(cliName string) (string, error) {
	return renderTemplate("config.go.tmpl", struct {
		CLIName string
	}{
		CLIName: cliName,
	})
}

func internalConfigGoContent(cliName string) (string, error) {
	return renderTemplate("internal_config.go.tmpl", struct {
		CLIName string
	}{
		CLIName: cliName,
	})
}

func authGoContent(cliName string, defs []authSchemeDefinition) (string, error) {
	return renderTemplate("auth.go.tmpl", struct {
		CLIName     string
		AuthSchemes string
	}{
		CLIName:     cliName,
		AuthSchemes: authSchemeDefinitionsLiteral(defs),
	})
}

// clientGoContent generates the internal/client/client.go content.
func clientGoContent(openAPI *spec.OpenAPI) (string, error) {
	baseURL := ""
	if len(openAPI.Servers) > 0 {
		baseURL = openAPI.Servers[0].URL
	}

	return renderTemplate("client.go.tmpl", struct {
		DefaultBaseURL string
	}{
		DefaultBaseURL: baseURL,
	})
}

func authSchemeDefinitionsLiteral(defs []authSchemeDefinition) string {
	var sb strings.Builder
	for _, def := range defs {
		_, _ = fmt.Fprintf(&sb, "\t{\n")
		_, _ = fmt.Fprintf(&sb, "\t\tName: %q,\n", def.Name)
		_, _ = fmt.Fprintf(&sb, "\t\tConfigKey: %q,\n", def.ConfigKey)
		_, _ = fmt.Fprintf(&sb, "\t\tType: %q,\n", def.Type)
		_, _ = fmt.Fprintf(&sb, "\t\tAuthorizationURL: %q,\n", def.AuthorizationURL)
		_, _ = fmt.Fprintf(&sb, "\t\tTokenURL: %q,\n", def.TokenURL)
		_, _ = fmt.Fprintf(&sb, "\t\tHasClientCredentials: %t,\n", def.HasClientCredentials)
		_, _ = fmt.Fprintf(&sb, "\t\tHasPasswordFlow: %t,\n", def.HasPasswordFlow)
		_, _ = fmt.Fprintf(&sb, "\t\tHasAuthorizationCode: %t,\n", def.HasAuthorizationCode)
		_, _ = fmt.Fprintf(&sb, "\t\tHasImplicitFlow: %t,\n", def.HasImplicitFlow)
		_, _ = fmt.Fprintf(&sb, "\t},\n")
	}
	return sb.String()
}

// --- Naming helpers ---

// operationSubCommand derives a subcommand name for an operation.
func operationSubCommand(op *spec.Operation, method, path string) string {
	if op.OperationID != "" {
		parts := strings.SplitN(op.OperationID, "_", 2)
		if len(parts) == 2 {
			return toKebab(parts[1])
		}
		return toKebab(op.OperationID)
	}
	segments := strings.Split(strings.Trim(path, "/"), "/")
	hasParam := false
	for _, s := range segments {
		if strings.HasPrefix(s, "{") {
			hasParam = true
			break
		}
	}
	switch strings.ToUpper(method) {
	case "GET":
		if hasParam {
			return "get"
		}
		return "list"
	case "POST":
		return "create"
	case "PUT":
		// PUT is a full replacement; use "update". If both PUT and PATCH exist under
		// the same tag without an operationId, the duplicate-resolution pass above
		// will append the HTTP method suffix (e.g. "update-put", "patch-patch").
		return "update"
	case "PATCH":
		// PATCH is a partial update; keep a distinct name from PUT.
		return "patch"
	case "DELETE":
		return "delete"
	default:
		return strings.ToLower(method)
	}
}

// toKebab converts a string to kebab-case.
func toKebab(s string) string {
	var result strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 && result.Len() > 0 {
				str := result.String()
				if str[len(str)-1] != '-' {
					result.WriteRune('-')
				}
			}
			result.WriteRune(unicode.ToLower(r))
		} else if r == '_' || r == ' ' || r == '-' || r == '/' {
			if result.Len() > 0 {
				str := result.String()
				if str[len(str)-1] != '-' {
					result.WriteRune('-')
				}
			}
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// toPascal converts a string to PascalCase.
func toPascal(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_' || r == ' ' || r == '/'
	})
	var result strings.Builder
	for _, p := range parts {
		if len(p) > 0 {
			result.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return result.String()
}

// camelCase converts a string to camelCase.
func camelCase(s string) string {
	p := toPascal(s)
	if len(p) == 0 {
		return p
	}
	return strings.ToLower(p[:1]) + p[1:]
}

// kebabCase converts a string to all-lowercase kebab-case.
func kebabCase(s string) string {
	return strings.ToLower(toKebab(s))
}

// safeIdent ensures a string is a valid Go identifier.
func safeIdent(s string) string {
	if s == "" {
		return "v"
	}
	var b strings.Builder
	for i, r := range s {
		if i == 0 && unicode.IsDigit(r) {
			b.WriteRune('v')
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			b.WriteRune(r)
		}
	}
	result := b.String()
	if result == "" {
		return "v"
	}
	return result
}
