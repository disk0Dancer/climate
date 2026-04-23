package commands

import (
	"runtime/debug"
	"testing"
)

func TestResolvedVersionPrefersLdflagsValue(t *testing.T) {
	originalVersion := version
	originalReadBuildInfo := readBuildInfo
	defer func() {
		version = originalVersion
		readBuildInfo = originalReadBuildInfo
	}()

	version = "v1.2.3"
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		t.Fatal("readBuildInfo should not be called when explicit version is set")
		return nil, false
	}

	if got := resolvedVersion(); got != "v1.2.3" {
		t.Fatalf("resolvedVersion() = %q, want %q", got, "v1.2.3")
	}
}

func TestResolvedVersionUsesModuleVersionWhenAvailable(t *testing.T) {
	originalVersion := version
	originalReadBuildInfo := readBuildInfo
	defer func() {
		version = originalVersion
		readBuildInfo = originalReadBuildInfo
	}()

	version = "dev"
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{Version: "v9.9.9"},
		}, true
	}

	if got := resolvedVersion(); got != "v9.9.9" {
		t.Fatalf("resolvedVersion() = %q, want %q", got, "v9.9.9")
	}
}

func TestResolvedVersionFallsBackToVCSRevision(t *testing.T) {
	originalVersion := version
	originalReadBuildInfo := readBuildInfo
	defer func() {
		version = originalVersion
		readBuildInfo = originalReadBuildInfo
	}()

	version = "dev"
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return &debug.BuildInfo{
			Main: debug.Module{Version: "(devel)"},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "1234567890abcdef"},
				{Key: "vcs.modified", Value: "true"},
			},
		}, true
	}

	if got := resolvedVersion(); got != "dev+1234567890ab-dirty" {
		t.Fatalf("resolvedVersion() = %q, want %q", got, "dev+1234567890ab-dirty")
	}
}

func TestResolvedVersionFallsBackToDevWhenNoBuildInfo(t *testing.T) {
	originalVersion := version
	originalReadBuildInfo := readBuildInfo
	defer func() {
		version = originalVersion
		readBuildInfo = originalReadBuildInfo
	}()

	version = "dev"
	readBuildInfo = func() (*debug.BuildInfo, bool) {
		return nil, false
	}

	if got := resolvedVersion(); got != "dev" {
		t.Fatalf("resolvedVersion() = %q, want %q", got, "dev")
	}
}
