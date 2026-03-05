package graph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// BatchRequestItem represents a single request within a Graph API batch.
type BatchRequestItem struct {
	ID     string `json:"id"`
	Method string `json:"method"`
	URL    string `json:"url"`
}

// BatchResponseItem represents a single response within a Graph API batch response.
type BatchResponseItem struct {
	ID     string          `json:"id"`
	Status int             `json:"status"`
	Body   json.RawMessage `json:"body"`
}

// BatchRequest is the top-level batch request envelope.
type BatchRequest struct {
	Requests []BatchRequestItem `json:"requests"`
}

// BatchResponse is the top-level batch response envelope.
type BatchResponse struct {
	Responses []BatchResponseItem `json:"responses"`
}

// ExecuteBatch sends a batch request to the Graph API /$batch endpoint and
// returns the individual response items. The Graph API supports up to 20
// requests per batch.
func (c *Client) ExecuteBatch(ctx context.Context, requests []BatchRequestItem) ([]BatchResponseItem, error) {
	batchReq := BatchRequest{Requests: requests}
	body, err := json.Marshal(batchReq)
	if err != nil {
		return nil, fmt.Errorf("marshaling batch request: %w", err)
	}

	url := c.baseURL + "/$batch"
	resp, err := c.do(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("executing batch request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("batch request: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var batchResp BatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		return nil, fmt.Errorf("decoding batch response: %w", err)
	}

	return batchResp.Responses, nil
}
