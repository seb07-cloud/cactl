package types

// PolicyStatus represents the status of a single tracked policy.
type PolicyStatus struct {
	Slug         string `json:"slug"`
	Version      string `json:"version"`
	LastDeployed string `json:"last_deployed"`
	DeployedBy   string `json:"deployed_by"`
	SyncStatus   string `json:"sync_status"` // "in-sync", "drifted", "missing", "unknown"
	LiveObjectID string `json:"live_object_id"`
}

// StatusOutput is the top-level JSON output for the status command.
type StatusOutput struct {
	SchemaVersion int            `json:"schema_version"`
	Tenant        string         `json:"tenant"`
	Policies      []PolicyStatus `json:"policies"`
	Summary       StatusSummary  `json:"summary"`
}

// StatusSummary counts policies by sync status.
type StatusSummary struct {
	Total   int `json:"total"`
	InSync  int `json:"in_sync"`
	Drifted int `json:"drifted"`
	Missing int `json:"missing"`
	Unknown int `json:"unknown"`
}
