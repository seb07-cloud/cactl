package graph

import "context"

// GraphClient defines the interface for Microsoft Graph API operations
// on Conditional Access policies. Implementations include the real Client
// (authenticated HTTP) and test mocks.
type GraphClient interface {
	ListPolicies(ctx context.Context) ([]Policy, error)
	GetPolicy(ctx context.Context, policyID string) (*Policy, error)
}
