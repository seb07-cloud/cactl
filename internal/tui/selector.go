// Package tui provides interactive terminal UI components using charmbracelet/huh.
package tui

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/seb07-cloud/cactl/internal/state"
)

// SelectPolicy presents an arrow-key selector for tracked policy slugs.
// Returns "" when the user selects "Quit".
func SelectPolicy(slugs []string) (string, error) {
	var selected string
	options := make([]huh.Option[string], 0, len(slugs)+1)
	options = append(options, huh.NewOption("Quit", ""))
	for _, s := range slugs {
		options = append(options, huh.NewOption(s, s))
	}
	err := huh.NewSelect[string]().
		Title("Select a policy").
		Options(options...).
		Height(len(options)+2). // +2 for title and padding
		Value(&selected).
		Run()
	return selected, err
}

// SelectVersion presents an arrow-key selector for version tags with diff summaries.
// Each option label shows "version  date  summary".
// Returns "" when the user selects "Back to policies".
func SelectVersion(tags []state.VersionTag, summaries []string) (string, error) {
	var selected string
	options := make([]huh.Option[string], 0, len(tags)+1)
	options = append(options, huh.NewOption("← Back to policies", ""))
	for i, t := range tags {
		date := t.Timestamp
		if len(date) >= 10 {
			date = date[:10]
		}
		summary := ""
		if i < len(summaries) {
			summary = summaries[i]
		}
		label := fmt.Sprintf("%-8s  %s  %s", t.Version, date, summary)
		options = append(options, huh.NewOption(label, t.Version))
	}
	err := huh.NewSelect[string]().
		Title("Select a version to inspect").
		Options(options...).
		Height(len(options)+2). // +2 for title and padding
		Value(&selected).
		Run()
	return selected, err
}

// SelectAction presents a selector for actions after viewing a diff.
// Returns one of: "restore", "back", "quit".
func SelectAction() (string, error) {
	var selected string
	options := []huh.Option[string]{
		huh.NewOption("Restore this version", "restore"),
		huh.NewOption("Back to versions", "back"),
		huh.NewOption("Quit", "quit"),
	}
	err := huh.NewSelect[string]().
		Title("What would you like to do?").
		Options(options...).
		Value(&selected).
		Run()
	return selected, err
}

// ConfirmRestore asks for confirmation before restoring a policy version.
func ConfirmRestore(slug, version string) (bool, error) {
	var confirmed bool
	err := huh.NewConfirm().
		Title(fmt.Sprintf("Restore %s to %s?", slug, version)).
		Affirmative("Yes").
		Negative("No").
		Value(&confirmed).
		Run()
	return confirmed, err
}

// ConfirmOverwrite warns about uncommitted changes and asks for confirmation.
func ConfirmOverwrite(filePath string) (bool, error) {
	var confirmed bool
	err := huh.NewConfirm().
		Title(fmt.Sprintf("Overwrite %s with uncommitted changes?", filePath)).
		Description("The file has uncommitted local changes that will be lost.").
		Affirmative("Yes").
		Negative("No").
		Value(&confirmed).
		Run()
	return confirmed, err
}
