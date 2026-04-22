// Package skill generates plain-text agent prompts that describe how to use
// a generated CLI.  The output is intentionally human- and LLM-readable so
// that an agent can paste it into its own context, system prompt, or skill
// registry without any further parsing.
package skill

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/disk0Dancer/climate/internal/auth"
	"github.com/disk0Dancer/climate/internal/manifest"
	"github.com/disk0Dancer/climate/internal/spec"
)

// Mode controls how verbose the generated prompt is.
type Mode string

const (
	// ModeFull produces one documented command per OpenAPI operation.
	ModeFull Mode = "full"
	// ModeCompact produces a shorter summary grouped only by tag.
	ModeCompact Mode = "compact"
)

// GenerateCLIPrompt returns a plain-text agent prompt that describes how to
// invoke the CLI and how to self-register it as a skill.
func GenerateCLIPrompt(entry manifest.CLIEntry, openAPI *spec.OpenAPI, mode Mode) string {
	bin := entry.Name
	if entry.BinaryPath != "" {
		bin = entry.BinaryPath
	}

	var b strings.Builder

	// ── Header ────────────────────────────────────────────────────────────────
	b.WriteString("# Skill: " + entry.Name + "\n\n")
	b.WriteString("You now have access to the `" + entry.Name + "` CLI tool ")
	b.WriteString("(generated from the " + openAPI.Info.Title + " API, version " + openAPI.Info.Version + ").\n")
	if openAPI.Info.Description != "" {
		b.WriteString("\n" + openAPI.Info.Description + "\n")
	}
	b.WriteString("\nBinary: `" + bin + "`\n")

	// ── Auth ──────────────────────────────────────────────────────────────────
	schemes := auth.ParseSchemes(openAPI)
	if len(schemes) > 0 {
		b.WriteString("\n## Authentication\n\n")
		b.WriteString("Set the following environment variables before calling any command:\n\n")
		seen := map[string]bool{}
		for _, scheme := range schemes {
			switch scheme.Type {
			case auth.SchemeAPIKey:
				v := envUpper(entry.Name) + "_" + envUpper(scheme.Name) + "_API_KEY"
				if !seen[v] {
					seen[v] = true
					b.WriteString("- `" + v + "` — API key for " + scheme.Name + "\n")
				}
			case auth.SchemeHTTPBearer:
				v := envUpper(entry.Name) + "_TOKEN"
				if !seen[v] {
					seen[v] = true
					b.WriteString("- `" + v + "` — Bearer token\n")
				}
			case auth.SchemeHTTPBasic:
				u := envUpper(entry.Name) + "_USERNAME"
				p := envUpper(entry.Name) + "_PASSWORD"
				if !seen[u] {
					seen[u] = true
					b.WriteString("- `" + u + "` — Username for basic auth\n")
					b.WriteString("- `" + p + "` — Password for basic auth\n")
				}
			case auth.SchemeOAuth2:
				id := envUpper(entry.Name) + "_CLIENT_ID"
				sc := envUpper(entry.Name) + "_CLIENT_SECRET"
				if !seen[id] {
					seen[id] = true
					b.WriteString("- `" + id + "` — OAuth2 client ID\n")
					b.WriteString("- `" + sc + "` — OAuth2 client secret\n")
				}
			}
		}
	}

	// ── Commands ──────────────────────────────────────────────────────────────
	b.WriteString("\n## Commands\n\n")
	b.WriteString("All commands accept `--output=json` (default) and return JSON.\n")
	if len(openAPI.Servers) > 0 {
		b.WriteString("Default base URL: `" + openAPI.Servers[0].URL + "` ")
		b.WriteString("(override with `--base-url` or `" + envUpper(entry.Name) + "_BASE_URL`).\n")
		if len(openAPI.Servers[0].Variables) > 0 {
			varNames := make([]string, 0, len(openAPI.Servers[0].Variables))
			for name := range openAPI.Servers[0].Variables {
				varNames = append(varNames, name)
			}
			sort.Strings(varNames)
			b.WriteString("\nServer URL variables:\n")
			for _, name := range varNames {
				v := openAPI.Servers[0].Variables[name]
				flag := "--server-var-" + kebabCase(name)
				env := serverVariableEnvName(entry.Name, name)
				def := v.Default
				if def == "" {
					def = "(empty)"
				}
				b.WriteString(fmt.Sprintf("- `%s` / `%s` (default: `%s`)\n", flag, env, def))
			}
		}
	}
	b.WriteString("\n")

	if mode == ModeCompact {
		writeCompactSection(&b, bin, openAPI)
	} else {
		writeFullSection(&b, bin, openAPI)
	}

	// ── Output & errors ───────────────────────────────────────────────────────
	b.WriteString("\n## Output format\n\n")
	b.WriteString("On success (HTTP 2xx) the command exits 0 and prints the API response body as JSON.\n\n")
	b.WriteString("On error the command exits non-zero and prints to stderr:\n\n")
	b.WriteString("```json\n")
	b.WriteString("{\n")
	b.WriteString("  \"error\": {\n")
	b.WriteString("    \"status\":  <HTTP status code, or 0 for a CLI error>,\n")
	b.WriteString("    \"code\":    \"<HTTPError | CliError>\",\n")
	b.WriteString("    \"message\": \"<human-readable description>\",\n")
	b.WriteString("    \"raw\":     <original API response body, if any>\n")
	b.WriteString("  }\n")
	b.WriteString("}\n")
	b.WriteString("```\n")

	// ── Self-registration instructions ───────────────────────────────────────
	b.WriteString("\n## How to register this as your skill\n\n")
	b.WriteString("1. Remember skill id `cli." + entry.Name + "`.\n")
	b.WriteString("2. Whenever the user asks about " + openAPI.Info.Title + ", use the commands listed above.\n")
	b.WriteString("3. Always append `--output=json` and parse stdout as JSON.\n")
	b.WriteString("4. If a command needs authentication, check for the required environment variable first ")
	b.WriteString("and ask the user to provide it if missing.\n")
	b.WriteString("5. On error, surface the `error.message` field to the user.\n")

	return b.String()
}

