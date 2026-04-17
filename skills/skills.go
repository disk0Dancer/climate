// Package skills embeds the static Markdown skill files that ship with climate.
// Each file is a plain-text prompt an LLM agent can read to self-register a skill.
package skills

import _ "embed"

// ClimateMD is the content of skills/climate.md — the skill prompt for
// climate.generator itself.
//
//go:embed climate.md
var ClimateMD string
