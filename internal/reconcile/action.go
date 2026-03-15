package reconcile

import "fmt"

// ActionType classifies the reconciliation action for a policy.
type ActionType int

const (
	// ActionNoop means the policy is in sync (no changes needed).
	ActionNoop ActionType = iota
	// ActionCreate means the policy exists in backend but not yet deployed.
	ActionCreate
	// ActionUpdate means the policy exists in both but has drifted.
	ActionUpdate
	// ActionRecreate means the policy was tracked but deleted from live (ghost).
	ActionRecreate
	// ActionUntracked means the policy exists in live but is not tracked.
	ActionUntracked
	// ActionDuplicate means multiple live policies share the same displayName.
	ActionDuplicate
)

// String returns a human-readable label for the action type.
func (a ActionType) String() string {
	switch a {
	case ActionNoop:
		return "noop"
	case ActionCreate:
		return "create"
	case ActionUpdate:
		return "update"
	case ActionRecreate:
		return "recreate"
	case ActionUntracked:
		return "untracked"
	case ActionDuplicate:
		return "duplicate"
	default:
		return fmt.Sprintf("unknown(%d)", int(a))
	}
}

// PolicyAction describes a single reconciliation action for a policy.
type PolicyAction struct {
	Slug         string
	Action       ActionType
	DisplayName  string
	BackendJSON  map[string]interface{}
	LiveJSON     map[string]interface{}
	LiveObjectID string
	Diff         []FieldDiff
	Warnings     []string
	// DuplicateIDs holds the live object IDs of duplicate policies (same displayName).
	// Only populated for ActionDuplicate actions.
	DuplicateIDs []string
	// Version fields are populated by the plan command after semver computation.
	VersionFrom string
	VersionTo   string
	BumpLevel   string
}
