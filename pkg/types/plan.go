package types

// PlanOutput is the top-level JSON output structure for `cactl plan --output json`.
// SchemaVersion enables forward-compatible tooling.
type PlanOutput struct {
	SchemaVersion int            `json:"schema_version"`
	Actions       []ActionOutput `json:"actions"`
	Summary       SummaryOutput  `json:"summary"`
	Warnings      []string       `json:"warnings,omitempty"`
}

// ActionOutput describes a single policy action in JSON output.
type ActionOutput struct {
	Slug        string       `json:"slug"`
	DisplayName string       `json:"display_name"`
	Action      string       `json:"action"`
	VersionFrom string       `json:"version_from,omitempty"`
	VersionTo   string       `json:"version_to,omitempty"`
	BumpLevel   string       `json:"bump_level,omitempty"`
	Diffs       []DiffOutput `json:"diffs,omitempty"`
	Warnings    []string     `json:"warnings,omitempty"`
}

// DiffOutput describes a single field-level diff in JSON output.
type DiffOutput struct {
	Path     string      `json:"path"`
	Type     string      `json:"type"`
	OldValue interface{} `json:"old_value,omitempty"`
	NewValue interface{} `json:"new_value,omitempty"`
}

// SummaryOutput holds the count of each action type for JSON output.
type SummaryOutput struct {
	Create    int `json:"create"`
	Update    int `json:"update"`
	Recreate  int `json:"recreate"`
	Untracked int `json:"untracked"`
	Noop      int `json:"noop"`
}
