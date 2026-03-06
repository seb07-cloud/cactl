package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/seb07-cloud/cactl/internal/reconcile"
	"github.com/seb07-cloud/cactl/internal/resolve"
	"github.com/seb07-cloud/cactl/internal/validate"
	"github.com/seb07-cloud/cactl/pkg/types"
)

// Additional ANSI codes for diff output.
const (
	colorCyan    = "\033[36m"
	colorMagenta = "\033[35m"
	colorBold    = "\033[1m"
	colorDim     = "\033[2m"
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

// actionVerb returns a human-readable description for each action type.
func actionVerb(action reconcile.ActionType) string {
	switch action {
	case reconcile.ActionCreate:
		return "create"
	case reconcile.ActionUpdate:
		return "update"
	case reconcile.ActionRecreate:
		return "recreate"
	case reconcile.ActionUntracked:
		return "untracked"
	default:
		return ""
	}
}

// RenderPlan renders a terraform-style plan output to the given writer.
func RenderPlan(w io.Writer, actions []reconcile.PolicyAction, validations []validate.ValidationResult, resolver *resolve.Resolver, useColor bool) {
	actionable := 0
	for _, a := range actions {
		if a.Action != reconcile.ActionNoop {
			actionable++
		}
	}

	if actionable == 0 {
		fmt.Fprintln(w, "No changes. Infrastructure is up-to-date.")
		return
	}

	// Header
	c := colorFunc(useColor)
	fmt.Fprintf(w, "\n%s\n", c(colorBold, "cactl will perform the following actions:"))
	fmt.Fprintln(w, "")

	for _, a := range actions {
		if a.Action == reconcile.ActionNoop {
			continue
		}
		renderAction(w, a, resolver, useColor)
	}

	// Validation warnings/errors
	if len(validations) > 0 {
		fmt.Fprintln(w, "")
		fmt.Fprintf(w, "%s\n", c(colorBold, "Validations:"))
		for _, v := range validations {
			prefix := "WARNING:"
			clr := colorYellow
			if v.Severity == validate.SeverityError {
				prefix = "ERROR:"
				clr = colorRed
			}
			fmt.Fprintf(w, "  %s %s %s: %s\n",
				c(clr, prefix),
				c(colorDim, "["+v.Rule+"]"),
				v.Policy,
				v.Message,
			)
		}
	}

	// Summary
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

	fmt.Fprintln(w, "")
	parts := []string{}
	if create > 0 {
		parts = append(parts, c(colorGreen, fmt.Sprintf("%d to create", create)))
	}
	if update > 0 {
		parts = append(parts, c(colorYellow, fmt.Sprintf("%d to update", update)))
	}
	if recreate > 0 {
		parts = append(parts, c(colorRed, fmt.Sprintf("%d to recreate", recreate)))
	}
	if untracked > 0 {
		parts = append(parts, c(colorCyan, fmt.Sprintf("%d untracked", untracked)))
	}
	fmt.Fprintf(w, "Plan: %s\n", strings.Join(parts, ", "))
}

// renderAction renders a single policy action with its details.
func renderAction(w io.Writer, a reconcile.PolicyAction, resolver *resolve.Resolver, useColor bool) {
	c := colorFunc(useColor)
	s, clr := sigil(a.Action)

	// Header line: sigil + slug + metadata
	header := fmt.Sprintf("%s %s", c(clr+colorBold, s), c(colorBold, a.Slug))

	switch a.Action {
	case reconcile.ActionCreate:
		header += c(colorDim, " (new)")
	case reconcile.ActionUpdate:
		if a.VersionFrom != "" && a.VersionTo != "" {
			header += c(colorDim, fmt.Sprintf(" %s -> %s", a.VersionFrom, a.VersionTo))
			if a.BumpLevel != "" {
				header += c(clr, fmt.Sprintf(" [%s]", a.BumpLevel))
			}
		}
	case reconcile.ActionRecreate:
		header += c(colorRed, " (deleted from tenant, will recreate)")
	case reconcile.ActionUntracked:
		header += c(colorDim, " (not managed by cactl)")
	}

	fmt.Fprintln(w, header)

	// Field-level diffs
	if len(a.Diff) > 0 {
		for _, d := range a.Diff {
			renderFieldDiff(w, d, resolver, useColor)
		}
	}

	// Major bump warning
	if a.BumpLevel == "MAJOR" {
		fmt.Fprintf(w, "      %s\n", c(colorYellow, "WARNING: MAJOR version bump -- review scope change carefully"))
	}

	fmt.Fprintln(w, "")
}

// renderFieldDiff prints a single field diff with clear old/new labeling.
func renderFieldDiff(w io.Writer, d reconcile.FieldDiff, resolver *resolve.Resolver, useColor bool) {
	c := colorFunc(useColor)
	indent := "      "

	switch d.Type {
	case reconcile.DiffAdded:
		newVal := formatValue(d.NewValue, resolver)
		fmt.Fprintf(w, "%s%s %s = %s\n",
			indent,
			c(colorGreen, "+"),
			c(colorDim, d.Path),
			c(colorGreen, newVal),
		)

	case reconcile.DiffRemoved:
		oldVal := formatValue(d.OldValue, resolver)
		fmt.Fprintf(w, "%s%s %s = %s\n",
			indent,
			c(colorRed, "-"),
			c(colorDim, d.Path),
			c(colorRed, oldVal),
		)

	case reconcile.DiffChanged:
		oldVal := formatValue(d.OldValue, resolver)
		newVal := formatValue(d.NewValue, resolver)
		fmt.Fprintf(w, "%s%s %s\n", indent, c(colorYellow, "~"), c(colorDim, d.Path))
		fmt.Fprintf(w, "%s  %s %s\n", indent, c(colorRed, "-"), oldVal)
		fmt.Fprintf(w, "%s  %s %s\n", indent, c(colorGreen, "+"), newVal)
	}
}

// colorFunc returns a function that wraps text in ANSI codes if color is enabled.
func colorFunc(useColor bool) func(string, string) string {
	if useColor {
		return func(code, text string) string {
			return code + text + colorReset
		}
	}
	return func(_, text string) string {
		return text
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
		if len(v) == 0 {
			return "[]"
		}
		items := make([]string, 0, len(v))
		for _, item := range v {
			items = append(items, formatValue(item, resolver))
		}
		if len(items) <= 3 {
			return "[" + strings.Join(items, ", ") + "]"
		}
		// Multi-line for long lists
		lines := make([]string, 0, len(items)+2)
		lines = append(lines, "[")
		for _, item := range items {
			lines = append(lines, "          "+item+",")
		}
		lines = append(lines, "        ]")
		return strings.Join(lines, "\n")
	case map[string]interface{}:
		b, err := json.MarshalIndent(v, "        ", "  ")
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// RenderFieldDiffs renders a list of field diffs to the writer using the standard
// sigil format (+, -, ~). This is the public entry point used by the TUI restore
// wizard to display historical diffs without needing a resolver.
func RenderFieldDiffs(w io.Writer, diffs []reconcile.FieldDiff, useColor bool) {
	for _, d := range diffs {
		renderFieldDiff(w, d, nil, useColor)
	}
}

// DiffSummary returns a concise summary string for a set of field diffs.
// Example: "3 fields changed: state, displayName, conditions"
func DiffSummary(diffs []reconcile.FieldDiff) string {
	if len(diffs) == 0 {
		return "no changes"
	}

	// Collect unique top-level path segments
	seen := make(map[string]struct{})
	var paths []string
	for _, d := range diffs {
		top := d.Path
		if idx := strings.Index(d.Path, "."); idx >= 0 {
			top = d.Path[:idx]
		}
		if _, ok := seen[top]; !ok {
			seen[top] = struct{}{}
			paths = append(paths, top)
		}
	}

	display := paths
	suffix := ""
	if len(display) > 3 {
		display = display[:3]
		suffix = ", ..."
	}

	return fmt.Sprintf("%d fields changed: %s%s", len(diffs), strings.Join(display, ", "), suffix)
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
