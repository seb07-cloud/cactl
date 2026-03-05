package semver

// SemverConfig holds configurable field trigger lists for determining bump levels.
type SemverConfig struct {
	MajorFields []string
	MinorFields []string
}

// DefaultSemverConfig returns sensible defaults for Conditional Access policy field triggers.
// Major: changes to user/group/app inclusion/exclusion and policy state (enabling/disabling).
// Minor: changes to conditions, grant controls, session controls.
// All other fields: patch.
func DefaultSemverConfig() SemverConfig {
	return SemverConfig{
		MajorFields: []string{
			"conditions.users.includeUsers",
			"conditions.users.includeGroups",
			"conditions.users.excludeUsers",
			"conditions.users.excludeGroups",
			"conditions.applications.includeApplications",
			"conditions.applications.excludeApplications",
			"state",
		},
		MinorFields: []string{
			"conditions",
			"grantControls",
			"sessionControls",
		},
	}
}
