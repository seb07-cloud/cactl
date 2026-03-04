package config

import (
	"fmt"

	"github.com/sebdah/cactl/pkg/types"
)

// validOutputFormats lists the accepted values for the --output flag.
var validOutputFormats = map[string]bool{
	"human": true,
	"json":  true,
}

// validLogLevels lists the accepted values for the --log-level flag.
var validLogLevels = map[string]bool{
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

// validAuthModes lists the accepted values for the --auth-mode flag.
var validAuthModes = map[string]bool{
	"az-cli":             true,
	"client-secret":      true,
	"client-certificate": true,
}

// Validate checks the Config values and returns an ExitError with code 3
// (validation error) if any value is invalid.
func Validate(cfg *types.Config) error {
	if cfg.Output != "" && !validOutputFormats[cfg.Output] {
		return &types.ExitError{
			Code:    types.ExitValidationError,
			Message: fmt.Sprintf("invalid output format %q: must be one of human, json", cfg.Output),
		}
	}

	if cfg.LogLevel != "" && !validLogLevels[cfg.LogLevel] {
		return &types.ExitError{
			Code:    types.ExitValidationError,
			Message: fmt.Sprintf("invalid log level %q: must be one of debug, info, warn, error", cfg.LogLevel),
		}
	}

	if cfg.Auth.Mode != "" && !validAuthModes[cfg.Auth.Mode] {
		return &types.ExitError{
			Code:    types.ExitValidationError,
			Message: fmt.Sprintf("invalid auth mode %q: must be one of az-cli, client-secret, client-certificate", cfg.Auth.Mode),
		}
	}

	return nil
}