// ── internal helpers ──────────────────────────────────────────────────────────

func writeFullSection(b *strings.Builder, bin string, openAPI *spec.OpenAPI) {
	// Group operations by tag, preserving a stable order.
	type opEntry struct {
		tag     string
		method  string
		path    string
		subCmd  string
		summary string
		params  []spec.Parameter
		hasBody bool
	}

	tagOps := map[string][]opEntry{}
	tagOrder := []string{}
	seen := map[string]bool{}

	for path, item := range openAPI.Paths {
		for method, op := range item.Operations() {
			tag := "default"
			if len(op.Tags) > 0 {
				tag = op.Tags[0]
			}
			if !seen[tag] {
				seen[tag] = true
				tagOrder = append(tagOrder, tag)
			}
			tagOps[tag] = append(tagOps[tag], opEntry{
				tag:     tag,
				method:  method,
				path:    path,
				subCmd:  operationSubCmd(op, method, path),
				summary: op.Summary,
				params:  op.Parameters,
				hasBody: op.RequestBody != nil,
			})
		}
	}
	sort.Strings(tagOrder)

	for _, tag := range tagOrder {
		ops := tagOps[tag]
		// Sort ops within the tag for deterministic output.
		sort.Slice(ops, func(i, j int) bool {
			return ops[i].subCmd < ops[j].subCmd
		})

		b.WriteString("### " + tag + "\n\n")

		for _, op := range ops {
			// Build the example command line.
			var cmdParts []string
			cmdParts = append(cmdParts, bin, tag, op.subCmd)

			var paramDescs []string
			for _, p := range op.params {
				flag := "--" + kebabCase(p.Name)
				placeholder := " <" + p.Name + ">"
				if p.Required {
					cmdParts = append(cmdParts, flag+placeholder)
				} else {
					cmdParts = append(cmdParts, "["+flag+placeholder+"]")
				}
				desc := p.Description
				if desc == "" {
					desc = p.Name
				}
				req := ""
				if p.Required {
					req = " (required)"
				}
				paramDescs = append(paramDescs, fmt.Sprintf("  - `%s`%s — %s", flag, req, desc))
			}
			if op.hasBody {
				cmdParts = append(cmdParts, "[--data-json '<json>']", "[--data-file <path>]")
				paramDescs = append(paramDescs, "  - `--data-json` — inline JSON request body")
				paramDescs = append(paramDescs, "  - `--data-file` — path to a JSON file with the request body")
			}
			cmdParts = append(cmdParts, "--output=json")

			summary := op.summary
			if summary == "" {
				summary = fmt.Sprintf("%s %s", op.method, op.path)
			}
			b.WriteString("**" + summary + "**\n\n")
			b.WriteString("```\n" + strings.Join(cmdParts, " ") + "\n```\n")

			if len(paramDescs) > 0 {
				b.WriteString("\nParameters:\n")
				for _, d := range paramDescs {
					b.WriteString(d + "\n")
				}
			}
			b.WriteString("\n")
		}
	}
}

func writeCompactSection(b *strings.Builder, bin string, openAPI *spec.OpenAPI) {
	tagDescs := map[string]string{}
	for _, t := range openAPI.Tags {
		tagDescs[t.Name] = t.Description
	}

	seenTags := map[string]bool{}
	var tags []string
	for _, item := range openAPI.Paths {
		for _, op := range item.Operations() {
			tag := "default"
			if len(op.Tags) > 0 {
				tag = op.Tags[0]
			}
			if !seenTags[tag] {
				seenTags[tag] = true
				tags = append(tags, tag)
			}
		}
	}
	sort.Strings(tags)

	for _, tag := range tags {
		desc := tagDescs[tag]
		if desc == "" {
			desc = "Operations for " + tag
		}
		b.WriteString("### " + tag + "\n\n")
		b.WriteString(desc + "\n\n")
		b.WriteString("```\n" + bin + " " + tag + " <subcommand> [flags] --output=json\n```\n\n")
		b.WriteString("Run `" + bin + " " + tag + " --help` to see all subcommands.\n\n")
	}
}

func operationSubCmd(op *spec.Operation, method, path string) string {
	if op.OperationID != "" {
		parts := strings.SplitN(op.OperationID, "_", 2)
		if len(parts) == 2 {
			return kebabCase(parts[1])
		}
		return kebabCase(op.OperationID)
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
	case "PUT", "PATCH":
		return "update"
	case "DELETE":
		return "delete"
	default:
		return strings.ToLower(method)
	}
}

func envUpper(s string) string {
	return strings.ToUpper(strings.NewReplacer("-", "_", ".", "_").Replace(s))
}

func kebabCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteRune('-')
			}
			b.WriteRune(r + 32)
		} else if r == '_' || r == ' ' {
			b.WriteRune('-')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func serverVariableEnvName(cliName, name string) string {
	var b strings.Builder
	prevLowerOrDigit := false
	prevUnderscore := false

	for _, r := range name {
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
	return envUpper(cliName) + "_SERVER_VAR_" + suffix
}
