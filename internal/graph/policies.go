package graph

import (
	"bytes"
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

// CreatePolicy creates a new Conditional Access policy and returns the
// server-assigned ID.
func (c *Client) CreatePolicy(ctx context.Context, policyJSON map[string]interface{}) (string, error) {
	body, err := json.Marshal(policyJSON)
	if err != nil {
		return "", fmt.Errorf("marshaling policy: %w", err)
	}

	url := c.baseURL + "/identity/conditionalAccess/policies"
	resp, err := c.do(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating policy: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("creating policy: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decoding create response: %w", err)
	}

	return result.ID, nil
}

// UpdatePolicy updates an existing Conditional Access policy by ID.
func (c *Client) UpdatePolicy(ctx context.Context, id string, policyJSON map[string]interface{}) error {
	body, err := json.Marshal(policyJSON)
	if err != nil {
		return fmt.Errorf("marshaling policy: %w", err)
	}

	url := c.baseURL + "/identity/conditionalAccess/policies/" + id
	resp, err := c.do(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("updating policy %s: %w", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("updating policy %s: HTTP %d: %s", id, resp.StatusCode, string(respBody))
	}

	return nil
}

// DeletePolicy deletes a Conditional Access policy by ID.
func (c *Client) DeletePolicy(ctx context.Context, id string) error {
	url := c.baseURL + "/identity/conditionalAccess/policies/" + id
	resp, err := c.do(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("deleting policy %s: %w", id, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deleting policy %s: HTTP %d: %s", id, resp.StatusCode, string(respBody))
	}

	return nil
}
