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
			name: "no paths or webhooks",
			spec: &spec.OpenAPI{
				OpenAPI: "3.0.0",
				Info:    spec.Info{Title: "Test", Version: "1.0.0"},
				Paths:   map[string]spec.PathItem{},
			},
			wantErr: true,
		},
		{
			name: "webhooks without paths",
			spec: &spec.OpenAPI{
				OpenAPI: "3.1.0",
				Info:    spec.Info{Title: "Webhook API", Version: "1.0.0"},
				Webhooks: map[string]spec.PathItem{
					"payment.succeeded": {
						Post: &spec.Operation{Summary: "Payment succeeded"},
					},
				},
			},
			wantErr: false,
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

func TestParse_WebhooksAndCallbacks(t *testing.T) {
	yamlSpec := `
openapi: "3.1.0"
x-climate-signature-mode: hmac
info:
  title: "Webhook API"
  version: "1.0.0"
paths:
  /subscriptions:
    post:
      operationId: subscriptions_create
      x-climate-signature-header: X-Custom-Signature
      callbacks:
        invoicePaid:
          "{$request.body#/callback_url}":
            post:
              summary: "Invoice paid callback"
              x-climate-event-name: invoice-paid
              x-climate-event-path: /webhooks/invoice-paid
              requestBody:
                content:
                  application/json:
                    schema:
                      type: object
                      properties:
                        event:
                          type: string
webhooks:
  payment.succeeded:
    post:
      summary: "Payment succeeded webhook"
      x-climate-event-name: payment-succeeded
      x-climate-signature-algorithm: sha512
      x-climate-signature-include-timestamp: true
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                id:
                  type: string
`
	s, err := spec.Parse("test.yaml", []byte(yamlSpec))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(s.Webhooks) != 1 {
		t.Fatalf("Webhooks count = %d, want 1", len(s.Webhooks))
	}
	if s.XClimateSignatureMode != "hmac" {
		t.Fatalf("root signature mode = %q, want hmac", s.XClimateSignatureMode)
	}
	webhookOp := s.Webhooks["payment.succeeded"].Post
	if webhookOp == nil || webhookOp.Summary != "Payment succeeded webhook" {
		t.Fatal("expected parsed webhook operation")
	}
	if webhookOp.XClimateSignatureAlgorithm != "sha512" {
		t.Fatalf("webhook algorithm = %q, want sha512", webhookOp.XClimateSignatureAlgorithm)
	}
	subscriptionOp := s.Paths["/subscriptions"].Post
	if subscriptionOp == nil {
		t.Fatal("expected parsed subscriptions operation")
	}
	if subscriptionOp.XClimateSignatureHeader != "X-Custom-Signature" {
		t.Fatalf("callback parent signature header = %q", subscriptionOp.XClimateSignatureHeader)
	}
	callback, ok := subscriptionOp.Callbacks["invoicePaid"]
	if !ok {
		t.Fatal("expected callback to be parsed")
	}
	item, ok := callback["{$request.body#/callback_url}"]
	if !ok || item.Post == nil {
		t.Fatal("expected callback expression path item")
	}
	if item.Post.XClimateEventName != "invoice-paid" {
		t.Fatalf("callback event name = %q, want invoice-paid", item.Post.XClimateEventName)
	}
	if item.Post.XClimateEventPath != "/webhooks/invoice-paid" {
		t.Fatalf("callback event path = %q", item.Post.XClimateEventPath)
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

func TestServerVariableEnvName(t *testing.T) {
	tests := []struct {
		name     string
		variable string
		want     string
	}{
		{name: "PETSTORE", variable: "region", want: "PETSTORE_SERVER_VAR_REGION"},
		{name: "PETSTORE", variable: "basePath", want: "PETSTORE_SERVER_VAR_BASE_PATH"},
		{name: "MY_API", variable: "region-id", want: "MY_API_SERVER_VAR_REGION_ID"},
	}
	for _, tt := range tests {
		got := spec.ServerVariableEnvName(tt.name, tt.variable)
		if got != tt.want {
			t.Errorf("ServerVariableEnvName(%q,%q) = %q, want %q", tt.name, tt.variable, got, tt.want)
		}
	}
}
