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
