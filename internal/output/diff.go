package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/seb07-cloud/cactl/internal/reconcile"
	"github.com/seb07-cloud/cactl/internal/resolve"
	"github.com/seb07-cloud/cactl/internal/validate"
	"github.com/seb07-cloud/cactl/pkg/types"
)

// Additional ANSI color codes for diff output.
const (
	colorCyan    = "\033[36m"
	colorMagenta = "\033[35m"
)

// sigil returns the sigil character and color for an action type.
func sigil(action reconcile.ActionType) (string, string) {
	switch action {
	case reconcile.ActionCreate:
		return "+", colorGreen
	case reconcile.ActionUpdate:
		return "~", colorYellow
	case reconcile.ActionRecreate:
		return "-/+", colorRed
	case reconcile.ActionUntracked:
		return "?", colorCyan
	default:
		return " ", colorReset
	}
}

// RenderPlan renders a terraform-style plan output to the given writer.
// Non-noop actions are displayed with sigils, field-level diffs, version
// bump information, validation warnings/errors, and a summary line.
func RenderPlan(w io.Writer, actions []reconcile.PolicyAction, validations []validate.ValidationResult, resolver *resolve.Resolver, useColor bool) {
	actionableCount := 0

	for _, a := range actions {
		if a.Action == reconcile.ActionNoop {
			continue
		}
		actionableCount++

		s, c := sigil(a.Action)
		if useColor {
			fmt.Fprintf(w, "%s%s%s %s", c, s, colorReset, a.Slug)
		} else {
			fmt.Fprintf(w, "%s %s", s, a.Slug)
		}

		// Action-specific details
		switch a.Action {
		case reconcile.ActionCreate:
			fmt.Fprintf(w, " (new policy, initial version 1.0.0)\n")

		case reconcile.ActionUpdate:
			if a.VersionFrom != "" && a.VersionTo != "" {
				fmt.Fprintf(w, " (%s -> %s, %s)\n", a.VersionFrom, a.VersionTo, a.BumpLevel)
			} else {
				fmt.Fprint(w, "\n")
			}
			// Print field-level diffs
			for _, d := range a.Diff {
				renderFieldDiff(w, d, resolver, useColor)
			}

		case reconcile.ActionRecreate:
			if useColor {
				fmt.Fprintf(w, " %sWARNING: policy deleted from tenant, will be recreated%s\n", colorRed, colorReset)
			} else {
				fmt.Fprintf(w, " WARNING: policy deleted from tenant, will be recreated\n")
			}

		case reconcile.ActionUntracked:
			fmt.Fprintf(w, " (untracked: exists in tenant, not in backend)\n")
		}

		// SEMV-06: Major bump warning
		if a.BumpLevel == "MAJOR" {
			if useColor {
				fmt.Fprintf(w, "  %sWARNING: MAJOR version bump -- review scope change carefully%s\n", colorYellow, colorReset)
			} else {
				fmt.Fprintf(w, "  WARNING: MAJOR version bump -- review scope change carefully\n")
			}
		}
	}

	// Validation warnings/errors
	if len(validations) > 0 {
		fmt.Fprint(w, "\n")
		for _, v := range validations {
			prefix := "WARNING"
			c := colorYellow
			if v.Severity == validate.SeverityError {
				prefix = "ERROR"
				c = colorRed
			}
			if useColor {
				fmt.Fprintf(w, "%s%s%s [%s] %s: %s\n", c, prefix, colorReset, v.Rule, v.Policy, v.Message)
			} else {
				fmt.Fprintf(w, "%s [%s] %s: %s\n", prefix, v.Rule, v.Policy, v.Message)
			}
		}
	}

	// Summary line
	var create, update, recreate, untracked int
	for _, a := range actions {
		switch a.Action {
		case reconcile.ActionCreate:
			create++
		case reconcile.ActionUpdate:
			update++
		case reconcile.ActionRecreate:
			recreate++
		case reconcile.ActionUntracked:
			untracked++
		}
	}

	if actionableCount > 0 {
		fmt.Fprint(w, "\n")
	}
	fmt.Fprintf(w, "Plan: %d to create, %d to update, %d to recreate, %d untracked.\n",
		create, update, recreate, untracked)
}

