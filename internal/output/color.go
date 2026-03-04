package output

import (
	"os"

	"github.com/spf13/viper"
)

// ShouldUseColor determines whether ANSI color output should be enabled.
// It checks multiple signals in priority order:
//   - --no-color flag or CACTL_NO_COLOR env var
//   - NO_COLOR env var (https://no-color.org convention)
//   - --ci flag (CI mode disables color)
//   - Whether stdout is a terminal
func ShouldUseColor(v *viper.Viper) bool {
	// Explicit flag/env takes priority
	if v.GetBool("no-color") {
		return false
	}

	// Respect NO_COLOR convention (any non-empty value)
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// CI mode disables color by default
	if v.GetBool("ci") {
		return false
	}

	// Check if stdout is a terminal
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	if (fileInfo.Mode() & os.ModeCharDevice) == 0 {
		return false // piped output, no color
	}

	return true
}
