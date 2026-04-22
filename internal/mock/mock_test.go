package mock_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/disk0Dancer/climate/internal/mock"
	"github.com/disk0Dancer/climate/internal/spec"
)

func petStoreSpec() *spec.OpenAPI {
	return &spec.OpenAPI{
		OpenAPI: "3.0.0",
		Info:    spec.Info{Title: "Petstore", Version: "1.0.0"},
		Paths: map[string]spec.PathItem{
			"/pets": {
				Get: &spec.Operation{
					OperationID: "listPets",
					Responses: map[string]spec.Response{
						"200": {
							Content: map[string]spec.MediaType{
								"application/json": {
									Schema: &spec.Schema{
										Type:  "array",
										Items: &spec.Schema{Type: "object"},
									},
								},
							},
						},
					},
				},
				Post: &spec.Operation{
					OperationID: "createPet",
					Responses: map[string]spec.Response{
						"201": {
							Content: map[string]spec.MediaType{
								"application/json": {
									Schema: &spec.Schema{Type: "object"},
								},
							},
						},
					},
				},
			},
			"/pets/{petId}": {
				Get: &spec.Operation{
					OperationID: "getPet",
					Responses: map[string]spec.Response{
						"200": {
							Content: map[string]spec.MediaType{
								"application/json": {
									Schema: &spec.Schema{
										Type: "object",
										Properties: map[string]*spec.Schema{
											"id":   {Type: "integer"},
											"name": {Type: "string"},
										},
									},
								},
							},
						},
					},
				},
				Delete: &spec.Operation{
					OperationID: "deletePet",
					Responses: map[string]spec.Response{
						"204": {},
					},
				},
			},
		},
	}
}

func newTestServer(t *testing.T, openAPI *spec.OpenAPI) *httptest.Server {
	t.Helper()
	s := mock.NewServer(openAPI, ":0", 0)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts
}

