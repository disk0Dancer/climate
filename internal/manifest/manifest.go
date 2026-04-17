// Package manifest manages the local climate manifest file.
package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const manifestVersion = 1

// CLIEntry represents a single registered CLI in the manifest.
type CLIEntry struct {
	Name                    string    `json:"name"`
	BinaryPath              string    `json:"binary_path"`
	SourceDir               string    `json:"source_dir"`
	Version                 string    `json:"version"`
	OpenAPIHash             string    `json:"openapi_hash"`
	OpenAPISpec             string    `json:"openapi_spec"`
	RepositoryURL           string    `json:"repository_url,omitempty"`
	RepositoryFullName      string    `json:"repository_full_name,omitempty"`
	RepositorySSHURL        string    `json:"repository_ssh_url,omitempty"`
	RepositoryDefaultBranch string    `json:"repository_default_branch,omitempty"`
	PublishedAt             time.Time `json:"published_at,omitempty"`
	LifecycleManaged        bool      `json:"lifecycle_managed,omitempty"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

// Manifest is the top-level manifest structure.
type Manifest struct {
	Version int        `json:"version"`
	CLIs    []CLIEntry `json:"clis"`
	path    string
}

// DefaultPath returns the default manifest file path.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	return filepath.Join(home, ".climate", "manifest.json"), nil
}

// Load loads the manifest from the default path, creating it if it doesn't exist.
func Load() (*Manifest, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return LoadFrom(path)
}

// LoadFrom loads the manifest from a specific path.
func LoadFrom(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Manifest{Version: manifestVersion, CLIs: []CLIEntry{}, path: path}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading manifest %s: %w", path, err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}
	m.path = path
	return &m, nil
}

// Save writes the manifest to disk.
func (m *Manifest) Save() error {
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		return fmt.Errorf("creating manifest directory: %w", err)
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("serializing manifest: %w", err)
	}
	if err := os.WriteFile(m.path, data, 0o600); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}
	return nil
}

// Upsert adds or updates a CLI entry.
func (m *Manifest) Upsert(entry CLIEntry) {
	for i, e := range m.CLIs {
		if e.Name == entry.Name {
			if entry.RepositoryURL == "" {
				entry.RepositoryURL = e.RepositoryURL
			}
			if entry.RepositoryFullName == "" {
				entry.RepositoryFullName = e.RepositoryFullName
			}
			if entry.RepositorySSHURL == "" {
				entry.RepositorySSHURL = e.RepositorySSHURL
			}
			if entry.RepositoryDefaultBranch == "" {
				entry.RepositoryDefaultBranch = e.RepositoryDefaultBranch
			}
			if entry.PublishedAt.IsZero() {
				entry.PublishedAt = e.PublishedAt
			}
			entry.LifecycleManaged = entry.LifecycleManaged || e.LifecycleManaged
			entry.CreatedAt = e.CreatedAt
			if entry.CreatedAt.IsZero() {
				entry.CreatedAt = time.Now()
			}
			entry.UpdatedAt = time.Now()
			m.CLIs[i] = entry
			return
		}
	}
	now := time.Now()
	entry.CreatedAt = now
	entry.UpdatedAt = now
	m.CLIs = append(m.CLIs, entry)
}

// Get returns a CLI entry by name.
func (m *Manifest) Get(name string) (CLIEntry, bool) {
	for _, e := range m.CLIs {
		if e.Name == name {
			return e, true
		}
	}
	return CLIEntry{}, false
}

// Remove removes a CLI entry by name.
func (m *Manifest) Remove(name string) bool {
	for i, e := range m.CLIs {
		if e.Name == name {
			m.CLIs = append(m.CLIs[:i], m.CLIs[i+1:]...)
			return true
		}
	}
	return false
}

// List returns all CLI entries. Always returns a non-nil slice.
func (m *Manifest) List() []CLIEntry {
	if m.CLIs == nil {
		return []CLIEntry{}
	}
	return m.CLIs
}

// Path returns the manifest file path.
func (m *Manifest) Path() string {
	return m.path
}
