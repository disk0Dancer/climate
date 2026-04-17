package skills_test

import (
	"strings"
	"testing"

	"github.com/disk0Dancer/climate/skills"
)

func TestClimateMDEmbedded(t *testing.T) {
	if skills.ClimateMD == "" {
		t.Fatal("ClimateMD should not be empty — skills/climate.md must be embedded")
	}
	if !strings.Contains(skills.ClimateMD, "climate.generator") {
		t.Error("skills/climate.md should mention climate.generator")
	}
	if !strings.Contains(skills.ClimateMD, "climate generate") {
		t.Error("skills/climate.md should document the generate command")
	}
	if !strings.Contains(skills.ClimateMD, "climate skill generate") {
		t.Error("skills/climate.md should document the skill generate command")
	}
	if !strings.Contains(skills.ClimateMD, "## Typical agent workflow") {
		t.Error("skills/climate.md should contain an agent workflow section")
	}
}