// renderFieldDiff prints a single field diff with sigils and resolved display names.
func renderFieldDiff(w io.Writer, d reconcile.FieldDiff, resolver *resolve.Resolver, useColor bool) {
	switch d.Type {
	case reconcile.DiffAdded:
		newVal := formatValue(d.NewValue, resolver)
		if useColor {
			fmt.Fprintf(w, "    %s+ %s:%s %s\n", colorGreen, d.Path, colorReset, newVal)
		} else {
			fmt.Fprintf(w, "    + %s: %s\n", d.Path, newVal)
		}

	case reconcile.DiffRemoved:
		oldVal := formatValue(d.OldValue, resolver)
		if useColor {
			fmt.Fprintf(w, "    %s- %s:%s %s\n", colorRed, d.Path, colorReset, oldVal)
		} else {
			fmt.Fprintf(w, "    - %s: %s\n", d.Path, oldVal)
		}

	case reconcile.DiffChanged:
		oldVal := formatValue(d.OldValue, resolver)
		newVal := formatValue(d.NewValue, resolver)
		if useColor {
			fmt.Fprintf(w, "    %s~ %s:%s %s -> %s\n", colorYellow, d.Path, colorReset, oldVal, newVal)
		} else {
			fmt.Fprintf(w, "    ~ %s: %s -> %s\n", d.Path, oldVal, newVal)
		}
	}
}

// formatValue converts a diff value to a display string, resolving GUIDs if a resolver is available.
func formatValue(val interface{}, resolver *resolve.Resolver) string {
	if val == nil {
		return "(none)"
	}

	switch v := val.(type) {
	case string:
		if resolver != nil {
			name := resolver.DisplayName(v)
			if name != v {
				return fmt.Sprintf("%q (%s)", v, name)
			}
		}
		return fmt.Sprintf("%q", v)
	case []interface{}:
		items := make([]string, 0, len(v))
		for _, item := range v {
			items = append(items, formatValue(item, resolver))
		}
		return fmt.Sprintf("[%s]", joinItems(items))
	default:
		return fmt.Sprintf("%v", v)
	}
}

// joinItems joins formatted items with ", ".
func joinItems(items []string) string {
	result := ""
	for i, item := range items {
		if i > 0 {
			result += ", "
		}
		result += item
	}
	return result
}

// RenderPlanJSON renders the plan output as JSON to the given writer.
func RenderPlanJSON(w io.Writer, actions []reconcile.PolicyAction, validations []validate.ValidationResult, resolver *resolve.Resolver) error {
	out := types.PlanOutput{
		SchemaVersion: 1,
		Actions:       make([]types.ActionOutput, 0, len(actions)),
	}

	for _, a := range actions {
		if a.Action == reconcile.ActionNoop {
			continue
		}

		ao := types.ActionOutput{
			Slug:        a.Slug,
			DisplayName: a.DisplayName,
			Action:      a.Action.String(),
			VersionFrom: a.VersionFrom,
			VersionTo:   a.VersionTo,
			BumpLevel:   a.BumpLevel,
		}

		// Convert field diffs
		for _, d := range a.Diff {
			diffType := "changed"
			switch d.Type {
			case reconcile.DiffAdded:
				diffType = "added"
			case reconcile.DiffRemoved:
				diffType = "removed"
			}
			ao.Diffs = append(ao.Diffs, types.DiffOutput{
				Path:     d.Path,
				Type:     diffType,
				OldValue: d.OldValue,
				NewValue: d.NewValue,
			})
		}

		ao.Warnings = a.Warnings

		out.Actions = append(out.Actions, ao)
	}

	// Build summary
	for _, a := range actions {
		switch a.Action {
		case reconcile.ActionCreate:
			out.Summary.Create++
		case reconcile.ActionUpdate:
			out.Summary.Update++
		case reconcile.ActionRecreate:
			out.Summary.Recreate++
		case reconcile.ActionUntracked:
			out.Summary.Untracked++
		case reconcile.ActionNoop:
			out.Summary.Noop++
		}
	}

	// Collect validation warnings into top-level Warnings
	for _, v := range validations {
		out.Warnings = append(out.Warnings, fmt.Sprintf("[%s] %s: %s: %s", v.Severity, v.Rule, v.Policy, v.Message))
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling plan JSON: %w", err)
	}

	_, err = w.Write(data)
	if err != nil {
		return fmt.Errorf("writing plan JSON: %w", err)
	}
	_, err = w.Write([]byte("\n"))
	return err
}
