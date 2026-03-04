package output

// Renderer defines the interface for rendering CLI output.
// Implementations handle formatting differences between human-readable
// and structured JSON output.
type Renderer interface {
	// Success renders a success message.
	Success(msg string)
	// Error renders an error message.
	Error(msg string)
	// Info renders an informational message.
	Info(msg string)
	// Warn renders a warning message.
	Warn(msg string)
	// Print renders a plain message with no prefix or level.
	Print(msg string)
}

// NewRenderer creates a Renderer for the given format.
// If format is "json", returns a JSONRenderer.
// Otherwise returns a HumanRenderer with color support based on useColor.
func NewRenderer(format string, useColor bool) Renderer {
	if format == "json" {
		return &JSONRenderer{}
	}
	return &HumanRenderer{useColor: useColor}
}
