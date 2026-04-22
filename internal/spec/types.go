// Package spec provides types and loading logic for OpenAPI 3.0/3.1 specifications.
package spec

// OpenAPI represents the top-level OpenAPI document.
type OpenAPI struct {
	OpenAPI    string                `json:"openapi"    yaml:"openapi"`
	Info       Info                  `json:"info"       yaml:"info"`
	Servers    []Server              `json:"servers"    yaml:"servers"`
	Paths      map[string]PathItem   `json:"paths"      yaml:"paths"`
	Components Components            `json:"components" yaml:"components"`
	Security   []SecurityRequirement `json:"security"   yaml:"security"`
	Tags       []Tag                 `json:"tags"       yaml:"tags"`
}

// Info holds API metadata.
type Info struct {
	Title       string `json:"title"       yaml:"title"`
	Version     string `json:"version"     yaml:"version"`
	Description string `json:"description" yaml:"description"`
}

// Server represents an API server.
type Server struct {
	URL         string                    `json:"url"         yaml:"url"`
	Description string                    `json:"description" yaml:"description"`
	Variables   map[string]ServerVariable `json:"variables"   yaml:"variables"`
}

// ServerVariable represents one templated variable for a server URL.
type ServerVariable struct {
	Enum        []string `json:"enum"        yaml:"enum"`
	Default     string   `json:"default"     yaml:"default"`
	Description string   `json:"description" yaml:"description"`
}

// Tag represents an OpenAPI tag.
type Tag struct {
	Name        string `json:"name"        yaml:"name"`
	Description string `json:"description" yaml:"description"`
}

// PathItem holds all operations for a path.
type PathItem struct {
	Get     *Operation `json:"get"     yaml:"get"`
	Post    *Operation `json:"post"    yaml:"post"`
	Put     *Operation `json:"put"     yaml:"put"`
	Patch   *Operation `json:"patch"   yaml:"patch"`
	Delete  *Operation `json:"delete"  yaml:"delete"`
	Head    *Operation `json:"head"    yaml:"head"`
	Options *Operation `json:"options" yaml:"options"`
}

// Operations returns all non-nil operations with their HTTP method.
func (pi PathItem) Operations() map[string]*Operation {
	ops := map[string]*Operation{}
	if pi.Get != nil {
		ops["GET"] = pi.Get
	}
	if pi.Post != nil {
		ops["POST"] = pi.Post
	}
	if pi.Put != nil {
		ops["PUT"] = pi.Put
	}
	if pi.Patch != nil {
		ops["PATCH"] = pi.Patch
	}
	if pi.Delete != nil {
		ops["DELETE"] = pi.Delete
	}
	if pi.Head != nil {
		ops["HEAD"] = pi.Head
	}
	if pi.Options != nil {
		ops["OPTIONS"] = pi.Options
	}
	return ops
}

// Operation represents an OpenAPI operation.
type Operation struct {
	OperationID string                `json:"operationId" yaml:"operationId"`
	Summary     string                `json:"summary"     yaml:"summary"`
	Description string                `json:"description" yaml:"description"`
	Tags        []string              `json:"tags"        yaml:"tags"`
	Parameters  []Parameter           `json:"parameters"  yaml:"parameters"`
	RequestBody *RequestBody          `json:"requestBody" yaml:"requestBody"`
	Responses   map[string]Response   `json:"responses"   yaml:"responses"`
	Security    []SecurityRequirement `json:"security"    yaml:"security"`
	Deprecated  bool                  `json:"deprecated"  yaml:"deprecated"`
}

// Parameter represents an OpenAPI parameter.
type Parameter struct {
	Ref         string  `json:"$ref"        yaml:"$ref"`
	Name        string  `json:"name"        yaml:"name"`
	In          string  `json:"in"          yaml:"in"`
	Description string  `json:"description" yaml:"description"`
	Required    bool    `json:"required"    yaml:"required"`
	Schema      *Schema `json:"schema"      yaml:"schema"`
}

// RequestBody represents the request body of an operation.
type RequestBody struct {
	Description string               `json:"description" yaml:"description"`
	Required    bool                 `json:"required"    yaml:"required"`
	Content     map[string]MediaType `json:"content"     yaml:"content"`
}

// MediaType represents a media type entry.
type MediaType struct {
	Schema *Schema `json:"schema" yaml:"schema"`
}

// Schema is a simplified JSON Schema.
type Schema struct {
	Type        string             `json:"type"        yaml:"type"`
	Format      string             `json:"format"      yaml:"format"`
	Description string             `json:"description" yaml:"description"`
	Enum        []interface{}      `json:"enum"        yaml:"enum"`
	Properties  map[string]*Schema `json:"properties"  yaml:"properties"`
	Items       *Schema            `json:"items"       yaml:"items"`
	Ref         string             `json:"$ref"        yaml:"$ref"`
}

// Response represents an API response.
type Response struct {
	Description string               `json:"description" yaml:"description"`
	Content     map[string]MediaType `json:"content"     yaml:"content"`
}

// Components holds reusable schema components.
type Components struct {
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes" yaml:"securitySchemes"`
	Schemas         map[string]*Schema        `json:"schemas"         yaml:"schemas"`
	Parameters      map[string]Parameter      `json:"parameters"      yaml:"parameters"`
}

// SecurityScheme represents an OpenAPI security scheme.
type SecurityScheme struct {
	Type             string      `json:"type"             yaml:"type"`
	Description      string      `json:"description"      yaml:"description"`
	Name             string      `json:"name"             yaml:"name"`
	In               string      `json:"in"               yaml:"in"`
	Scheme           string      `json:"scheme"           yaml:"scheme"`
	BearerFormat     string      `json:"bearerFormat"     yaml:"bearerFormat"`
	Flows            *OAuthFlows `json:"flows"            yaml:"flows"`
	OpenIDConnectURL string      `json:"openIdConnectUrl" yaml:"openIdConnectUrl"`
}

// OAuthFlows holds OAuth2 flow configurations.
type OAuthFlows struct {
	AuthorizationCode *OAuthFlow `json:"authorizationCode" yaml:"authorizationCode"`
	Implicit          *OAuthFlow `json:"implicit"          yaml:"implicit"`
	Password          *OAuthFlow `json:"password"          yaml:"password"`
	ClientCredentials *OAuthFlow `json:"clientCredentials" yaml:"clientCredentials"`
}

// OAuthFlow represents a single OAuth2 flow.
type OAuthFlow struct {
	AuthorizationURL string            `json:"authorizationUrl" yaml:"authorizationUrl"`
	TokenURL         string            `json:"tokenUrl"         yaml:"tokenUrl"`
	RefreshURL       string            `json:"refreshUrl"       yaml:"refreshUrl"`
	Scopes           map[string]string `json:"scopes"           yaml:"scopes"`
}

// SecurityRequirement maps scheme names to required scopes.
type SecurityRequirement map[string][]string
