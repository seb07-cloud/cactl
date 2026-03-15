package output

import (
	"fmt"
	"strings"

	"github.com/seb07-cloud/cactl/internal/reconcile"
)

// ANSI color codes for terminal output.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
)

// HumanRenderer outputs human-readable messages with optional ANSI colors.
type HumanRenderer struct {
	useColor bool
}

// Success renders a success message with a green checkmark prefix.
func (r *HumanRenderer) Success(msg string) {
	if r.useColor {
		fmt.Printf("%sOK%s %s\n", colorGreen, colorReset, msg)
	} else {
		fmt.Printf("OK %s\n", msg)
	}
}

// Error renders an error message with a red cross prefix.
func (r *HumanRenderer) Error(msg string) {
	if r.useColor {
		fmt.Printf("%sERROR%s %s\n", colorRed, colorReset, msg)
	} else {
		fmt.Printf("ERROR %s\n", msg)
	}
}

// Info renders an informational message with a blue info prefix.
func (r *HumanRenderer) Info(msg string) {
	if r.useColor {
		fmt.Printf("%si%s %s\n", colorBlue, colorReset, msg)
	} else {
		fmt.Printf("INFO %s\n", msg)
	}
}

// Warn renders a warning message with a yellow warning prefix.
func (r *HumanRenderer) Warn(msg string) {
	if r.useColor {
		fmt.Printf("%s!%s %s\n", colorYellow, colorReset, msg)
	} else {
		fmt.Printf("WARN %s\n", msg)
	}
}

// Print renders a plain message with no prefix.
func (r *HumanRenderer) Print(msg string) {
	fmt.Println(msg)
}

// FormatApplied returns a colored "Applied: <sigil> <message>" string.
// The sigil color matches the plan output for each action type.
func FormatApplied(action reconcile.ActionType, message string, useColor bool) string {
	c := colorFunc(useColor)
	s, clr := sigil(action)
	return fmt.Sprintf("Applied: %s %s", c(clr, s), message)
}

// FormatApplySummary returns a colored apply summary line matching the plan style.
func FormatApplySummary(created, updated, recreated int, useColor bool) string {
	c := colorFunc(useColor)
	var parts []string
	if created > 0 {
		parts = append(parts, c(colorGreen, fmt.Sprintf("%d created", created)))
	}
	if updated > 0 {
		parts = append(parts, c(colorYellow, fmt.Sprintf("%d updated", updated)))
	}
	if recreated > 0 {
		parts = append(parts, c(colorRed, fmt.Sprintf("%d recreated", recreated)))
	}
	if len(parts) == 0 {
		return "Apply complete: no changes."
	}
	return fmt.Sprintf("Apply complete: %s.", strings.Join(parts, ", "))
}