func TestMock_GetList(t *testing.T) {
	ts := newTestServer(t, petStoreSpec())
	resp, err := http.Get(ts.URL + "/pets")
	if err != nil {
		t.Fatalf("GET /pets: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	var body interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Errorf("decode body: %v", err)
	}
	arr, ok := body.([]interface{})
	if !ok {
		t.Errorf("expected JSON array, got %T", body)
	}
	if len(arr) == 0 {
		t.Error("expected at least one element in array response")
	}
}

func TestMock_GetByID(t *testing.T) {
	ts := newTestServer(t, petStoreSpec())
	resp, err := http.Get(ts.URL + "/pets/42")
	if err != nil {
		t.Fatalf("GET /pets/42: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	var obj map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&obj); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := obj["id"]; !ok {
		t.Error("response object missing 'id' field")
	}
	if _, ok := obj["name"]; !ok {
		t.Error("response object missing 'name' field")
	}
}

func TestMock_Post(t *testing.T) {
	ts := newTestServer(t, petStoreSpec())
	resp, err := http.Post(ts.URL+"/pets", "application/json",
		strings.NewReader(`{"name":"Fido"}`))
	if err != nil {
		t.Fatalf("POST /pets: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want 201", resp.StatusCode)
	}
}

func TestMock_Delete(t *testing.T) {
	ts := newTestServer(t, petStoreSpec())
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/pets/1", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /pets/1: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
}

func TestMock_MethodNotAllowed(t *testing.T) {
	ts := newTestServer(t, petStoreSpec())
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/pets", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /pets: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestMock_NotFound(t *testing.T) {
	ts := newTestServer(t, petStoreSpec())
	resp, err := http.Get(ts.URL + "/nonexistent")
	if err != nil {
		t.Fatalf("GET /nonexistent: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestMock_Latency(t *testing.T) {
	s := mock.NewServer(petStoreSpec(), ":0", 50*time.Millisecond)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	start := time.Now()
	resp, err := http.Get(ts.URL + "/pets")
	if err != nil {
		t.Fatalf("GET /pets: %v", err)
	}
	resp.Body.Close()
	elapsed := time.Since(start)
	if elapsed < 50*time.Millisecond {
		t.Errorf("expected at least 50ms latency, got %v", elapsed)
	}
}

func TestMock_Summary(t *testing.T) {
	s := mock.NewServer(petStoreSpec(), ":8080", 0)
	summary := s.Summary()
	if summary == "" {
		t.Error("expected non-empty summary")
	}
	// Should contain both paths and methods
	if !strings.Contains(summary, "/pets") {
		t.Error("summary should mention /pets")
	}
}

func TestMock_ContentTypeJSON(t *testing.T) {
	ts := newTestServer(t, petStoreSpec())
	resp, err := http.Get(ts.URL + "/pets")
	if err != nil {
		t.Fatalf("GET /pets: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json; body: %s", ct, body)
	}
}

func TestMock_SchemaWithRef(t *testing.T) {
	openAPI := &spec.OpenAPI{
		OpenAPI: "3.0.0",
		Info:    spec.Info{Title: "T", Version: "1.0.0"},
		Paths: map[string]spec.PathItem{
			"/items": {
				Get: &spec.Operation{
					Responses: map[string]spec.Response{
						"200": {Content: map[string]spec.MediaType{
							"application/json": {Schema: &spec.Schema{
								Ref: "#/components/schemas/Item",
							}},
						}},
					},
				},
			},
		},
		Components: spec.Components{
			Schemas: map[string]*spec.Schema{
				"Item": {
					Type: "object",
					Properties: map[string]*spec.Schema{
						"id":   {Type: "integer"},
						"name": {Type: "string"},
					},
				},
			},
		},
	}

	ts := newTestServer(t, openAPI)
	resp, err := http.Get(ts.URL + "/items")
	if err != nil {
		t.Fatalf("GET /items: %v", err)
	}
	defer resp.Body.Close()
	var obj map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&obj); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := obj["id"]; !ok {
		t.Error("resolved $ref: response missing 'id'")
	}
}

func TestGenerateEventPayload_RequestBody(t *testing.T) {
	openAPI := &spec.OpenAPI{
		OpenAPI: "3.0.0",
		Info:    spec.Info{Title: "Events", Version: "1.0.0"},
		Paths: map[string]spec.PathItem{
			"/events/order-created": {
				Post: &spec.Operation{
					RequestBody: &spec.RequestBody{
						Content: map[string]spec.MediaType{
							"application/json": {
								Schema: &spec.Schema{
									Type: "object",
									Properties: map[string]*spec.Schema{
										"eventId": {Type: "string"},
										"amount":  {Type: "number"},
									},
								},
							},
						},
					},
					Responses: map[string]spec.Response{
						"202": {},
					},
				},
			},
		},
	}

	payload, err := mock.GenerateEventPayload(openAPI, "/events/order-created", "POST")
	if err != nil {
		t.Fatalf("GenerateEventPayload error: %v", err)
	}
	obj, ok := payload.(map[string]interface{})
	if !ok {
		t.Fatalf("payload type = %T, want object", payload)
	}
	if _, ok := obj["eventId"]; !ok {
		t.Error("missing eventId in generated payload")
	}
	if _, ok := obj["amount"]; !ok {
		t.Error("missing amount in generated payload")
	}
}

func TestGenerateEventPayload_FallbackToResponse(t *testing.T) {
	payload, err := mock.GenerateEventPayload(petStoreSpec(), "/pets", "GET")
	if err != nil {
		t.Fatalf("GenerateEventPayload error: %v", err)
	}
	if _, ok := payload.([]interface{}); !ok {
		t.Fatalf("payload type = %T, want []interface{} fallback from response schema", payload)
	}
}

func TestGenerateEventPayload_InvalidOperation(t *testing.T) {
	_, err := mock.GenerateEventPayload(petStoreSpec(), "/pets", "TRACE")
	if err == nil {
		t.Fatal("expected error for undefined method")
	}
}

func TestEmitEvent(t *testing.T) {
	var (
		gotMethod      string
		gotContentType string
		gotBody        map[string]interface{}
	)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer target.Close()

	status, err := mock.EmitEvent(target.URL, "post", map[string]interface{}{
		"event": "order.created",
		"id":    "evt_1",
	})
	if err != nil {
		t.Fatalf("EmitEvent error: %v", err)
	}
	if status != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", status, http.StatusAccepted)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if !strings.HasPrefix(gotContentType, "application/json") {
		t.Errorf("content-type = %q, want application/json", gotContentType)
	}
	if gotBody["event"] != "order.created" {
		t.Errorf("event = %v, want order.created", gotBody["event"])
	}
}
