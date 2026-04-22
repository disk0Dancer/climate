// Package mock provides a local HTTP mock server that serves synthetic
// responses for every endpoint defined in an OpenAPI 3.x specification.
//
// # Design
//
// ## Problem
//
// During development and testing, developers often need to work against APIs
// that are unavailable — because the real service is slow, requires special
// credentials, produces side-effects, or simply doesn't exist yet.  Standing
// up a full service just to test a CLI or a frontend is expensive.
//
// ## Approach
//
// mock.Server reads an OpenAPI spec and registers one HTTP handler per path.
// When a request arrives the server:
//
//  1. Matches the request path against each registered pattern, resolving
//     path parameters (e.g. /pets/{petId}).
//  2. Looks up the operation's first successful response (2xx) and its schema.
//  3. Generates a synthetic JSON value that conforms to the schema — objects
//     get all declared properties with placeholder values, arrays get one
//     example element, scalars get type-appropriate zero values.
//  4. Returns the value with the appropriate HTTP status code and
//     Content-Type: application/json header.
//
// An optional artificial latency can be configured to simulate realistic
// network conditions.
//
// ## Supported response types
//
//   - object  → {"field": <generated value>, ...}
//   - array   → [<generated element>]
//   - string  → "example"
//   - integer → 1
//   - number  → 1.0
//   - boolean → true
//   - (unknown/empty) → {}
//
// ## Not in scope
//
// Request body validation, stateful CRUD simulation, and auth enforcement are
// intentionally out of scope for the mock server.  Use a dedicated contract-
// testing tool (e.g. Prism, WireMock) when those features are needed.
package mock

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/disk0Dancer/climate/internal/spec"
)

// Server is a local HTTP mock server driven by an OpenAPI specification.
type Server struct {
	openAPI  *spec.OpenAPI
	addr     string
	latency  time.Duration
	mux      *http.ServeMux
	patterns []routePattern
}

// routePattern holds a compiled path pattern and the operations it handles.
type routePattern struct {
	raw     string         // original OpenAPI path, e.g. "/pets/{petId}"
	re      *regexp.Regexp // compiled pattern for matching request paths
	handler http.HandlerFunc
}

// NewServer creates a mock server for the given spec.  addr is a TCP address
// such as ":8080".  latency adds an artificial delay to every response.
func NewServer(openAPI *spec.OpenAPI, addr string, latency time.Duration) *Server {
	s := &Server{
		openAPI: openAPI,
		addr:    addr,
		latency: latency,
		mux:     http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// Handler returns the underlying http.Handler so the server can be embedded
// in tests via httptest.NewServer.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.latency > 0 {
			time.Sleep(s.latency)
		}
		// Try parametric patterns first (longest match first for specificity).
		for _, pat := range s.patterns {
			if pat.re.MatchString(r.URL.Path) {
				pat.handler(w, r)
				return
			}
		}
		http.NotFound(w, r)
	})
}

// ListenAndServe starts the HTTP server and blocks until it returns an error.
func (s *Server) ListenAndServe() error {
	srv := &http.Server{
		Addr:              s.addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return srv.ListenAndServe()
}

// Addr returns the configured listen address.
func (s *Server) Addr() string {
	return s.addr
}

// registerRoutes builds one handler per OpenAPI path.
func (s *Server) registerRoutes() {
	// Sort paths so that more specific (longer) paths are matched first.
	paths := make([]string, 0, len(s.openAPI.Paths))
	for p := range s.openAPI.Paths {
		paths = append(paths, p)
	}
	sort.Slice(paths, func(i, j int) bool {
		return len(paths[i]) > len(paths[j])
	})

	for _, path := range paths {
		item := s.openAPI.Paths[path]
		re := pathToRegexp(path)
		handler := s.makeHandler(path, item)
		s.patterns = append(s.patterns, routePattern{raw: path, re: re, handler: handler})
	}
}

// pathToRegexp converts an OpenAPI path template like "/pets/{petId}" to a
// regexp that matches actual request paths.
func pathToRegexp(path string) *regexp.Regexp {
	escaped := regexp.QuoteMeta(path)
	// Replace escaped \{name\} placeholders with a catch-all segment.
	re := regexp.MustCompile(`\\\{[^}]+\\\}`)
	pattern := re.ReplaceAllString(escaped, `[^/]+`)
	return regexp.MustCompile(`^` + pattern + `$`)
}

// makeHandler returns an http.HandlerFunc that serves mock responses for all
// HTTP methods defined on the given PathItem.
func (s *Server) makeHandler(path string, item spec.PathItem) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ops := item.Operations()
		op, ok := ops[r.Method]
		if !ok {
			// Method not defined — return 405.
			w.Header().Set("Allow", allowedMethods(ops))
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		statusCode, body := s.generateResponse(op)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		if encErr := json.NewEncoder(w).Encode(body); encErr != nil {
			// The header is already written; log to stderr as best effort.
			fmt.Fprintf(w, `{"error":"response encoding failed"}`)
		}
	}
}

