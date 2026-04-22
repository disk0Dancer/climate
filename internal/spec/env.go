package spec

import (
	"strings"
	"unicode"
)

// ServerVariableEnvName builds a normalized environment variable name for an
// OpenAPI server variable override, using an uppercase CLI prefix.
func ServerVariableEnvName(cliPrefixUpper, variableName string) string {
	var b strings.Builder
	prevLowerOrDigit := false
	prevUnderscore := false

	for _, r := range variableName {
		switch {
		case unicode.IsUpper(r):
			if prevLowerOrDigit && !prevUnderscore && b.Len() > 0 {
				b.WriteRune('_')
			}
			b.WriteRune(r)
			prevLowerOrDigit = false
			prevUnderscore = false
		case unicode.IsLower(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToUpper(r))
			prevLowerOrDigit = true
			prevUnderscore = false
		default:
			if !prevUnderscore && b.Len() > 0 {
				b.WriteRune('_')
				prevUnderscore = true
			}
			prevLowerOrDigit = false
		}
	}

	suffix := strings.Trim(b.String(), "_")
	if suffix == "" {
		suffix = "VAR"
	}
	return cliPrefixUpper + "_SERVER_VAR_" + suffix
}
