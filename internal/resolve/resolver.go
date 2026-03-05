// Package resolve provides display name resolution for Azure AD object GUIDs.
// It uses batched Graph API requests to efficiently resolve GUIDs to human-readable
// display names, with caching and graceful degradation for deleted objects.
package resolve

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/seb07-cloud/cactl/internal/graph"
)

const batchSize = 20

// BatchClient is the interface required for batch Graph API operations.
// Using an interface instead of *graph.Client enables mock injection in tests.
type BatchClient interface {
	ExecuteBatch(ctx context.Context, requests []graph.BatchRequestItem) ([]graph.BatchResponseItem, error)
}

// ObjectRef identifies an Azure AD object to resolve.
type ObjectRef struct {
	ID   string
	Type string // one of: "user", "group", "namedLocation", "servicePrincipal"
}

// Resolver resolves Azure AD object GUIDs to display names using batched
// Graph API requests. Results are cached to avoid redundant API calls.
type Resolver struct {
	client BatchClient
	cache  map[string]string
	mu     sync.Mutex
}

// NewResolver creates a new Resolver backed by the given batch client.
func NewResolver(client BatchClient) *Resolver {
	return &Resolver{
		client: client,
		cache:  make(map[string]string),
	}
}

// objectTypeToURL maps an ObjectRef type to its Graph API URL template.
func objectTypeToURL(ref ObjectRef) string {
	switch ref.Type {
	case "user":
		return fmt.Sprintf("/users/%s?$select=id,displayName", ref.ID)
	case "group":
		return fmt.Sprintf("/groups/%s?$select=id,displayName", ref.ID)
	case "namedLocation":
		return fmt.Sprintf("/identity/conditionalAccess/namedLocations/%s", ref.ID)
	case "servicePrincipal":
		return fmt.Sprintf("/servicePrincipals/%s?$select=id,displayName", ref.ID)
	default:
		return fmt.Sprintf("/directoryObjects/%s?$select=id,displayName", ref.ID)
	}
}

// ResolveAll resolves all given object references via batched Graph API requests.
// Already-cached IDs are skipped. Responses are cached for future DisplayName lookups.
// 404 responses are cached as "{id} (deleted)". Other errors cache the raw ID.
func (r *Resolver) ResolveAll(ctx context.Context, refs []ObjectRef) error {
	r.mu.Lock()
	// Filter out already-cached refs and deduplicate by ID
	seen := make(map[string]bool)
	var toResolve []ObjectRef
	for _, ref := range refs {
		if _, cached := r.cache[ref.ID]; cached {
			continue
		}
		if seen[ref.ID] {
			continue
		}
		seen[ref.ID] = true
		toResolve = append(toResolve, ref)
	}
	r.mu.Unlock()

	if len(toResolve) == 0 {
		return nil
	}

	// Chunk into batches of 20 (Graph API limit)
	for i := 0; i < len(toResolve); i += batchSize {
		end := i + batchSize
		if end > len(toResolve) {
			end = len(toResolve)
		}
		chunk := toResolve[i:end]

		// Build batch request items
		items := make([]graph.BatchRequestItem, len(chunk))
		idMap := make(map[string]string) // batch item ID -> object GUID
		for j, ref := range chunk {
			batchID := fmt.Sprintf("%d", j)
			items[j] = graph.BatchRequestItem{
				ID:     batchID,
				Method: "GET",
				URL:    objectTypeToURL(ref),
			}
			idMap[batchID] = ref.ID
		}

		responses, err := r.client.ExecuteBatch(ctx, items)
		if err != nil {
			// On batch failure, cache raw IDs for graceful degradation
			r.mu.Lock()
			for _, ref := range chunk {
				r.cache[ref.ID] = ref.ID
			}
			r.mu.Unlock()
			continue
		}

		r.mu.Lock()
		for _, resp := range responses {
			objectID := idMap[resp.ID]
			switch {
			case resp.Status == 200:
				var obj struct {
					DisplayName string `json:"displayName"`
				}
				if err := json.Unmarshal(resp.Body, &obj); err == nil && obj.DisplayName != "" {
					r.cache[objectID] = obj.DisplayName
				} else {
					r.cache[objectID] = objectID
				}
			case resp.Status == 404:
				r.cache[objectID] = fmt.Sprintf("%s (deleted)", objectID)
			default:
				r.cache[objectID] = objectID
			}
		}
		r.mu.Unlock()
	}

	return nil
}

// DisplayName returns the cached display name for the given ID.
// If the ID was not resolved, returns the raw ID as a fallback.
func (r *Resolver) DisplayName(id string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if name, ok := r.cache[id]; ok {
		return name
	}
	return id
}

// guidFields maps JSON paths to their object types for GUID extraction.
var guidFields = map[string]string{
	"includeUsers":        "user",
	"excludeUsers":        "user",
	"includeGroups":       "group",
	"excludeGroups":       "group",
	"includeLocations":    "namedLocation",
	"excludeLocations":    "namedLocation",
	"includeApplications": "servicePrincipal",
	"excludeApplications": "servicePrincipal",
}

// CollectRefs scans policy JSON maps for known GUID fields and returns ObjectRef
// slices with the correct types. It accepts raw policy JSON maps (as
// map[string]interface{}) so it can be used before the reconcile package exists.
func CollectRefs(policyMaps []map[string]interface{}) []ObjectRef {
	seen := make(map[string]bool)
	var refs []ObjectRef

	for _, pm := range policyMaps {
		extractRefs(pm, seen, &refs)
	}

	return refs
}

// extractRefs recursively walks a JSON map looking for GUID fields.
func extractRefs(obj map[string]interface{}, seen map[string]bool, refs *[]ObjectRef) {
	for key, val := range obj {
		if objType, isGUIDField := guidFields[key]; isGUIDField {
			// Value should be a slice of strings (GUIDs)
			if arr, ok := val.([]interface{}); ok {
				for _, item := range arr {
					if id, ok := item.(string); ok && !seen[id] && isGUID(id) {
						seen[id] = true
						*refs = append(*refs, ObjectRef{ID: id, Type: objType})
					}
				}
			}
		}
		// Recurse into nested maps
		if nested, ok := val.(map[string]interface{}); ok {
			extractRefs(nested, seen, refs)
		}
	}
}

// isGUID checks if a string looks like a GUID (basic length + format check).
// Known sentinel values like "All", "None", "GuestsOrExternalUsers" are not GUIDs.
func isGUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	// Check UUID format: 8-4-4-4-12
	if s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return false
	}
	return true
}
