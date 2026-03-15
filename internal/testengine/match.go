package testengine

import "strings"

// matchUsers checks if the sign-in context user matches the policy's user conditions.
// Include logic: user matches if they appear in ANY include list (users OR groups OR roles).
// Exclude logic: user is excluded if they match ANY exclude list.
// Pattern: include first, then exclude overrides.
func matchUsers(conditions map[string]interface{}, ctx *SignInContext) bool {
	users := getNestedMap(conditions, "users")
	if users == nil {
		return true // No user conditions = matches all
	}

	includeUsers := getStringSlice(users, "includeUsers")
	includeGroups := getStringSlice(users, "includeGroups")
	includeRoles := getStringSlice(users, "includeRoles")

	// If all include lists are empty, no users are targeted
	if len(includeUsers) == 0 && len(includeGroups) == 0 && len(includeRoles) == 0 {
		return false
	}

	// Check inclusion (OR across lists: any match is sufficient)
	included := false

	// Check includeUsers
	for _, u := range includeUsers {
		if u == "All" {
			included = true
			break
		}
		if u == "GuestsOrExternalUsers" && ctx.User == "guest" {
			included = true
			break
		}
		if u == ctx.User {
			included = true
			break
		}
	}

	// Check includeGroups if not already included
	if !included {
		for _, g := range includeGroups {
			for _, cg := range ctx.Groups {
				if g == cg {
					included = true
					break
				}
			}
			if included {
				break
			}
		}
	}

	// Check includeRoles if not already included
	if !included {
		for _, r := range includeRoles {
			for _, cr := range ctx.Roles {
				if r == cr {
					included = true
					break
				}
			}
			if included {
				break
			}
		}
	}

	if !included {
		return false
	}

	// Check exclusions (any match = excluded)
	excludeUsers := getStringSlice(users, "excludeUsers")
	for _, u := range excludeUsers {
		if u == "GuestsOrExternalUsers" && ctx.User == "guest" {
			return false
		}
		if u == ctx.User {
			return false
		}
	}

	excludeGroups := getStringSlice(users, "excludeGroups")
	for _, g := range excludeGroups {
		for _, cg := range ctx.Groups {
			if g == cg {
				return false
			}
		}
	}

	excludeRoles := getStringSlice(users, "excludeRoles")
	for _, r := range excludeRoles {
		for _, cr := range ctx.Roles {
			if r == cr {
				return false
			}
		}
	}

	return true
}

// matchApplications checks if the sign-in context application matches the policy's application conditions.
func matchApplications(conditions map[string]interface{}, ctx *SignInContext) bool {
	apps := getNestedMap(conditions, "applications")
	if apps == nil {
		return true // No application conditions = matches all
	}

	includeApps := getStringSlice(apps, "includeApplications")
	excludeApps := getStringSlice(apps, "excludeApplications")

	return matchStringList(includeApps, excludeApps, []string{ctx.Application})
}

// matchClientAppTypes checks if the sign-in context client app type matches the policy's client app type conditions.
// If absent or empty, matches all client app types.
func matchClientAppTypes(conditions map[string]interface{}, ctx *SignInContext) bool {
	types := getStringSlice(conditions, "clientAppTypes")
	if len(types) == 0 {
		return true // No client app type filter = matches all
	}

	for _, t := range types {
		if t == "all" || t == ctx.ClientAppType {
			return true
		}
	}

	return false
}

// matchPlatforms checks if the sign-in context platform matches the policy's platform conditions.
// If no platforms block exists, matches all platforms.
func matchPlatforms(conditions map[string]interface{}, ctx *SignInContext) bool {
	platforms := getNestedMap(conditions, "platforms")
	if platforms == nil {
		return true // No platform conditions = matches all
	}

	// If context has no platform specified, match (unspecified = any)
	if ctx.Platform == "" {
		return true
	}

	includePlatforms := getStringSlice(platforms, "includePlatforms")
	excludePlatforms := getStringSlice(platforms, "excludePlatforms")

	return matchStringList(includePlatforms, excludePlatforms, []string{ctx.Platform})
}

