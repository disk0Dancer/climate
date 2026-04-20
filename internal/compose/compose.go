// Package compose merges multiple OpenAPI 3.x specifications into a single
// composite specification.  Each source spec is assigned a path prefix so
// that all its routes are namespaced under that prefix in the merged document.
// The resulting spec can be fed directly to the generator to produce a single
// CLI that acts as a facade over several microservices.
//
// # Design
//
// ## Problem
//
// Large systems are commonly decomposed into microservices, each with its own
// OpenAPI spec.  Developers and agents must juggle many separate CLIs or deal
// with raw HTTP calls to reach different services.  A single "gateway" CLI
// that knows about all services is far more ergonomic.
//
// ## Approach
//
// compose.Merge takes a list of (source, prefix) pairs.  For every source
// spec it:
//
//  1. Loads and validates the individual spec.
//  2. Rewrites every path key so that the given prefix is prepended.
//     (e.g. "/pets" → "/v1/pets" when prefix is "/v1")
//  3. Namespaces every component schema, parameter and security scheme with
//     a sanitised version of the prefix to avoid name collisions between
//     services.
//  4. Rewrites intra-document $ref strings inside the spec to point at the
//     new, namespaced component names.
//  5. Merges all paths, components and tags into a single OpenAPI document.
//
// Security schemes are merged from all input specs.  The caller may choose a
// single unified auth strategy (e.g. a shared Bearer token gateway) or leave
// each service's scheme in place.
//
// ## Output
//
// Merge returns a *spec.OpenAPI value that is ready to be passed to
// generator.Generate.  A convenience MergeToBytes helper also produces the
// canonical JSON serialisation, which is required by generator.Generate.
package compose

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/disk0Dancer/climate/internal/spec"
)

// SpecInput describes one microservice spec that should be included in the
// composed output.
type SpecInput struct {
	// Source is a file path or HTTP(S) URL pointing at an OpenAPI 3.x document.
	Source string
	// Prefix is a non-empty path prefix (e.g. "/orders/v1") prepended to every
	// path defined in Source.  It must start with "/".
	Prefix string
}

// Options controls how the merge is performed.
type Options struct {
	// Title is the info.title of the merged spec.  Defaults to "Composed API".
	Title string
	// Version is the info.version of the merged spec.  Defaults to "1.0.0".
	Version string
	// Description is the info.description of the merged spec.
	Description string
	// Servers are the server entries written to the merged spec.  When empty
	// the servers of the first input spec are used.
	Servers []spec.Server
}

// Merge loads each input spec, applies the configured prefix to all of its
// paths, namespaces its component names, and merges everything into one
// OpenAPI document.
func Merge(inputs []SpecInput, opts Options) (*spec.OpenAPI, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("compose: at least one spec input is required")
	}

	if opts.Title == "" {
		opts.Title = "Composed API"
	}
	if opts.Version == "" {
		opts.Version = "1.0.0"
	}

	out := &spec.OpenAPI{
		OpenAPI: "3.0.0",
		Info: spec.Info{
			Title:       opts.Title,
			Version:     opts.Version,
			Description: opts.Description,
		},
		Paths: make(map[string]spec.PathItem),
		Components: spec.Components{
			SecuritySchemes: make(map[string]spec.SecurityScheme),
			Schemas:         make(map[string]*spec.Schema),
			Parameters:      make(map[string]spec.Parameter),
		},
	}

	for i, inp := range inputs {
		if inp.Source == "" {
			return nil, fmt.Errorf("compose: input %d has empty source", i)
		}
		if err := validatePrefix(inp.Prefix); err != nil {
			return nil, fmt.Errorf("compose: input %d (%s): %w", i, inp.Source, err)
		}

		s, err := spec.Load(inp.Source)
		if err != nil {
			return nil, fmt.Errorf("compose: loading %s: %w", inp.Source, err)
		}

		ns := prefixToNamespace(inp.Prefix)
		mergeSpec(out, s, inp.Prefix, ns)

		// Use the first spec's servers when the caller did not supply any.
		if i == 0 && len(opts.Servers) == 0 && len(s.Servers) > 0 {
			out.Servers = s.Servers
		}
	}

	if len(opts.Servers) > 0 {
		out.Servers = opts.Servers
	}

	return out, nil
}

// MergeToBytes calls Merge and then JSON-encodes the result.  The returned
// bytes can be passed as the rawSpec argument to generator.Generate.
func MergeToBytes(inputs []SpecInput, opts Options) (*spec.OpenAPI, []byte, error) {
	merged, err := Merge(inputs, opts)
	if err != nil {
		return nil, nil, err
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		return nil, nil, fmt.Errorf("compose: serialising merged spec: %w", err)
	}
	return merged, raw, nil
}

// validatePrefix checks that prefix is non-empty and starts with "/".
func validatePrefix(prefix string) error {
	if prefix == "" {
		return fmt.Errorf("prefix must not be empty")
	}
	if !strings.HasPrefix(prefix, "/") {
		return fmt.Errorf("prefix %q must start with '/'", prefix)
	}
	return nil
}

// prefixToNamespace converts a path prefix like "/orders/v1" to a safe
// component-name namespace like "orders-v1".
func prefixToNamespace(prefix string) string {
	ns := strings.TrimPrefix(prefix, "/")
	ns = strings.ReplaceAll(ns, "/", "-")
	ns = strings.ReplaceAll(ns, "_", "-")
	// Keep only alphanumeric and hyphens.
	var b strings.Builder
	for _, r := range ns {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "svc"
	}
	return result
}

