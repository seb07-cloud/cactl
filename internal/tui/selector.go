// Package tui provides interactive terminal UI components using charmbracelet/huh.
package tui

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/seb07-cloud/cactl/internal/state"
)

// SelectPolicy presents an arrow-key selector for tracked policy slugs.
func SelectPolicy(slugs []string) (string, error) {
	var selected string
	options := make([]huh.Option[string], len(slugs))
	for i, s := range slugs {
		options[i] = huh.NewOption(s, s)
	}
	err := huh.NewSelect[string]().
		Title("Select a policy").
		Options(options...).
		Value(&selected).
		Run()
	return selected, err
}

// SelectVersion presents an arrow-key selector for version tags with diff summaries.
// Each option label shows "version  date  summary".
func SelectVersion(tags []state.VersionTag, summaries []string) (string, error) {
	var selected string
	options := make([]huh.Option[string], len(tags))
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
		options[i] = huh.NewOption(label, t.Version)
	}
	err := huh.NewSelect[string]().
		Title("Select a version to inspect").
		Options(options...).
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
