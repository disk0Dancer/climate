// Package generator produces Go CLI source code from an OpenAPI specification.
package generator

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/disk0Dancer/climate/internal/auth"
	"github.com/disk0Dancer/climate/internal/spec"
)

// Version is the current climate version.
const Version = "0.1.0"

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

	// Write go.mod
	if err := writeFile(filepath.Join(outDir, "go.mod"), goModContent(cliName)); err != nil {
		return err
	}

	// Write main.go
	if err := writeFile(filepath.Join(outDir, "main.go"), mainGoContent(cliName)); err != nil {
		return err
	}

	// Write cmd/root.go
	cmdDir := filepath.Join(outDir, "cmd")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(cmdDir, "root.go"), rootGoContent(openAPI, cliName, schemes)); err != nil {
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

	// Write internal/client/client.go
	clientDir := filepath.Join(outDir, "internal", "client")
	if err := os.MkdirAll(clientDir, 0o755); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(clientDir, "client.go"), clientGoContent(openAPI)); err != nil {
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
func mainGoContent(cliName string) string {
	return fmt.Sprintf(`package main

import (
	"fmt"
	"os"

	"%s/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
`, cliName)
}

// rootGoContent returns the cmd/root.go content.
func rootGoContent(openAPI *spec.OpenAPI, cliName string, schemes []auth.Scheme) string {
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
		%s = os.Getenv(%q)
	}`, varName, varName, envVar)
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
			u = os.Getenv(%q)
		}
		p := password
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
			tok = os.Getenv(%q)
		}
		if tok == "" {
			cid := clientID
			if cid == "" {
				cid = os.Getenv(%q)
			}
			csec := clientSecret
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

	return fmt.Sprintf(`package cmd

import (
%s)

var (
	outputFormat string
	baseURL      string
%s%s)

const defaultBaseURLTemplate = %q

var version = %q

var rootCmd = &cobra.Command{
	Use:     %q,
	Short:   %q,
	Version: version,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&outputFormat, "output", "json", "Output format: json|table|raw")
	rootCmd.PersistentFlags().StringVar(&baseURL, "base-url", "", "Override API base URL")
%s%s}

func getBaseURL() string {
	if baseURL != "" {
		return baseURL
	}
	if v := os.Getenv(%q); v != "" {
		return v
	}
	return resolveDefaultBaseURL()
}

func resolveDefaultBaseURL() string {
%s
}

// getAuthHeaders returns HTTP headers required for authentication.
// Priority: CLI flag → environment variable → empty.
func getAuthHeaders() map[string]string {
	headers := map[string]string{}
%s
	return headers
}

// getAuthQueryParams returns query parameters required for authentication
// (used when an API key scheme has in: query).
func getAuthQueryParams() map[string]string {
	params := map[string]string{}
%s
	return params
}

// writeOutput prints v as indented JSON to stdout.
func writeOutput(v interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintln(os.Stderr, "error encoding output:", err)
		os.Exit(1)
	}
}

// exitWithError prints an error as JSON to stderr and exits non-zero.
func exitWithError(statusCode int, code, message string, raw interface{}) {
	type errObj struct {
		Status  int         `+"`json:\"status\"`"+`
		Code    string      `+"`json:\"code\"`"+`
		Message string      `+"`json:\"message\"`"+`
		Raw     interface{} `+"`json:\"raw,omitempty\"`"+`
	}
	type errorWrapper struct {
		Error errObj `+"`json:\"error\"`"+`
	}
	obj := errorWrapper{Error: errObj{Status: statusCode, Code: code, Message: message, Raw: raw}}
	enc := json.NewEncoder(os.Stderr)
	enc.SetIndent("", "  ")
	_ = enc.Encode(obj)
	os.Exit(1)
	}
%s`,
		imports.String(),
		authVarDecls.String(),
		serverVarDecls.String(),
		baseURL,
		openAPI.Info.Version,
		cliName,
		description,
		authFlagInits.String(),
		serverVarFlagInits.String(),
		cliUpper+"_BASE_URL",
		defaultBaseURLResolver,
		authHeadersBody.String(),
		authQueryBody.String(),
		oauth2Helper,
	)
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

// clientGoContent generates the internal/client/client.go content.
func clientGoContent(openAPI *spec.OpenAPI) string {
	baseURL := ""
	if len(openAPI.Servers) > 0 {
		baseURL = openAPI.Servers[0].URL
	}

	return fmt.Sprintf(`package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBaseURL is the default server URL.
const DefaultBaseURL = %q

// Client is an HTTP API client.
type Client struct {
	BaseURL    string
	Headers    map[string]string
	HTTPClient *http.Client
}

// Response holds an API response.
type Response struct {
	StatusCode int
	Body       string
	Raw        interface{}
}

// NewClient creates a new Client.
func NewClient(baseURL string, headers map[string]string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Headers:    headers,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Do executes an HTTP request.
func (c *Client) Do(method, path string, query map[string]string, body []byte, extraHeaders ...map[string]string) (*Response, error) {
	fullURL := c.BaseURL + path
	if len(query) > 0 {
		params := url.Values{}
		for k, v := range query {
			params.Set(k, v)
		}
		fullURL += "?" + params.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %%w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	for _, eh := range extraHeaders {
		for k, v := range eh {
			req.Header.Set(k, v)
		}
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %%w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %%w", err)
	}

	var raw interface{}
	if len(respBody) > 0 {
		_ = json.Unmarshal(respBody, &raw)
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       string(respBody),
		Raw:        raw,
	}, nil
}
	`, baseURL)
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
