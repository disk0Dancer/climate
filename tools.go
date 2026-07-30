// Package tools pins dependencies that generated CLIs require at build time but
// that climate itself does not import directly. The blank import keeps
// filippo.io/age (used by the generated internal/secrets package) in the module
// graph so `go mod tidy`/`go mod download` retain it and it stays in the module
// cache for hermetic generate-and-build tests.
//
// This file has no build tag on purpose: a `//go:build tools` constraint would
// exclude it from the default package list, letting a plain `go mod tidy` drop
// the dependency. Nothing imports this package, so it adds no weight to the
// climate binary.
package tools

import (
	_ "filippo.io/age"
	_ "github.com/itchyny/gojq"
)
