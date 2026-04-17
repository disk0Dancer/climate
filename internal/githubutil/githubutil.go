// Package githubutil provides a small GitHub REST API client used by climate
// to publish generated CLIs into managed repositories.
package githubutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.github.com"

// Repository is the subset of GitHub repository metadata used by climate.
type Repository struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	HTMLURL       string `json:"html_url"`
	CloneURL      string `json:"clone_url"`
	SSHURL        string `json:"ssh_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}

// EnsureRepositoryRequest describes the repository climate should create or reuse.
type EnsureRepositoryRequest struct {
	Owner         string
	Name          string
	Description   string
	Homepage      string
	Private       bool
	ReuseExisting bool
}

// Client is a minimal GitHub API client.
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

// NewClient creates a client that talks to the public GitHub API.
func NewClient(token string) *Client {
	return NewClientWithBaseURL(token, defaultBaseURL, &http.Client{Timeout: 30 * time.Second})
}

// NewClientWithBaseURL exists for tests and custom GitHub deployments.
func NewClientWithBaseURL(token, baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
		token:      token,
	}
}

// CurrentUser returns the login of the authenticated GitHub user.
func (c *Client) CurrentUser(ctx context.Context) (string, error) {
	var resp struct {
		Login string `json:"login"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/user", nil, &resp); err != nil {
		return "", err
	}
	if resp.Login == "" {
		return "", fmt.Errorf("github api returned an empty login")
	}
	return resp.Login, nil
}

// GetRepository fetches a repository by owner/name.
func (c *Client) GetRepository(ctx context.Context, owner, name string) (*Repository, error) {
	var repo Repository
	if err := c.doJSON(ctx, http.MethodGet, "/repos/"+url.PathEscape(owner)+"/"+url.PathEscape(name), nil, &repo); err != nil {
		return nil, err
	}
	return &repo, nil
}

// EnsureRepository creates a repository or reuses an existing one.
// The returned boolean indicates whether the repository was created by this call.
func (c *Client) EnsureRepository(ctx context.Context, req EnsureRepositoryRequest) (*Repository, bool, error) {
	if req.Name == "" {
		return nil, false, fmt.Errorf("repository name is required")
	}

	currentUser, err := c.CurrentUser(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("fetching authenticated user: %w", err)
	}

	owner := req.Owner
	if owner == "" {
		owner = currentUser
	}

	payload := map[string]interface{}{
		"name":        req.Name,
		"description": req.Description,
		"homepage":    req.Homepage,
		"private":     req.Private,
		"auto_init":   false,
	}

	endpoint := "/user/repos"
	if owner != currentUser {
		endpoint = "/orgs/" + url.PathEscape(owner) + "/repos"
	}

	var repo Repository
	statusErr := c.doJSON(ctx, http.MethodPost, endpoint, payload, &repo)
	if statusErr == nil {
		return &repo, true, nil
	}

	apiErr, ok := statusErr.(*Error)
	if ok && apiErr.StatusCode == http.StatusUnprocessableEntity && req.ReuseExisting {
		existing, getErr := c.GetRepository(ctx, owner, req.Name)
		if getErr != nil {
			return nil, false, fmt.Errorf("repository already exists but could not be fetched: %w", getErr)
		}
		return existing, false, nil
	}

	return nil, false, statusErr
}

// Error is a structured GitHub API error.
type Error struct {
	StatusCode int
	Message    string
	Details    []string
}

func (e *Error) Error() string {
	if len(e.Details) == 0 {
		return fmt.Sprintf("github api: %s (HTTP %d)", e.Message, e.StatusCode)
	}
	return fmt.Sprintf("github api: %s (HTTP %d): %s", e.Message, e.StatusCode, strings.Join(e.Details, "; "))
}

func (c *Client) doJSON(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding github request: %w", err)
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("creating github request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling github api: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading github response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiResp struct {
			Message string `json:"message"`
			Errors  []struct {
				Message  string `json:"message"`
				Resource string `json:"resource"`
				Field    string `json:"field"`
				Code     string `json:"code"`
			} `json:"errors"`
		}
		if err := json.Unmarshal(respBody, &apiResp); err != nil {
			return &Error{StatusCode: resp.StatusCode, Message: strings.TrimSpace(string(respBody))}
		}
		details := make([]string, 0, len(apiResp.Errors))
		for _, item := range apiResp.Errors {
			parts := []string{}
			if item.Resource != "" {
				parts = append(parts, item.Resource)
			}
			if item.Field != "" {
				parts = append(parts, item.Field)
			}
			if item.Code != "" {
				parts = append(parts, item.Code)
			}
			if item.Message != "" {
				parts = append(parts, item.Message)
			}
			if len(parts) > 0 {
				details = append(details, strings.Join(parts, " "))
			}
		}
		return &Error{StatusCode: resp.StatusCode, Message: apiResp.Message, Details: details}
	}

	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decoding github response: %w", err)
	}
	return nil
}
