package config

import (
	"errors"
	"testing"

	"github.com/seb07-cloud/cactl/pkg/types"
)

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &types.Config{
		Output:   "json",
		LogLevel: "info",
		Auth:     types.AuthConfig{Mode: "az-cli"},
	}
	if err := Validate(cfg); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

func TestValidate_EmptyFieldsAreValid(t *testing.T) {
	cfg := &types.Config{}
	if err := Validate(cfg); err != nil {
		t.Errorf("Validate() = %v, want nil for empty config", err)
	}
}

func TestValidate_InvalidOutput(t *testing.T) {
	cfg := &types.Config{Output: "xml"}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() = nil, want error for invalid output format")
	}
	var exitErr *types.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error type = %T, want *types.ExitError", err)
	}
	if exitErr.Code != types.ExitValidationError {
		t.Errorf("exit code = %d, want %d", exitErr.Code, types.ExitValidationError)
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	cfg := &types.Config{LogLevel: "verbose"}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() = nil, want error for invalid log level")
	}
	var exitErr *types.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error type = %T, want *types.ExitError", err)
	}
	if exitErr.Code != types.ExitValidationError {
		t.Errorf("exit code = %d, want %d", exitErr.Code, types.ExitValidationError)
	}
}

func TestValidate_InvalidAuthMode(t *testing.T) {
	cfg := &types.Config{Auth: types.AuthConfig{Mode: "oauth"}}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() = nil, want error for invalid auth mode")
	}
	var exitErr *types.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error type = %T, want *types.ExitError", err)
	}
	if exitErr.Code != types.ExitValidationError {
		t.Errorf("exit code = %d, want %d", exitErr.Code, types.ExitValidationError)
	}
}

func TestValidate_AllValidCombinations(t *testing.T) {
	for _, output := range []string{"", "human", "json"} {
		for _, level := range []string{"", "debug", "info", "warn", "error"} {
			for _, mode := range []string{"", "az-cli", "client-secret", "client-certificate"} {
				cfg := &types.Config{
					Output:   output,
					LogLevel: level,
					Auth:     types.AuthConfig{Mode: mode},
				}
				if err := Validate(cfg); err != nil {
					t.Errorf("Validate(output=%q, level=%q, mode=%q) = %v, want nil",
						output, level, mode, err)
				}
			}
		}
	}
}