// generateResponse picks the first 2xx response defined on the operation and
// produces a synthetic value matching its schema.
func (s *Server) generateResponse(op *spec.Operation) (int, interface{}) {
	if op == nil {
		return http.StatusOK, map[string]interface{}{}
	}

	// Collect and sort response codes so we pick the lowest 2xx deterministically.
	codes := make([]string, 0, len(op.Responses))
	for c := range op.Responses {
		codes = append(codes, c)
	}
	sort.Strings(codes)

	for _, code := range codes {
		statusCode := parseStatusCode(code)
		if statusCode < 200 || statusCode >= 300 {
			continue
		}
		resp := op.Responses[code]
		schema := responseSchema(resp)
		return statusCode, generateValue(schema, s.openAPI, 0)
	}

	// No 2xx response defined — return 200 with an empty object.
	return http.StatusOK, map[string]interface{}{}
}

// responseSchema extracts the first JSON schema from a response's content map.
func responseSchema(resp spec.Response) *spec.Schema {
	for _, mt := range resp.Content {
		if mt.Schema != nil {
			return mt.Schema
		}
	}
	return nil
}

// generateValue produces a Go value that conforms to the given schema.
// depth prevents infinite recursion for self-referential schemas.
func generateValue(s *spec.Schema, openAPI *spec.OpenAPI, depth int) interface{} {
	const maxDepth = 4
	if depth > maxDepth {
		return nil
	}
	if s == nil {
		return map[string]interface{}{}
	}

	// Resolve $ref.
	if s.Ref != "" {
		resolved := resolveRef(s.Ref, openAPI)
		return generateValue(resolved, openAPI, depth+1)
	}

	switch s.Type {
	case "object":
		obj := map[string]interface{}{}
		for name, prop := range s.Properties {
			obj[name] = generateValue(prop, openAPI, depth+1)
		}
		return obj
	case "array":
		elem := generateValue(s.Items, openAPI, depth+1)
		return []interface{}{elem}
	case "string":
		if len(s.Enum) > 0 {
			if str, ok := s.Enum[0].(string); ok {
				return str
			}
		}
		return "example"
	case "integer":
		return 1
	case "number":
		return 1.0
	case "boolean":
		return true
	default:
		return map[string]interface{}{}
	}
}

// resolveRef resolves a JSON reference like "#/components/schemas/Pet" to a
// Schema from the given OpenAPI document.
func resolveRef(ref string, openAPI *spec.OpenAPI) *spec.Schema {
	const schemaPrefix = "#/components/schemas/"
	if strings.HasPrefix(ref, schemaPrefix) {
		name := strings.TrimPrefix(ref, schemaPrefix)
		if s, ok := openAPI.Components.Schemas[name]; ok {
			return s
		}
	}
	return nil
}

// allowedMethods returns a comma-separated string of HTTP methods that have
// operations defined.
func allowedMethods(ops map[string]*spec.Operation) string {
	methods := make([]string, 0, len(ops))
	for m := range ops {
		methods = append(methods, m)
	}
	sort.Strings(methods)
	return strings.Join(methods, ", ")
}

// parseStatusCode converts an OpenAPI response code string (e.g. "200",
// "default") to an integer.  "default" maps to 200.
func parseStatusCode(code string) int {
	if code == "default" {
		return http.StatusOK
	}
	n, err := strconv.Atoi(code)
	if err != nil {
		return http.StatusOK
	}
	return n
}

// Summary returns a human-readable table of all registered routes, one per
// line, formatted as "METHOD /path".
func (s *Server) Summary() string {
	var sb strings.Builder
	paths := make([]string, 0, len(s.openAPI.Paths))
	for p := range s.openAPI.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, path := range paths {
		item := s.openAPI.Paths[path]
		for method := range item.Operations() {
			sb.WriteString(fmt.Sprintf("  %-7s %s\n", method, path))
		}
	}
	return sb.String()
}

// GenerateEventPayload builds a synthetic JSON payload for a specific OpenAPI
// operation, intended for webhook/event simulation scenarios.
//
// The function prefers the operation requestBody schema (`application/json`
// first). If no requestBody schema is available, it falls back to the first
// successful 2xx response schema.
func GenerateEventPayload(openAPI *spec.OpenAPI, path string, method string) (interface{}, error) {
	if openAPI == nil {
		return nil, errors.New("openapi spec is nil")
	}
	item, ok := openAPI.Paths[path]
	if !ok {
		return nil, fmt.Errorf("path %q not found in spec", path)
	}

	ops := item.Operations()
	op, ok := ops[strings.ToUpper(strings.TrimSpace(method))]
	if !ok || op == nil {
		return nil, fmt.Errorf("method %q is not defined for path %q", method, path)
	}

	if schema := requestBodySchema(op.RequestBody); schema != nil {
		return generateValue(schema, openAPI, 0), nil
	}

	_, body := (&Server{openAPI: openAPI}).generateResponse(op)
	return body, nil
}

// EmitEvent sends a JSON payload as an HTTP event to targetURL using method.
// It returns the response status code from the target endpoint.
func EmitEvent(targetURL string, method string, payload interface{}) (int, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(strings.TrimSpace(method)), targetURL, bytes.NewReader(b))
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("send event: %w", err)
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func requestBodySchema(rb *spec.RequestBody) *spec.Schema {
	if rb == nil || len(rb.Content) == 0 {
		return nil
	}
	if mt, ok := rb.Content["application/json"]; ok && mt.Schema != nil {
		return mt.Schema
	}

	contentTypes := make([]string, 0, len(rb.Content))
	for ct := range rb.Content {
		contentTypes = append(contentTypes, ct)
	}
	sort.Strings(contentTypes)
	for _, ct := range contentTypes {
		if mt := rb.Content[ct]; mt.Schema != nil {
			return mt.Schema
		}
	}
	return nil
}
