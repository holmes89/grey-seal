// Package github adapts a small slice of the GitHub REST API to the
// agent.PullRequestOpener interface. Deliberately not the GitHub MCP server
// — this opens the PR directly from grey-seal's own backend once an agent
// run's outcome is satisfied, so no MCP server declaration or vault
// credential is required on the Managed Agents side.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	agentsvc "github.com/holmes89/grey-seal/lib/greyseal/agent"
)

// Client calls the GitHub REST API directly.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient builds a Client against api.github.com.
func NewClient() *Client {
	return &Client{
		baseURL:    "https://api.github.com",
		httpClient: &http.Client{},
	}
}

var _ agentsvc.PullRequestOpener = (*Client)(nil)

type createPullRequestBody struct {
	Title string `json:"title"`
	Head  string `json:"head"`
	Base  string `json:"base"`
	Body  string `json:"body,omitempty"`
}

type createPullRequestResponse struct {
	HTMLURL string `json:"html_url"`
	Message string `json:"message"` // populated on error responses
}

func (c *Client) OpenPullRequest(ctx context.Context, req agentsvc.OpenPullRequestRequest) (string, error) {
	owner, repo, err := parseOwnerRepo(req.RepoURL)
	if err != nil {
		return "", err
	}

	base := req.Base
	if base == "" {
		base = "main"
	}

	reqBody, err := json.Marshal(createPullRequestBody{
		Title: req.Title,
		Head:  req.Branch,
		Base:  base,
		Body:  req.Body,
	})
	if err != nil {
		return "", fmt.Errorf("marshal pull request body: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/%s/pulls", c.baseURL, owner, repo)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("create pull request request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+req.Token)
	httpReq.Header.Set("Accept", "application/vnd.github+json")
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("create pull request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	var body createPullRequestResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("decode create pull request response: %w", err)
	}
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("create pull request returned status %d: %s", resp.StatusCode, body.Message)
	}
	return body.HTMLURL, nil
}

// parseOwnerRepo extracts "owner", "repo" from a GitHub repo URL, e.g.
// "https://github.com/holmes89/firefly" or "git@github.com:holmes89/firefly.git".
func parseOwnerRepo(repoURL string) (owner, repo string, err error) {
	trimmed := strings.TrimSuffix(repoURL, ".git")
	trimmed = strings.TrimPrefix(trimmed, "git@github.com:")
	trimmed = strings.TrimPrefix(trimmed, "https://github.com/")
	trimmed = strings.TrimPrefix(trimmed, "http://github.com/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("could not parse owner/repo from URL %q", repoURL)
	}
	return parts[0], parts[1], nil
}