// mergeSpec copies the paths and components of src into dst, applying the
// given prefix to all paths and namespacing component names with ns.
func mergeSpec(dst, src *spec.OpenAPI, prefix, ns string) {
	// Namespace component schemas.
	schemaMap := make(map[string]string) // oldName → newName
	for name, schema := range src.Components.Schemas {
		newName := ns + "-" + name
		schemaMap[name] = newName
		dst.Components.Schemas[newName] = rewriteSchemaRefs(schema, schemaMap, ns)
	}

	// Namespace component parameters.
	paramMap := make(map[string]string)
	for name, param := range src.Components.Parameters {
		newName := ns + "-" + name
		paramMap[name] = newName
		p := param
		p.Schema = rewriteSchemaRefs(p.Schema, schemaMap, ns)
		dst.Components.Parameters[newName] = p
	}

	// Merge security schemes (last writer wins for same name).
	for name, scheme := range src.Components.SecuritySchemes {
		dst.Components.SecuritySchemes[name] = scheme
	}

	// Merge tags (de-duplicate by name).
	existingTags := make(map[string]bool)
	for _, t := range dst.Tags {
		existingTags[t.Name] = true
	}
	for _, t := range src.Tags {
		if !existingTags[t.Name] {
			dst.Tags = append(dst.Tags, t)
			existingTags[t.Name] = true
		}
	}

	// Prefix and copy paths.
	normalised := strings.TrimRight(prefix, "/")
	for path, item := range src.Paths {
		newPath := normalised + path
		dst.Paths[newPath] = rewritePathItem(item, schemaMap, paramMap, ns)
	}
}

// rewritePathItem rewrites all $ref strings inside a PathItem's operations.
func rewritePathItem(item spec.PathItem, schemaMap, paramMap map[string]string, ns string) spec.PathItem {
	item.Get = rewriteOp(item.Get, schemaMap, paramMap, ns)
	item.Post = rewriteOp(item.Post, schemaMap, paramMap, ns)
	item.Put = rewriteOp(item.Put, schemaMap, paramMap, ns)
	item.Patch = rewriteOp(item.Patch, schemaMap, paramMap, ns)
	item.Delete = rewriteOp(item.Delete, schemaMap, paramMap, ns)
	item.Head = rewriteOp(item.Head, schemaMap, paramMap, ns)
	item.Options = rewriteOp(item.Options, schemaMap, paramMap, ns)
	return item
}

// rewriteOp rewrites $ref strings inside an operation, or returns nil if op
// is nil.
func rewriteOp(op *spec.Operation, schemaMap, paramMap map[string]string, ns string) *spec.Operation {
	if op == nil {
		return nil
	}
	rewritten := rewriteOperation(*op, schemaMap, paramMap, ns)
	return &rewritten
}

// rewriteOperation rewrites $ref strings inside an operation.
func rewriteOperation(op spec.Operation, schemaMap, paramMap map[string]string, ns string) spec.Operation {
	for i, p := range op.Parameters {
		if p.Ref != "" {
			op.Parameters[i].Ref = rewriteRef(p.Ref, "#/components/parameters/", paramMap, ns)
		}
		op.Parameters[i].Schema = rewriteSchemaRefs(p.Schema, schemaMap, ns)
	}
	if op.RequestBody != nil {
		rb := *op.RequestBody
		newContent := make(map[string]spec.MediaType, len(rb.Content))
		for mediaType, mt := range rb.Content {
			mt.Schema = rewriteSchemaRefs(mt.Schema, schemaMap, ns)
			newContent[mediaType] = mt
		}
		rb.Content = newContent
		op.RequestBody = &rb
	}
	newResponses := make(map[string]spec.Response, len(op.Responses))
	for code, resp := range op.Responses {
		newContent := make(map[string]spec.MediaType, len(resp.Content))
		for mediaType, mt := range resp.Content {
			mt.Schema = rewriteSchemaRefs(mt.Schema, schemaMap, ns)
			newContent[mediaType] = mt
		}
		resp.Content = newContent
		newResponses[code] = resp
	}
	op.Responses = newResponses
	return op
}

// rewriteSchemaRefs rewrites $ref fields inside a Schema tree.
func rewriteSchemaRefs(s *spec.Schema, schemaMap map[string]string, ns string) *spec.Schema {
	if s == nil {
		return nil
	}
	out := *s
	if out.Ref != "" {
		out.Ref = rewriteRef(out.Ref, "#/components/schemas/", schemaMap, ns)
	}
	if out.Items != nil {
		out.Items = rewriteSchemaRefs(out.Items, schemaMap, ns)
	}
	if len(out.Properties) > 0 {
		newProps := make(map[string]*spec.Schema, len(out.Properties))
		for k, v := range out.Properties {
			newProps[k] = rewriteSchemaRefs(v, schemaMap, ns)
		}
		out.Properties = newProps
	}
	return &out
}

// rewriteRef rewrites a $ref string that uses the given prefix.
// nameMap maps the old bare name to the new namespaced name.
func rewriteRef(ref, prefix string, nameMap map[string]string, ns string) string {
	if !strings.HasPrefix(ref, prefix) {
		return ref
	}
	oldName := strings.TrimPrefix(ref, prefix)
	if newName, ok := nameMap[oldName]; ok {
		return prefix + newName
	}
	// Fall back: namespace the name even if it was not found in the map
	// (e.g. inline schemas referenced before being declared).
	return prefix + ns + "-" + oldName
}
