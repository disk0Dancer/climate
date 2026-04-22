package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// RawBytes loads raw bytes from a file or URL without parsing.
func RawBytes(source string) ([]byte, error) {
	if isURL(source) {
		return fetchURL(source)
	}
	return os.ReadFile(source)
}

// Hash returns the SHA-256 hex hash of a spec source.
func Hash(source string) (string, error) {
	data, err := RawBytes(source)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// HashBytes returns the SHA-256 hex hash of raw bytes.
func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// NormalizeName returns a normalized, lowercase name from an API title.
func NormalizeName(title string) string {
	lower := strings.ToLower(title)
	replacer := strings.NewReplacer(
		" ", "-",
		"_", "-",
		"/", "-",
		".", "-",
	)
	name := replacer.Replace(lower)
	// Remove any remaining non-alphanumeric/hyphen characters
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "api"
	}
	return result
}

// FetchURL downloads rawURL with the given timeout and returns the body.
// It is the exported counterpart of the unexported fetchURL in loader.go.
func FetchURL(rawURL string, timeout time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", rawURL, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: HTTP %d", rawURL, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response from %s: %w", rawURL, err)
	}
	return body, nil
}

// IsURL reports whether s is an HTTP(S) URL.
// It delegates to the unexported isURL used by the loader.
func IsURL(s string) bool {
	return isURL(s)
}
