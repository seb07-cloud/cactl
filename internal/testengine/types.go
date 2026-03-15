package testengine

// TestSpec defines a test specification loaded from a YAML file.
// It contains a set of scenarios that describe sign-in contexts and
// expected CA policy evaluation outcomes.
type TestSpec struct {
	Name        string     `yaml:"name"`
	Description string     `yaml:"description,omitempty"`
	Policies    []string   `yaml:"policies,omitempty"` // Filter to specific policy slugs; empty = all
	Scenarios   []Scenario `yaml:"scenarios"`
}

// Scenario defines a single test case: a sign-in context and the expected outcome.
type Scenario struct {
	Name    string          `yaml:"name"`
	Context SignInContext   `yaml:"context"`
	Expect  ExpectedOutcome `yaml:"expect"`
}

// SignInContext describes the attributes of a simulated sign-in attempt.
type SignInContext struct {
	User            string   `yaml:"user,omitempty"`            // GUID, "any", "All", "guest"
	Groups          []string `yaml:"groups,omitempty"`          // Group GUIDs the user belongs to
	Roles           []string `yaml:"roles,omitempty"`           // Directory role GUIDs the user holds
	Application     string   `yaml:"application,omitempty"`     // App GUID or "All"
	ClientAppType   string   `yaml:"clientAppType,omitempty"`   // browser, mobileAppsAndDesktopClients, exchangeActiveSync, other, all
	Platform        string   `yaml:"platform,omitempty"`        // android, iOS, windows, macOS, linux, windowsPhone
	Location        string   `yaml:"location,omitempty"`        // GUID, "trusted", "untrusted", "All"
	SignInRiskLevel string   `yaml:"signInRiskLevel,omitempty"` // none, low, medium, high
	UserRiskLevel   string   `yaml:"userRiskLevel,omitempty"`   // none, low, medium, high
}

// ExpectedOutcome describes the expected result of evaluating policies against a sign-in context.
type ExpectedOutcome struct {
	Result          string                 `yaml:"result"`                    // block, grant, notApplicable
	Controls        []string               `yaml:"controls,omitempty"`        // Expected grant controls (e.g., mfa, compliantDevice)
	SessionControls map[string]interface{} `yaml:"sessionControls,omitempty"` // Expected session controls
}

// PolicyWithSlug pairs a policy slug with its raw JSON data.
type PolicyWithSlug struct {
	Slug string
	Data map[string]interface{}
}

// EvalResult represents the outcome of evaluating a single policy.
type EvalResult int

const (
	// ResultNotApplicable means the policy's conditions did not match the sign-in context.
	ResultNotApplicable EvalResult = iota
	// ResultBlock means the policy matched and blocks access.
	ResultBlock
	// ResultGrant means the policy matched and grants access (possibly with controls).
	ResultGrant
)

// String returns the human-readable name of the evaluation result.
func (r EvalResult) String() string {
	switch r {
	case ResultNotApplicable:
		return "notApplicable"
	case ResultBlock:
		return "block"
	case ResultGrant:
		return "grant"
	default:
		return "unknown"
	}
}

// PolicyDecision is the result of evaluating a single policy against a sign-in context.
type PolicyDecision struct {
	PolicySlug      string
	Result          EvalResult
	GrantControls   []string
	Operator        string // "AND" or "OR"
	SessionControls map[string]interface{}
}

// CombinedDecision is the combined result of evaluating all applicable policies.
type CombinedDecision struct {
	Result           EvalResult
	GrantControls    []string
	SessionControls  map[string]interface{}
	MatchingPolicies []string
}

// ScenarioResult records the outcome of a single scenario evaluation.
type ScenarioResult struct {
	ScenarioName     string
	Passed           bool
	Expected         ExpectedOutcome
	Got              CombinedDecision
	MatchingPolicies []string
	Error            string
}

// TestReport aggregates results across all test files.
type TestReport struct {
	Files []FileResult
}

// FileResult aggregates results for a single test file.
type FileResult struct {
	File      string
	Scenarios []ScenarioResult
	Passed    int
	Failed    int
	Errors    int
}
