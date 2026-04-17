// Package auth provides types and helpers for OpenAPI security scheme handling.
package auth

import "github.com/disk0Dancer/climate/internal/spec"

// SchemeType represents the type of an auth scheme.
type SchemeType string

const (
	SchemeAPIKey        SchemeType = "apiKey"
	SchemeHTTPBearer    SchemeType = "http_bearer"
	SchemeHTTPBasic     SchemeType = "http_basic"
	SchemeOAuth2        SchemeType = "oauth2"
	SchemeOpenIDConnect SchemeType = "openIdConnect"
	// SchemeUnknown is returned for unrecognised scheme types so callers can skip them.
	SchemeUnknown SchemeType = "unknown"
)

// Scheme holds normalized information about a single security scheme.
type Scheme struct {
	Name string
	Type SchemeType
	Spec spec.SecurityScheme
}

// ParseSchemes extracts and normalizes security schemes from an OpenAPI spec.
// Schemes with type SchemeUnknown are included so callers can decide whether to skip them.
func ParseSchemes(openAPI *spec.OpenAPI) []Scheme {
	var schemes []Scheme
	for name, ss := range openAPI.Components.SecuritySchemes {
		t := normalizeType(ss)
		schemes = append(schemes, Scheme{
			Name: name,
			Type: t,
			Spec: ss,
		})
	}
	return schemes
}

func normalizeType(ss spec.SecurityScheme) SchemeType {
	switch ss.Type {
	case "apiKey":
		return SchemeAPIKey
	case "http":
		switch ss.Scheme {
		case "bearer":
			return SchemeHTTPBearer
		case "basic":
			return SchemeHTTPBasic
		default:
			// e.g. "digest" or other unsupported HTTP auth schemes
			return SchemeUnknown
		}
	case "oauth2":
		return SchemeOAuth2
	case "openIdConnect":
		return SchemeOpenIDConnect
	default:
		return SchemeUnknown
	}
}

// EnvVarName returns the expected ENV variable name for a scheme and CLI name.
func EnvVarName(cliName, schemeName string, schemeType SchemeType) string {
	upper := func(s string) string {
		result := make([]byte, len(s))
		for i, b := range s {
			if b >= 'a' && b <= 'z' {
				result[i] = byte(b - 32)
			} else if b == '-' || b == '.' {
				result[i] = '_'
			} else {
				result[i] = byte(b)
			}
		}
		return string(result)
	}

	cli := upper(cliName)
	scheme := upper(schemeName)

	switch schemeType {
	case SchemeAPIKey:
		return cli + "_" + scheme + "_API_KEY"
	case SchemeHTTPBearer:
		return cli + "_TOKEN"
	case SchemeHTTPBasic:
		return cli + "_USERNAME" // password has separate var
	case SchemeOAuth2:
		return cli + "_CLIENT_ID"
	default:
		return cli + "_TOKEN"
	}
}
