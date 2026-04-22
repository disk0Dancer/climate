package spec_test

import (
	"testing"

	"github.com/disk0Dancer/climate/internal/spec"
)

func TestNormalizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Petstore", "petstore"},
		{"My API", "my-api"},
		{"My_API", "my-api"},
		{"My API v2.0", "my-api-v2-0"},
		{"  leading spaces  ", "leading-spaces"},
		{"", "api"},
		{"!!!special@@@", "special"},
		{"GitHub API", "github-api"},
	}
	for _, tt := range tests {
		got := spec.NormalizeName(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		spec    *spec.OpenAPI
		wantErr bool
	}{
		{
			name: "valid spec",
			spec: &spec.OpenAPI{
				OpenAPI: "3.0.0",
				Info:    spec.Info{Title: "Test", Version: "1.0.0"},
				Paths:   map[string]spec.PathItem{"/ping": {}},
			},
			wantErr: false,
		},
		{
			name: "missing openapi field",
			spec: &spec.OpenAPI{
				Info:  spec.Info{Title: "Test", Version: "1.0.0"},
				Paths: map[string]spec.PathItem{"/ping": {}},
			},
			wantErr: true,
		},
		{
			name: "unsupported version",
			spec: &spec.OpenAPI{
				OpenAPI: "2.0",
				Info:    spec.Info{Title: "Test", Version: "1.0.0"},
				Paths:   map[string]spec.PathItem{"/ping": {}},
			},
			wantErr: true,
		},
		{
			name: "missing title",
			spec: &spec.OpenAPI{
				OpenAPI: "3.0.0",
				Info:    spec.Info{Version: "1.0.0"},
				Paths:   map[string]spec.PathItem{"/ping": {}},
			},
			wantErr: true,
		},
		{
			name: "missing version",
			spec: &spec.OpenAPI{
				OpenAPI: "3.0.0",
				Info:    spec.Info{Title: "Test"},
				Paths:   map[string]spec.PathItem{"/ping": {}},
			},
			wantErr: true,
		},
		{
			name: "no paths",
			spec: &spec.OpenAPI{
				OpenAPI: "3.0.0",
				Info:    spec.Info{Title: "Test", Version: "1.0.0"},
				Paths:   map[string]spec.PathItem{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := spec.Validate(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParse_JSON(t *testing.T) {
	jsonSpec := `{
		"openapi": "3.0.0",
		"info": {"title": "Test API", "version": "1.0.0"},
		"paths": {"/ping": {}}
	}`
	s, err := spec.Parse("test.json", []byte(jsonSpec))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if s.Info.Title != "Test API" {
		t.Errorf("Title = %q, want %q", s.Info.Title, "Test API")
	}
}

func TestParse_YAML(t *testing.T) {
	yamlSpec := `
openapi: "3.0.0"
info:
  title: "YAML API"
  version: "2.0.0"
paths:
  /ping: {}
`
	s, err := spec.Parse("test.yaml", []byte(yamlSpec))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if s.Info.Title != "YAML API" {
		t.Errorf("Title = %q, want %q", s.Info.Title, "YAML API")
	}
	if s.Info.Version != "2.0.0" {
		t.Errorf("Version = %q, want %q", s.Info.Version, "2.0.0")
	}
}

func TestParse_ServerVariables(t *testing.T) {
	yamlSpec := `
openapi: "3.0.0"
info:
  title: "Server Vars API"
  version: "1.0.0"
servers:
  - url: "https://{region}.api.example.com/{basePath}"
    variables:
      region:
        default: "eu"
        enum: ["eu", "us"]
      basePath:
        default: "v1"
paths:
  /ping: {}
`
	s, err := spec.Parse("server-vars.yaml", []byte(yamlSpec))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(s.Servers) != 1 {
		t.Fatalf("servers count = %d, want 1", len(s.Servers))
	}
	srv := s.Servers[0]
	if srv.URL != "https://{region}.api.example.com/{basePath}" {
		t.Fatalf("server url = %q", srv.URL)
	}
	if got := srv.Variables["region"].Default; got != "eu" {
		t.Errorf("region default = %q, want %q", got, "eu")
	}
	if got := len(srv.Variables["region"].Enum); got != 2 {
		t.Errorf("region enum len = %d, want 2", got)
	}
	if got := srv.Variables["basePath"].Default; got != "v1" {
		t.Errorf("basePath default = %q, want %q", got, "v1")
	}
}

func TestIsURL(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"https://example.com/api.yaml", true},
		{"http://localhost:8080/spec.json", true},
		{"./local/file.yaml", false},
		{"/absolute/path.json", false},
		{"relative.yaml", false},
	}
	for _, tt := range tests {
		got := spec.IsURL(tt.input)
		if got != tt.want {
			t.Errorf("IsURL(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestHashBytes(t *testing.T) {
	h1 := spec.HashBytes([]byte("hello"))
	h2 := spec.HashBytes([]byte("hello"))
	h3 := spec.HashBytes([]byte("world"))

	if h1 != h2 {
		t.Error("HashBytes should be deterministic")
	}
	if h1 == h3 {
		t.Error("HashBytes should differ for different inputs")
	}
	if len(h1) != 64 {
		t.Errorf("HashBytes length = %d, want 64", len(h1))
	}
}

func TestPathItemOperations(t *testing.T) {
	getOp := &spec.Operation{OperationID: "getItems"}
	postOp := &spec.Operation{OperationID: "createItem"}

	item := spec.PathItem{
		Get:  getOp,
		Post: postOp,
	}

	ops := item.Operations()
	if len(ops) != 2 {
		t.Errorf("Operations() count = %d, want 2", len(ops))
	}
	if ops["GET"] != getOp {
		t.Error("GET operation not found")
	}
	if ops["POST"] != postOp {
		t.Error("POST operation not found")
	}
}
