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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/identity/conditionalAccess/policies", r.URL.Path)

		var body map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, "New Policy", body["displayName"])

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":          "new-id-123",
			"displayName": "New Policy",
		})
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	id, err := client.CreatePolicy(context.Background(), map[string]interface{}{
		"displayName": "New Policy",
		"state":       "disabled",
	})

	require.NoError(t, err)
	assert.Equal(t, "new-id-123", id)
}

func TestCreatePolicy_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
