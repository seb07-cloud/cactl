package testengine

// EvaluatePolicy evaluates a single CA policy against a sign-in context.
// It returns a PolicyDecision indicating whether the policy blocks, grants, or does not apply.
//
// Evaluation rules:
//   - state == "disabled" -> ResultNotApplicable
//   - state == "enabledForReportingButNotEnforced" -> evaluate normally (treat as enabled)
//   - state == "enabled" -> evaluate conditions
//   - All conditions must match (AND): users, apps, clientAppTypes, platforms, locations, signInRisk, userRisk
//   - If any condition does not match -> ResultNotApplicable
//   - If all match and grantControls contains "block" -> ResultBlock
//   - Otherwise -> ResultGrant with controls list and operator
func EvaluatePolicy(slug string, policy map[string]interface{}, ctx *SignInContext) PolicyDecision {
	decision := PolicyDecision{PolicySlug: slug}

	// Check policy state
	state, _ := policy["state"].(string)
	if state == "disabled" {
		decision.Result = ResultNotApplicable
		return decision
	}

	// Both "enabled" and "enabledForReportingButNotEnforced" are evaluated
	conditions, _ := policy["conditions"].(map[string]interface{})
	if conditions == nil {
		conditions = map[string]interface{}{}
	}

	// All conditions must match (AND logic)
	if !matchUsers(conditions, ctx) {
		decision.Result = ResultNotApplicable
		return decision
	}
	if !matchApplications(conditions, ctx) {
		decision.Result = ResultNotApplicable
		return decision
	}
	if !matchClientAppTypes(conditions, ctx) {
		decision.Result = ResultNotApplicable
		return decision
	}
	if !matchPlatforms(conditions, ctx) {
		decision.Result = ResultNotApplicable
		return decision
	}
	if !matchLocations(conditions, ctx) {
		decision.Result = ResultNotApplicable
		return decision
	}
	if !matchSignInRiskLevels(conditions, ctx) {
		decision.Result = ResultNotApplicable
		return decision
	}
	if !matchUserRiskLevels(conditions, ctx) {
		decision.Result = ResultNotApplicable
		return decision
	}

	// All conditions matched -- extract grant controls
	grantControls, _ := policy["grantControls"].(map[string]interface{})
	if grantControls != nil {
		controls := getStringSlice(grantControls, "builtInControls")
		decision.GrantControls = controls
		decision.Operator, _ = grantControls["operator"].(string)

		// Check for block
		for _, c := range controls {
			if c == "block" {
				decision.Result = ResultBlock
				decision.SessionControls = extractSessionControls(policy)
				return decision
			}
		}
	}

	// Grant (with or without controls)
	decision.Result = ResultGrant
	decision.SessionControls = extractSessionControls(policy)
	return decision
}

// EvaluateAll evaluates all policies against a sign-in context and combines results.
// Block wins: if any policy blocks, the combined result is block.
// Grant controls from all matching grant policies are collected.
// NotApplicable policies are skipped.
func EvaluateAll(policies []PolicyWithSlug, ctx *SignInContext) CombinedDecision {
	combined := CombinedDecision{
		Result: ResultNotApplicable,
	}

	hasBlock := false
	var allControls []string
	var allSessionControls map[string]interface{}
	var matchingSlugs []string

	for _, p := range policies {
		decision := EvaluatePolicy(p.Slug, p.Data, ctx)
		if decision.Result == ResultNotApplicable {
			continue
		}

		matchingSlugs = append(matchingSlugs, p.Slug)

		if decision.Result == ResultBlock {
			hasBlock = true
		}

		if decision.GrantControls != nil {
			allControls = append(allControls, decision.GrantControls...)
		}

		// Merge session controls
		if decision.SessionControls != nil {
			if allSessionControls == nil {
				allSessionControls = make(map[string]interface{})
			}
			for k, v := range decision.SessionControls {
				allSessionControls[k] = v
			}
		}
	}

	combined.MatchingPolicies = matchingSlugs

	if len(matchingSlugs) == 0 {
		return combined
	}

	if hasBlock {
		combined.Result = ResultBlock
	} else {
		combined.Result = ResultGrant
	}

	combined.GrantControls = allControls
	combined.SessionControls = allSessionControls

	return combined
}

// extractSessionControls extracts the sessionControls block from a policy if present.
func extractSessionControls(policy map[string]interface{}) map[string]interface{} {
	sc, ok := policy["sessionControls"].(map[string]interface{})
	if !ok {
		return nil
	}
	return sc
}