// matchLocations checks if the sign-in context location matches the policy's location conditions.
// "All" matches all locations. "AllTrusted" matches if ctx.Location == "trusted".
func matchLocations(conditions map[string]interface{}, ctx *SignInContext) bool {
	locations := getNestedMap(conditions, "locations")
	if locations == nil {
		return true // No location conditions = matches all
	}

	includeLocations := getStringSlice(locations, "includeLocations")
	excludeLocations := getStringSlice(locations, "excludeLocations")

	// Build effective values for the context location
	values := locationValues(ctx.Location)

	return matchStringListWithKeywords(includeLocations, excludeLocations, values)
}

// matchSignInRiskLevels checks if the sign-in risk level matches the policy's filter.
// Empty/absent array = no filter (matches all).
func matchSignInRiskLevels(conditions map[string]interface{}, ctx *SignInContext) bool {
	levels := getStringSlice(conditions, "signInRiskLevels")
	if len(levels) == 0 {
		return true // No filter = matches all
	}

	for _, l := range levels {
		if l == ctx.SignInRiskLevel {
			return true
		}
	}

	return false
}

// matchUserRiskLevels checks if the user risk level matches the policy's filter.
// Empty/absent array = no filter (matches all).
func matchUserRiskLevels(conditions map[string]interface{}, ctx *SignInContext) bool {
	levels := getStringSlice(conditions, "userRiskLevels")
	if len(levels) == 0 {
		return true // No filter = matches all
	}

	for _, l := range levels {
		if l == ctx.UserRiskLevel {
			return true
		}
	}

	return false
}

// matchStringList implements the standard CA include/exclude pattern.
// Returns true if any value matches the include list and none match the exclude list.
// "All" in the include list matches everything.
func matchStringList(includeList, excludeList, values []string) bool {
	if len(includeList) == 0 {
		return false
	}

	// Check includes
	included := false
	for _, inc := range includeList {
		if inc == "All" || inc == "all" {
			included = true
			break
		}
		for _, v := range values {
			if inc == v {
				included = true
				break
			}
		}
		if included {
			break
		}
	}
	if !included {
		return false
	}

	// Check excludes
	if len(excludeList) > 0 {
		excludeSet := make(map[string]bool, len(excludeList))
		for _, ex := range excludeList {
			excludeSet[ex] = true
		}
		for _, v := range values {
			if excludeSet[v] {
				return false
			}
		}
	}

	return true
}

// matchStringListWithKeywords extends matchStringList with location-specific keywords.
// "AllTrusted" in include matches if values contain "trusted".
func matchStringListWithKeywords(includeList, excludeList, values []string) bool {
	if len(includeList) == 0 {
		return false
	}

	// Check includes
	included := false
	for _, inc := range includeList {
		if inc == "All" {
			included = true
			break
		}
		if inc == "AllTrusted" {
			for _, v := range values {
				if v == "trusted" {
					included = true
					break
				}
			}
			if included {
				break
			}
			continue
		}
		for _, v := range values {
			if inc == v {
				included = true
				break
			}
		}
		if included {
			break
		}
	}
	if !included {
		return false
	}

	// Check excludes
	if len(excludeList) > 0 {
		excludeSet := make(map[string]bool, len(excludeList))
		for _, ex := range excludeList {
			excludeSet[ex] = true
		}
		for _, v := range values {
			if excludeSet[v] {
				return false
			}
		}
	}

	return true
}

// locationValues returns the set of values to match for a given location context.
func locationValues(location string) []string {
	if location == "" {
		return nil
	}
	return []string{location}
}

// --- Helpers (copied from internal/validate to avoid import coupling) ---

// getNestedValue walks a dot-separated path in a nested map and returns the value.
func getNestedValue(data map[string]interface{}, dotPath string) interface{} {
	parts := splitPath(dotPath)
	var current interface{} = data

	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current, ok = m[part]
		if !ok {
			return nil
		}
	}

	return current
}

// getNestedMap walks a dot-separated path and returns the value as a map, or nil.
func getNestedMap(data map[string]interface{}, key string) map[string]interface{} {
	val, ok := data[key]
	if !ok {
		return nil
	}
	m, ok := val.(map[string]interface{})
	if !ok {
		return nil
	}
	return m
}

// getStringSlice gets a key from a map and converts []interface{} to []string.
func getStringSlice(data map[string]interface{}, key string) []string {
	val, ok := data[key]
	if !ok {
		return nil
	}

	slice, ok := val.([]interface{})
	if !ok {
		return nil
	}

	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}

	return result
}

// splitPath splits a dot-separated path into parts.
func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	return strings.Split(path, ".")
}
