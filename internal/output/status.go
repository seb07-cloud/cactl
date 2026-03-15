package output

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/seb07-cloud/cactl/internal/state"
	"github.com/seb07-cloud/cactl/pkg/types"
)

// RenderStatus writes an aligned status table to w.
// If useColor is true, sync status labels are colored.
func RenderStatus(w io.Writer, entries []types.PolicyStatus, useColor bool) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "POLICY\tVERSION\tLAST DEPLOYED\tDEPLOYED BY\tSYNC")

	for _, e := range entries {
		syncLabel := formatSyncStatus(e.SyncStatus, useColor)
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", e.Slug, e.Version, e.LastDeployed, e.DeployedBy, syncLabel)
	}
	_ = tw.Flush()

	// Summary line
	summary := BuildSummary(entries)
	fmt.Fprintf(w, "\nStatus: %d total, %d in-sync, %d drifted, %d missing, %d unknown\n",
		summary.Total, summary.InSync, summary.Drifted, summary.Missing, summary.Unknown)
}

// RenderStatusJSON writes the status output as indented JSON.
func RenderStatusJSON(w io.Writer, output types.StatusOutput) error {
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling status JSON: %w", err)
	}
	_, err = w.Write(data)
	if err != nil {
		return fmt.Errorf("writing status JSON: %w", err)
	}
	fmt.Fprintln(w)
	return nil
}

// RenderHistory writes a version history table for a single policy.
func RenderHistory(w io.Writer, slug string, tags []state.VersionTag, useColor bool) {
	fmt.Fprintf(w, "Version history for %s:\n\n", slug)
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "VERSION\tDEPLOYED\tMESSAGE")

	for _, t := range tags {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", t.Version, t.Timestamp, t.Message)
	}
	_ = tw.Flush()
}

// BuildSummary computes a StatusSummary from a slice of PolicyStatus entries.
func BuildSummary(entries []types.PolicyStatus) types.StatusSummary {
	var s types.StatusSummary
	s.Total = len(entries)
	for _, e := range entries {
		switch e.SyncStatus {
		case "in-sync":
			s.InSync++
		case "drifted":
			s.Drifted++
		case "missing":
			s.Missing++
		case "unknown":
			s.Unknown++
		}
	}
	return s
}

// formatSyncStatus returns the sync status string with optional ANSI color.
func formatSyncStatus(status string, useColor bool) string {
	if !useColor {
		return status
	}
	switch status {
	case "in-sync":
		return colorGreen + status + colorReset
	case "drifted":
		return colorYellow + status + colorReset
	case "missing":
		return colorRed + status + colorReset
	default:
		return status
	}
}
