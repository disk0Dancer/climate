package compose_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/disk0Dancer/climate/internal/compose"
	"github.com/disk0Dancer/climate/internal/spec"
)

// writeTempSpec writes a minimal OpenAPI JSON spec to a temp file and returns
// its path.
func writeTempSpec(t *testing.T, title, version string, paths map[string]spec.PathItem) string {
	t.Helper()
	s := spec.OpenAPI{
		OpenAPI: "3.0.0",
		Info:    spec.Info{Title: title, Version: version},
		Paths:   paths,
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal spec: %v", err)
	}
	f := filepath.Join(t.TempDir(), "spec.json")
	if err := os.WriteFile(f, data, 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}
	return f
}

func TestMerge_PathPrefixing(t *testing.T) {
	src1 := writeTempSpec(t, "Orders", "1.0.0", map[string]spec.PathItem{
		"/orders": {Get: &spec.Operation{OperationID: "list_orders"}},
	})
	src2 := writeTempSpec(t, "Users", "1.0.0", map[string]spec.PathItem{
		"/users": {Get: &spec.Operation{OperationID: "list_users"}},
	})

	merged, err := compose.Merge([]compose.SpecInput{
		{Source: src1, Prefix: "/api/v1"},
		{Source: src2, Prefix: "/api/v2"},
	}, compose.Options{Title: "Gateway", Version: "1.0.0"})

	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}

	if _, ok := merged.Paths["/api/v1/orders"]; !ok {
		t.Error("expected path /api/v1/orders in merged spec")
	}
	if _, ok := merged.Paths["/api/v2/users"]; !ok {
		t.Error("expected path /api/v2/users in merged spec")
	}
	if _, ok := merged.Paths["/orders"]; ok {
		t.Error("original unprefixed path /orders should not appear in merged spec")
	}
}

func TestMerge_MetaDefaults(t *testing.T) {
	src := writeTempSpec(t, "Svc", "1.0.0", map[string]spec.PathItem{
		"/ping": {Get: &spec.Operation{OperationID: "ping"}},
	})

	merged, err := compose.Merge([]compose.SpecInput{{Source: src, Prefix: "/svc"}},
		compose.Options{})
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}
	if merged.Info.Title != "Composed API" {
		t.Errorf("default title = %q, want %q", merged.Info.Title, "Composed API")
	}
	if merged.Info.Version != "1.0.0" {
		t.Errorf("default version = %q, want %q", merged.Info.Version, "1.0.0")
	}
}

func TestMerge_ComponentNamespacing(t *testing.T) {
	s := spec.OpenAPI{
		OpenAPI: "3.0.0",
		Info:    spec.Info{Title: "A", Version: "1.0.0"},
		Paths: map[string]spec.PathItem{
			"/items": {Get: &spec.Operation{OperationID: "listItems"}},
		},
		Components: spec.Components{
			Schemas: map[string]*spec.Schema{
				"Item": {Type: "object"},
			},
		},
	}
	data, _ := json.Marshal(s)
	f := filepath.Join(t.TempDir(), "a.json")
	_ = os.WriteFile(f, data, 0o644)

	merged, err := compose.Merge([]compose.SpecInput{{Source: f, Prefix: "/svc-a"}},
		compose.Options{Title: "T", Version: "1.0.0"})
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}
	if _, ok := merged.Components.Schemas["svc-a-Item"]; !ok {
		t.Error("expected namespaced schema svc-a-Item")
	}
	if _, ok := merged.Components.Schemas["Item"]; ok {
		t.Error("un-namespaced schema Item should not appear in merged spec")
	}
}

func TestMerge_NoInputs(t *testing.T) {
	_, err := compose.Merge(nil, compose.Options{})
	if err == nil {
		t.Error("expected error for empty inputs, got nil")
	}
}

func TestMerge_BadPrefix(t *testing.T) {
	src := writeTempSpec(t, "X", "1.0.0", map[string]spec.PathItem{
		"/x": {Get: &spec.Operation{OperationID: "x"}},
	})
	_, err := compose.Merge([]compose.SpecInput{{Source: src, Prefix: "noslash"}},
		compose.Options{})
	if err == nil {
		t.Error("expected error for prefix without leading slash")
	}
}

func TestMergeToBytes(t *testing.T) {
	src := writeTempSpec(t, "B", "1.0.0", map[string]spec.PathItem{
		"/b": {Get: &spec.Operation{OperationID: "b"}},
	})
	merged, raw, err := compose.MergeToBytes([]compose.SpecInput{{Source: src, Prefix: "/b"}},
		compose.Options{Title: "B-Facade", Version: "1.0.0"})
	if err != nil {
		t.Fatalf("MergeToBytes() error = %v", err)
	}
	if merged == nil {
		t.Fatal("expected non-nil merged spec")
	}
	if len(raw) == 0 {
		t.Error("expected non-empty raw bytes")
	}
	var check spec.OpenAPI
	if err := json.Unmarshal(raw, &check); err != nil {
		t.Errorf("raw bytes are not valid JSON: %v", err)
	}
}

func TestMerge_TagDeduplication(t *testing.T) {
	makeSpec := func(title string, tag spec.Tag) string {
		s := spec.OpenAPI{
			OpenAPI: "3.0.0",
			Info:    spec.Info{Title: title, Version: "1.0.0"},
			Paths:   map[string]spec.PathItem{"/x": {Get: &spec.Operation{OperationID: title}}},
			Tags:    []spec.Tag{tag},
		}
		data, _ := json.Marshal(s)
		f := filepath.Join(t.TempDir(), "s.json")
		_ = os.WriteFile(f, data, 0o644)
		return f
	}
	src1 := makeSpec("A", spec.Tag{Name: "shared", Description: "first"})
	src2 := makeSpec("B", spec.Tag{Name: "shared", Description: "second"})

	merged, err := compose.Merge([]compose.SpecInput{
		{Source: src1, Prefix: "/a"},
		{Source: src2, Prefix: "/b"},
	}, compose.Options{})
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}
	count := 0
	for _, tg := range merged.Tags {
		if tg.Name == "shared" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected tag 'shared' to appear exactly once, got %d", count)
	}
}
