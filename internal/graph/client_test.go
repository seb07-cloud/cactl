package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTokenCredential returns a static token for testing.
type mockTokenCredential struct {
	token string
}

func (m *mockTokenCredential) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: m.token}, nil
}

// newTestClient creates a Client pointed at the given test server with a mock credential.
func newTestClient(serverURL string) *Client {
	c := NewClient(&mockTokenCredential{token: "test-token-123"}, "test-tenant")
	c.baseURL = serverURL
	return c
}

func TestListPolicies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"value": []map[string]interface{}{
				{"id": "policy-1", "displayName": "CA001: Require MFA", "state": "enabled"},
				{"id": "policy-2", "displayName": "CA002: Block legacy auth", "state": "disabled"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	policies, err := client.ListPolicies(context.Background())

	require.NoError(t, err)
	assert.Len(t, policies, 2)
	assert.Equal(t, "policy-1", policies[0].ID)
	assert.Equal(t, "CA001: Require MFA", policies[0].DisplayName)
	assert.Equal(t, "policy-2", policies[1].ID)
	assert.Equal(t, "CA002: Block legacy auth", policies[1].DisplayName)
}

func TestListPoliciesPagination(t *testing.T) {
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")

		if count == 1 {
			// Page 1: includes nextLink
			resp := fmt.Sprintf(`{
				"value": [{"id": "policy-1", "displayName": "Policy 1", "state": "enabled"}],
				"@odata.nextLink": "%s/page2"
			}`, r.Host) // Use placeholder; we'll fix the URL below
			// Actually need full URL with scheme
			resp = fmt.Sprintf(`{
				"value": [{"id": "policy-1", "displayName": "Policy 1", "state": "enabled"}],
				"@odata.nextLink": "http://%s/page2"
			}`, r.Host)
			w.Write([]byte(resp))
		} else {
			// Page 2: no nextLink
			resp := `{
				"value": [{"id": "policy-2", "displayName": "Policy 2", "state": "disabled"}]
			}`
			w.Write([]byte(resp))
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	policies, err := client.ListPolicies(context.Background())

	require.NoError(t, err)
	assert.Len(t, policies, 2)
	assert.Equal(t, "policy-1", policies[0].ID)
	assert.Equal(t, "policy-2", policies[1].ID)
}

func TestListPoliciesAuthHeader(t *testing.T) {
	var capturedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"value": []}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.ListPolicies(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "Bearer test-token-123", capturedAuth)
}

func TestGetPolicy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Path, "/identity/conditionalAccess/policies/policy-42")
		resp := map[string]interface{}{
			"id":          "policy-42",
			"displayName": "CA042: Test Policy",
			"state":       "enabled",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	p, err := client.GetPolicy(context.Background(), "policy-42")

	require.NoError(t, err)
	assert.Equal(t, "policy-42", p.ID)
	assert.Equal(t, "CA042: Test Policy", p.DisplayName)
	assert.NotEmpty(t, p.RawJSON, "RawJSON should be populated")
}

func TestCreatePolicy(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		switch {
		case callCount == 1 && r.Method == http.MethodGet:
			// Read-before-write: ListPolicies returns empty (no duplicate)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"value": []interface{}{},
			})
		case callCount == 2 && r.Method == http.MethodPost:
			// Actual create
			var body map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&body)
			require.NoError(t, err)
			assert.Equal(t, "New Policy", body["displayName"])

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":          "new-id-123",
				"displayName": "New Policy",
			})
		case callCount == 3 && r.Method == http.MethodGet:
			// Post-create verification: GetPolicy
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":          "new-id-123",
				"displayName": "New Policy",
			})
		default:
			t.Errorf("unexpected request #%d: %s %s", callCount, r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	id, err := client.CreatePolicy(context.Background(), map[string]interface{}{
		"displayName": "New Policy",
		"state":       "disabled",
	})

	require.NoError(t, err)
	assert.Equal(t, "new-id-123", id)
	assert.Equal(t, 3, callCount, "expected 3 requests: list, create, verify")
}

func TestCreatePolicy_ExistingDuplicate(t *testing.T) {
	// Read-before-write should return existing ID instead of POSTing
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "should only GET, never POST")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"value": []interface{}{
				map[string]interface{}{
					"id":          "existing-id-456",
					"displayName": "New Policy",
				},
			},
		})
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	id, err := client.CreatePolicy(context.Background(), map[string]interface{}{
		"displayName": "New Policy",
		"state":       "disabled",
	})

	require.NoError(t, err)
	assert.Equal(t, "existing-id-456", id)
}

func TestCreatePolicy_Error(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		if callCount == 1 && r.Method == http.MethodGet {
			// Read-before-write: no existing policy
			json.NewEncoder(w).Encode(map[string]interface{}{
				"value": []interface{}{},
			})
			return
		}
		// POST fails
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": {"code": "BadRequest", "message": "Invalid policy"}}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.CreatePolicy(context.Background(), map[string]interface{}{
		"displayName": "Bad Policy",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
	assert.Contains(t, err.Error(), "BadRequest")
}

