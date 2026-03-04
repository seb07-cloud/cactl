package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Policy represents a Conditional Access policy fetched from Microsoft Graph.
type Policy struct {
	ID               string          `json:"id"`
	DisplayName      string          `json:"displayName"`
	State            string          `json:"state"`
	CreatedDateTime  string          `json:"createdDateTime"`
	ModifiedDateTime string          `json:"modifiedDateTime"`
	TemplateID       *string         `json:"templateId"`
	RawJSON          json.RawMessage `json:"-"`
}

// policiesResponse is the Graph API list response shape.
type policiesResponse struct {
	Value    []json.RawMessage `json:"value"`
	NextLink string            `json:"@odata.nextLink"`
}

// ListPolicies fetches all Conditional Access policies from the Graph API,
// following @odata.nextLink pagination to retrieve all pages.
func (c *Client) ListPolicies(ctx context.Context) ([]Policy, error) {
	url := c.baseURL + "/identity/conditionalAccess/policies"

	var all []Policy
	for url != "" {
		resp, err := c.do(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("listing policies: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("listing policies: HTTP %d: %s", resp.StatusCode, string(body))
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("reading response body: %w", err)
		}

		var page policiesResponse
		if err := json.Unmarshal(bodyBytes, &page); err != nil {
			return nil, fmt.Errorf("decoding response: %w", err)
		}

		for _, raw := range page.Value {
			var p Policy
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, fmt.Errorf("decoding policy: %w", err)
			}
			p.RawJSON = raw
			all = append(all, p)
		}

		url = page.NextLink
	}

	return all, nil
}

// GetPolicy fetches a single Conditional Access policy by ID.
func (c *Client) GetPolicy(ctx context.Context, policyID string) (*Policy, error) {
	url := c.baseURL + "/identity/conditionalAccess/policies/" + policyID

	resp, err := c.do(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("getting policy %s: %w", policyID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("getting policy %s: HTTP %d: %s", policyID, resp.StatusCode, string(body))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var p Policy
	if err := json.Unmarshal(bodyBytes, &p); err != nil {
		return nil, fmt.Errorf("decoding policy: %w", err)
	}
	p.RawJSON = bodyBytes

	return &p, nil
}
