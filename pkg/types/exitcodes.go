package types

import "fmt"

// Exit code constants for the cactl CLI.
const (
	ExitSuccess         = 0 // Success, no changes needed
	ExitChanges         = 1 // Changes or drift detected
	ExitFatalError      = 2 // Fatal error (auth failure, network error, etc.)
	ExitValidationError = 3 // Validation error (invalid config, schema violation)
)

// ExitError is a custom error type that carries an exit code.
// Commands return ExitError to signal a specific exit code to main.go.
type ExitError struct {
	Code    int
	Message string
	Err     error
}

// Error implements the error interface.
func (e *ExitError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap returns the wrapped error for errors.As/errors.Is compatibility.
func (e *ExitError) Unwrap() error {
	return e.Err
}
