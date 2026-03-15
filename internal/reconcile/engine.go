package reconcile

import (
	"sort"

	"github.com/seb07-cloud/cactl/internal/state"
)

// BackendPolicy represents a policy read from the Git backend.
type BackendPolicy struct {
	Data map[string]interface{}
}

// LivePolicy represents a policy read from the live tenant (Graph API).
type LivePolicy struct {
	NormalizedData map[string]interface{}
	Slug           string
}

// Reconcile compares backend state against live tenant state using the manifest
// for tracking, and returns a list of actions needed to bring live into sync.
// Actions are sorted by slug for deterministic output.
func Reconcile(backend map[string]BackendPolicy, live map[string]LivePolicy, manifest *state.Manifest) []PolicyAction {
	var actions []PolicyAction

	// Step 1: Process backend policies
	for slug, bp := range backend {
		entry, tracked := manifest.Policies[slug]

		if !tracked {
			// Not in manifest -> Create
			actions = append(actions, PolicyAction{
				Slug:        slug,
				Action:      ActionCreate,
				BackendJSON: bp.Data,
			})
			continue
		}

		// Tracked -- check if live object still exists
		livePolicy, liveExists := live[entry.LiveObjectID]
		if !liveExists {
			// Ghost: manifest tracks it but live doesn't have it -> Recreate
			actions = append(actions, PolicyAction{
				Slug:         slug,
				Action:       ActionRecreate,
				BackendJSON:  bp.Data,
				LiveObjectID: entry.LiveObjectID,
			})
			continue
		}

		// Both exist -- compute diff
		diffs := ComputeDiff(bp.Data, livePolicy.NormalizedData)
		if len(diffs) == 0 {
			// Noop: in sync, don't emit an action
			continue
		}

		// Update: diffs found
		actions = append(actions, PolicyAction{
			Slug:         slug,
			Action:       ActionUpdate,
			BackendJSON:  bp.Data,
			LiveJSON:     livePolicy.NormalizedData,
			LiveObjectID: entry.LiveObjectID,
			Diff:         diffs,
		})
	}

	// Step 2: Find untracked live policies
	// Build set of all tracked LiveObjectIDs
	trackedIDs := make(map[string]bool)
	for _, entry := range manifest.Policies {
		if entry.LiveObjectID != "" {
			trackedIDs[entry.LiveObjectID] = true
		}
	}

	for liveID, lp := range live {
		if !trackedIDs[liveID] {
			actions = append(actions, PolicyAction{
				Slug:         lp.Slug,
				Action:       ActionUntracked,
				LiveJSON:     lp.NormalizedData,
				LiveObjectID: liveID,
			})
		}
	}

	// Step 3: Detect duplicate live policies (same displayName, different IDs)
	actions = append(actions, DetectDuplicates(live)...)

	// Sort by slug for deterministic output
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].Slug < actions[j].Slug
	})

	if len(actions) == 0 {
		return nil
	}
	return actions
}

// DetectDuplicates groups live policies by displayName and emits an
// ActionDuplicate for each group with more than one member.
func DetectDuplicates(live map[string]LivePolicy) []PolicyAction {
	// Group by displayName
	type dupEntry struct {
		id          string
		displayName string
	}
	groups := make(map[string][]dupEntry)
	for id, lp := range live {
		name, _ := lp.NormalizedData["displayName"].(string)
		if name == "" {
			continue
		}
		groups[name] = append(groups[name], dupEntry{id: id, displayName: name})
	}

	var actions []PolicyAction
	for displayName, entries := range groups {
		if len(entries) <= 1 {
			continue
		}
		// Sort IDs for deterministic output
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].id < entries[j].id
		})
		ids := make([]string, len(entries))
		for i, e := range entries {
			ids[i] = e.id
		}
		actions = append(actions, PolicyAction{
			Slug:         displayName,
			Action:       ActionDuplicate,
			DisplayName:  displayName,
			DuplicateIDs: ids,
		})
	}
	return actions
}
