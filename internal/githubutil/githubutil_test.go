package githubutil

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestEnsureRepositoryCreatesRepo(t *testing.T) {
	client := NewClientWithBaseURL("token", "https://api.example.test", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/user":
				return jsonResponse(http.StatusOK, map[string]string{"login": "disk0Dancer"}), nil
			case "/user/repos":
				if r.Method != http.MethodPost {
					t.Fatalf("expected POST, got %s", r.Method)
				}
				return jsonResponse(http.StatusCreated, Repository{
					Name:          "petstore",
					FullName:      "disk0Dancer/petstore",
					HTMLURL:       "https://github.com/disk0Dancer/petstore",
					CloneURL:      "https://github.com/disk0Dancer/petstore.git",
					SSHURL:        "git@github.com:disk0Dancer/petstore.git",
					DefaultBranch: "main",
				}), nil
			default:
				t.Fatalf("unexpected path %s", r.URL.Path)
				return nil, nil
			}
		}),
	})

	repo, created, err := client.EnsureRepository(context.Background(), EnsureRepositoryRequest{
		Name:          "petstore",
		ReuseExisting: true,
	})
	if err != nil {
		t.Fatalf("EnsureRepository() error = %v", err)
	}
	if !created {
		t.Fatal("expected repository to be created")
	}
	if repo.FullName != "disk0Dancer/petstore" {
		t.Fatalf("FullName = %q", repo.FullName)
	}
}

func TestEnsureRepositoryReusesExisting(t *testing.T) {
	client := NewClientWithBaseURL("token", "https://api.example.test", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/user":
				return jsonResponse(http.StatusOK, map[string]string{"login": "disk0Dancer"}), nil
			case "/user/repos":
				return jsonResponse(http.StatusUnprocessableEntity, map[string]interface{}{
					"message": "Validation Failed",
					"errors": []map[string]string{
						{"resource": "Repository", "code": "already_exists"},
					},
				}), nil
			case "/repos/disk0Dancer/petstore":
				return jsonResponse(http.StatusOK, Repository{
					Name:          "petstore",
					FullName:      "disk0Dancer/petstore",
					HTMLURL:       "https://github.com/disk0Dancer/petstore",
					CloneURL:      "https://github.com/disk0Dancer/petstore.git",
					SSHURL:        "git@github.com:disk0Dancer/petstore.git",
					DefaultBranch: "main",
				}), nil
			default:
				t.Fatalf("unexpected path %s", r.URL.Path)
				return nil, nil
			}
		}),
	})

	repo, created, err := client.EnsureRepository(context.Background(), EnsureRepositoryRequest{
		Name:          "petstore",
		ReuseExisting: true,
	})
	if err != nil {
		t.Fatalf("EnsureRepository() error = %v", err)
	}
	if created {
		t.Fatal("expected repository reuse path")
	}
	if repo.SSHURL == "" {
		t.Fatal("expected ssh url to be populated")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func jsonResponse(status int, payload interface{}) *http.Response {
	body, _ := json.Marshal(payload)
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(string(body))),
	}
}
