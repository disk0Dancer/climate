//go:build tools

// Package tools pins dependencies that generated CLIs require at build time but
// that climate itself does not import directly. Keeping the blank import here
// makes `go mod tidy` retain filippo.io/age (used by the generated
// internal/secrets package) so it stays in the module cache for hermetic
// generate-and-build tests.
package tools

import _ "filippo.io/age"
