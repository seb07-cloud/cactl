package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/seb07-cloud/cactl/internal/reconcile"
	"github.com/seb07-cloud/cactl/internal/validate"
	"github.com/seb07-cloud/cactl/pkg/types"
)

func TestRenderPlan_CreateAction(t *testing.T) {
	actions := []reconcile.PolicyAction{
		{
			Slug:   "require-mfa",
			Action: reconcile.ActionCreate,
		},
	}

	var buf bytes.Buffer
	RenderPlan(&buf, actions, nil, nil, false)
	out := buf.String()

	if !strings.Contains(out, "+ require-mfa") {
		t.Errorf("expected '+ require-mfa' sigil, got:\n%s", out)
	}
	if !strings.Contains(out, "(new)") {
		t.Errorf("expected create indicator, got:\n%s", out)
	}
}

func TestRenderPlan_UpdateAction(t *testing.T) {
	actions := []reconcile.PolicyAction{
		{
			Slug:        "require-mfa",
			Action:      reconcile.ActionUpdate,
			VersionFrom: "1.0.0",
			VersionTo:   "1.1.0",
			BumpLevel:   "MINOR",
			Diff: []reconcile.FieldDiff{
				{
					Path:     "state",
					Type:     reconcile.DiffChanged,
					OldValue: "disabled",
					NewValue: "enabled",
				},
			},
		},
	}

	var buf bytes.Buffer
	RenderPlan(&buf, actions, nil, nil, false)
	out := buf.String()

	if !strings.Contains(out, "~ require-mfa") {
		t.Errorf("expected '~ require-mfa' sigil, got:\n%s", out)
	}
	if !strings.Contains(out, "1.1.0") {
		t.Errorf("expected version info, got:\n%s", out)
	}
	if !strings.Contains(out, "state") {
		t.Errorf("expected field diff for state, got:\n%s", out)
	}
	if !strings.Contains(out, "disabled") || !strings.Contains(out, "enabled") {
		t.Errorf("expected old/new values, got:\n%s", out)
	}
}

func TestRenderPlan_SummaryLine(t *testing.T) {
	actions := []reconcile.PolicyAction{
		{Slug: "a", Action: reconcile.ActionCreate},
		{Slug: "b", Action: reconcile.ActionUpdate},
		{Slug: "c", Action: reconcile.ActionRecreate},
		{Slug: "d", Action: reconcile.ActionUntracked},
	}

	var buf bytes.Buffer
	RenderPlan(&buf, actions, nil, nil, false)
	out := buf.String()

	if !strings.Contains(out, "1 to create") {
		t.Errorf("expected create count in summary, got:\n%s", out)
	}
	if !strings.Contains(out, "1 to update") {
		t.Errorf("expected update count in summary, got:\n%s", out)
	}
	if !strings.Contains(out, "1 to recreate") {
		t.Errorf("expected recreate count in summary, got:\n%s", out)
	}
	if !strings.Contains(out, "1 untracked") {
		t.Errorf("expected untracked count in summary, got:\n%s", out)
	}
}

func TestRenderPlanJSON_ValidOutput(t *testing.T) {
	actions := []reconcile.PolicyAction{
		{
			Slug:   "require-mfa",
			Action: reconcile.ActionCreate,
		},
		{
			Slug:   "block-legacy",
			Action: reconcile.ActionUpdate,
			Diff: []reconcile.FieldDiff{
				{Path: "state", Type: reconcile.DiffChanged, OldValue: "disabled", NewValue: "enabled"},
			},
		},
	}

	var buf bytes.Buffer
	err := RenderPlanJSON(&buf, actions, nil, nil)
	if err != nil {
		t.Fatalf("RenderPlanJSON returned error: %v", err)
	}

	var out types.PlanOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON output: %v\nraw: %s", err, buf.String())
	}

	if out.SchemaVersion != 1 {
		t.Errorf("expected schema_version=1, got %d", out.SchemaVersion)
	}
	if len(out.Actions) != 2 {
		t.Errorf("expected 2 actions, got %d", len(out.Actions))
	}
	if out.Summary.Create != 1 {
		t.Errorf("expected summary create=1, got %d", out.Summary.Create)
	}
	if out.Summary.Update != 1 {
		t.Errorf("expected summary update=1, got %d", out.Summary.Update)
	}
}