func TestUpdatePolicy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/identity/conditionalAccess/policies/policy-99", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	err := client.UpdatePolicy(context.Background(), "policy-99", map[string]interface{}{
		"state": "enabled",
	})

	require.NoError(t, err)
}

func TestDeletePolicy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/identity/conditionalAccess/policies/policy-77", r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	err := client.DeletePolicy(context.Background(), "policy-77")

	require.NoError(t, err)
}

func TestExecuteBatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/$batch", r.URL.Path)

		var batchReq BatchRequest
		err := json.NewDecoder(r.Body).Decode(&batchReq)
		require.NoError(t, err)
		assert.Len(t, batchReq.Requests, 2)

		resp := BatchResponse{
			Responses: []BatchResponseItem{
				{ID: "1", Status: 200, Body: json.RawMessage(`{"id":"user-1","displayName":"Alice"}`)},
				{ID: "2", Status: 200, Body: json.RawMessage(`{"id":"group-1","displayName":"Admins"}`)},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	responses, err := client.ExecuteBatch(context.Background(), []BatchRequestItem{
		{ID: "1", Method: "GET", URL: "/users/user-1?$select=id,displayName"},
		{ID: "2", Method: "GET", URL: "/groups/group-1?$select=id,displayName"},
	})

	require.NoError(t, err)
	assert.Len(t, responses, 2)
	assert.Equal(t, "1", responses[0].ID)
	assert.Equal(t, 200, responses[0].Status)
	assert.Equal(t, "2", responses[1].ID)
	assert.Equal(t, 200, responses[1].Status)

	// Verify we can decode response bodies
	var user struct {
		DisplayName string `json:"displayName"`
	}
	require.NoError(t, json.Unmarshal(responses[0].Body, &user))
	assert.Equal(t, "Alice", user.DisplayName)
}

func TestListPoliciesHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error": {"code": "Authorization_RequestDenied"}}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.ListPolicies(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

// MockGraphClient is a test double implementing GraphClient via configurable
// function fields. This allows table-driven tests without httptest servers.
type MockGraphClient struct {
	ListPoliciesFunc func(ctx context.Context) ([]Policy, error)
	GetPolicyFunc    func(ctx context.Context, policyID string) (*Policy, error)
}

// Verify MockGraphClient implements GraphClient at compile time.
var _ GraphClient = (*MockGraphClient)(nil)

func (m *MockGraphClient) ListPolicies(ctx context.Context) ([]Policy, error) {
	return m.ListPoliciesFunc(ctx)
}

func (m *MockGraphClient) GetPolicy(ctx context.Context, policyID string) (*Policy, error) {
	return m.GetPolicyFunc(ctx, policyID)
}

func TestMockGraphClient(t *testing.T) {
	tests := []struct {
		name      string
		mock      *MockGraphClient
		run       func(t *testing.T, c GraphClient)
	}{
		{
			name: "ListPolicies returns expected policies",
			mock: &MockGraphClient{
				ListPoliciesFunc: func(_ context.Context) ([]Policy, error) {
					return []Policy{
						{ID: "p1", DisplayName: "Policy One", State: "enabled"},
						{ID: "p2", DisplayName: "Policy Two", State: "disabled"},
					}, nil
				},
			},
			run: func(t *testing.T, c GraphClient) {
				policies, err := c.ListPolicies(context.Background())
				require.NoError(t, err)
				assert.Len(t, policies, 2)
				assert.Equal(t, "p1", policies[0].ID)
				assert.Equal(t, "Policy One", policies[0].DisplayName)
				assert.Equal(t, "p2", policies[1].ID)
			},
		},
		{
			name: "ListPolicies returns error",
			mock: &MockGraphClient{
				ListPoliciesFunc: func(_ context.Context) ([]Policy, error) {
					return nil, fmt.Errorf("network timeout")
				},
			},
			run: func(t *testing.T, c GraphClient) {
				policies, err := c.ListPolicies(context.Background())
				require.Error(t, err)
				assert.Nil(t, policies)
				assert.Contains(t, err.Error(), "network timeout")
			},
		},
		{
			name: "GetPolicy returns expected policy by ID",
			mock: &MockGraphClient{
				GetPolicyFunc: func(_ context.Context, id string) (*Policy, error) {
					if id == "policy-42" {
						return &Policy{ID: "policy-42", DisplayName: "CA042: Test", State: "enabled"}, nil
					}
					return nil, fmt.Errorf("not found: %s", id)
				},
			},
			run: func(t *testing.T, c GraphClient) {
				p, err := c.GetPolicy(context.Background(), "policy-42")
				require.NoError(t, err)
				assert.Equal(t, "policy-42", p.ID)
				assert.Equal(t, "CA042: Test", p.DisplayName)
			},
		},
		{
			name: "GetPolicy returns error for unknown ID",
			mock: &MockGraphClient{
				GetPolicyFunc: func(_ context.Context, id string) (*Policy, error) {
					return nil, fmt.Errorf("not found: %s", id)
				},
			},
			run: func(t *testing.T, c GraphClient) {
				p, err := c.GetPolicy(context.Background(), "unknown-id")
				require.Error(t, err)
				assert.Nil(t, p)
				assert.Contains(t, err.Error(), "not found: unknown-id")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, tc.mock)
		})
	}
}
