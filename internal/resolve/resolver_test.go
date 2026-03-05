package resolve

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/seb07-cloud/cactl/internal/graph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBatchClient records calls and returns configured responses.
type mockBatchClient struct {
	calls     [][]graph.BatchRequestItem
	responses [][]graph.BatchResponseItem
	callIndex int
}

func (m *mockBatchClient) ExecuteBatch(_ context.Context, requests []graph.BatchRequestItem) ([]graph.BatchResponseItem, error) {
	m.calls = append(m.calls, requests)
	idx := m.callIndex
	m.callIndex++
	if idx < len(m.responses) {
		return m.responses[idx], nil
	}
	return nil, nil
}

func TestResolveAll_Success(t *testing.T) {
	mock := &mockBatchClient{
		responses: [][]graph.BatchResponseItem{
			{
				{ID: "0", Status: 200, Body: json.RawMessage(`{"id":"user-guid-1","displayName":"Alice Smith"}`)},
				{ID: "1", Status: 200, Body: json.RawMessage(`{"id":"group-guid-1","displayName":"IT Admins"}`)},
			},
		},
	}

	r := NewResolver(mock)
	err := r.ResolveAll(context.Background(), []ObjectRef{
		{ID: "user-guid-1", Type: "user"},
		{ID: "group-guid-1", Type: "group"},
	})

	require.NoError(t, err)
	assert.Equal(t, "Alice Smith", r.DisplayName("user-guid-1"))
	assert.Equal(t, "IT Admins", r.DisplayName("group-guid-1"))
	assert.Len(t, mock.calls, 1)
}

func TestResolveAll_404Deleted(t *testing.T) {
	mock := &mockBatchClient{
		responses: [][]graph.BatchResponseItem{
			{
				{ID: "0", Status: 404, Body: json.RawMessage(`{"error":{"code":"Request_ResourceNotFound"}}`)},
			},
		},
	}

	r := NewResolver(mock)
	err := r.ResolveAll(context.Background(), []ObjectRef{
		{ID: "deleted-group-id", Type: "group"},
	})

	require.NoError(t, err)
	assert.Equal(t, "deleted-group-id (deleted)", r.DisplayName("deleted-group-id"))
}

func TestResolveAll_CachedSkipped(t *testing.T) {
	mock := &mockBatchClient{
		responses: [][]graph.BatchResponseItem{
			{
				{ID: "0", Status: 200, Body: json.RawMessage(`{"id":"user-1","displayName":"Alice"}`)},
			},
		},
	}

	r := NewResolver(mock)

	// First resolution
	err := r.ResolveAll(context.Background(), []ObjectRef{
		{ID: "user-1", Type: "user"},
	})
	require.NoError(t, err)
	assert.Len(t, mock.calls, 1)

	// Second resolution with same ID -- should not make another call
	err = r.ResolveAll(context.Background(), []ObjectRef{
		{ID: "user-1", Type: "user"},
	})
	require.NoError(t, err)
	assert.Len(t, mock.calls, 1, "cached ID should not trigger another batch call")
}

func TestCollectRefs(t *testing.T) {
	policyJSON := map[string]interface{}{
		"displayName": "Test Policy",
		"conditions": map[string]interface{}{
			"users": map[string]interface{}{
				"includeUsers":  []interface{}{"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"},
				"excludeGroups": []interface{}{"11111111-2222-3333-4444-555555555555"},
			},
			"locations": map[string]interface{}{
				"includeLocations": []interface{}{"99999999-8888-7777-6666-555555555555"},
			},
			"applications": map[string]interface{}{
				"includeApplications": []interface{}{"All"},
			},
		},
	}

	refs := CollectRefs([]map[string]interface{}{policyJSON})

	// "All" should be filtered out (not a GUID)
	assert.Len(t, refs, 3)

	// Build a map for easier assertion
	refMap := make(map[string]string)
	for _, ref := range refs {
		refMap[ref.ID] = ref.Type
	}

	assert.Equal(t, "user", refMap["aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"])
	assert.Equal(t, "group", refMap["11111111-2222-3333-4444-555555555555"])
	assert.Equal(t, "namedLocation", refMap["99999999-8888-7777-6666-555555555555"])
}

func TestDisplayName_Unknown(t *testing.T) {
	r := NewResolver(&mockBatchClient{})
	assert.Equal(t, "unknown-id", r.DisplayName("unknown-id"))
}

func TestResolveAll_Batching(t *testing.T) {
	// Create 25 refs -- should result in 2 batch calls (20 + 5)
	var refs []ObjectRef
	for i := 0; i < 25; i++ {
		id := fmt.Sprintf("aaaaaaaa-bbbb-cccc-dddd-%012d", i)
		refs = append(refs, ObjectRef{ID: id, Type: "user"})
	}

	// Prepare responses for both batches
	batch1Resp := make([]graph.BatchResponseItem, 20)
	for i := 0; i < 20; i++ {
		batch1Resp[i] = graph.BatchResponseItem{
			ID:     fmt.Sprintf("%d", i),
			Status: 200,
			Body:   json.RawMessage(fmt.Sprintf(`{"displayName":"User %d"}`, i)),
		}
	}
	batch2Resp := make([]graph.BatchResponseItem, 5)
	for i := 0; i < 5; i++ {
		batch2Resp[i] = graph.BatchResponseItem{
			ID:     fmt.Sprintf("%d", i),
			Status: 200,
			Body:   json.RawMessage(fmt.Sprintf(`{"displayName":"User %d"}`, i+20)),
		}
	}

	mock := &mockBatchClient{
		responses: [][]graph.BatchResponseItem{batch1Resp, batch2Resp},
	}

	r := NewResolver(mock)
	err := r.ResolveAll(context.Background(), refs)

	require.NoError(t, err)
	assert.Len(t, mock.calls, 2, "25 refs should produce 2 batch calls")
	assert.Len(t, mock.calls[0], 20, "first batch should have 20 items")
	assert.Len(t, mock.calls[1], 5, "second batch should have 5 items")
}