func TestRenderPlan_MajorBumpWarning(t *testing.T) {
	actions := []reconcile.PolicyAction{
		{
			Slug:      "require-mfa",
			Action:    reconcile.ActionUpdate,
			BumpLevel: "MAJOR",
			Diff: []reconcile.FieldDiff{
				{Path: "conditions.users.includeUsers", Type: reconcile.DiffChanged, OldValue: "old", NewValue: "new"},
			},
		},
	}

	var buf bytes.Buffer
	RenderPlan(&buf, actions, nil, nil, false)
	out := buf.String()

	if !strings.Contains(out, "MAJOR version bump") {
		t.Errorf("expected MAJOR bump warning, got:\n%s", out)
	}
}

func TestRenderPlan_NoColor(t *testing.T) {
	actions := []reconcile.PolicyAction{
		{
			Slug:   "require-mfa",
			Action: reconcile.ActionCreate,
		},
	}

	var buf bytes.Buffer
	RenderPlan(&buf, actions, nil, nil, false)
	out := buf.String()

	if strings.Contains(out, "\033[") {
		t.Errorf("no-color mode should not contain ANSI codes, got:\n%s", out)
	}
}

func TestRenderPlan_WithColor(t *testing.T) {
	actions := []reconcile.PolicyAction{
		{
			Slug:   "require-mfa",
			Action: reconcile.ActionCreate,
		},
	}

	var buf bytes.Buffer
	RenderPlan(&buf, actions, nil, nil, true)
	out := buf.String()

	if !strings.Contains(out, "\033[") {
		t.Errorf("color mode should contain ANSI codes, got:\n%s", out)
	}
}

func TestRenderPlan_ValidationWarnings(t *testing.T) {
	actions := []reconcile.PolicyAction{
		{Slug: "require-mfa", Action: reconcile.ActionCreate},
	}
	validations := []validate.ValidationResult{
		{Rule: "break-glass", Severity: validate.SeverityWarning, Policy: "require-mfa", Message: "account not excluded"},
	}

	var buf bytes.Buffer
	RenderPlan(&buf, actions, validations, nil, false)
	out := buf.String()

	if !strings.Contains(out, "break-glass") {
		t.Errorf("expected validation rule name, got:\n%s", out)
	}
	if !strings.Contains(out, "account not excluded") {
		t.Errorf("expected validation message, got:\n%s", out)
	}
}

func TestRenderPlanJSON_WithValidations(t *testing.T) {
	actions := []reconcile.PolicyAction{
		{Slug: "require-mfa", Action: reconcile.ActionCreate},
	}
	validations := []validate.ValidationResult{
		{Rule: "break-glass", Severity: validate.SeverityWarning, Policy: "require-mfa", Message: "account not excluded"},
	}

	var buf bytes.Buffer
	err := RenderPlanJSON(&buf, actions, validations, nil)
	if err != nil {
		t.Fatalf("RenderPlanJSON returned error: %v", err)
	}

	var out types.PlanOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if len(out.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(out.Warnings))
	}
}

func TestRenderPlan_EmptyActions(t *testing.T) {
	var buf bytes.Buffer
	RenderPlan(&buf, nil, nil, nil, false)
	out := buf.String()

	if !strings.Contains(out, "No changes") {
		t.Errorf("expected no-changes message, got:\n%s", out)
	}
}

func TestRenderPlan_DiffChanged_ShowsOldAndNew(t *testing.T) {
	actions := []reconcile.PolicyAction{
		{
			Slug:   "test-policy",
			Action: reconcile.ActionUpdate,
			Diff: []reconcile.FieldDiff{
				{
					Path:     "conditions.applications.includeApplications",
					Type:     reconcile.DiffChanged,
					OldValue: []interface{}{"All"},
					NewValue: []interface{}{},
				},
			},
		},
	}

	var buf bytes.Buffer
	RenderPlan(&buf, actions, nil, nil, false)
	out := buf.String()

	// Changed diffs should show old value with - and new value with +
	if !strings.Contains(out, `- ["All"]`) {
		t.Errorf("expected old value with - prefix, got:\n%s", out)
	}
	if !strings.Contains(out, "+ []") {
		t.Errorf("expected new value with + prefix, got:\n%s", out)
	}
}
