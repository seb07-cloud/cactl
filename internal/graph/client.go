// Package graph provides an HTTP client for Microsoft Graph API operations.
package graph

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

const (
	defaultBaseURL = "https://graph.microsoft.com/v1.0"
	graphScope     = "https://graph.microsoft.com/.default"
)

// Client is an authenticated HTTP client for Microsoft Graph API.
type Client struct {
	baseURL    string
	httpClient *http.Client
	credential azcore.TokenCredential
	tenantID   string
}

// NewClient creates a new Graph API client that authenticates using the
// provided azcore.TokenCredential.
func NewClient(credential azcore.TokenCredential, tenantID string) *Client {
	return &Client{
		baseURL: defaultBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		credential: credential,
		tenantID:   tenantID,
	}
}

// do executes an authenticated HTTP request against the Graph API.
func (c *Client) do(ctx context.Context, method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Acquire token
	token, err := c.credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{graphScope},
	})
	if err != nil {
		return nil, fmt.Errorf("acquiring token: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}

	return resp, nil
}
