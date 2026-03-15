package types

import (
	"errors"
	"testing"
)

func TestFirstTenant(t *testing.T) {
	tests := []struct {
		name    string
		tenants []string
		want    string
	}{
		{"with tenants", []string{"a", "b"}, "a"},
		{"empty", nil, ""},
		{"single", []string{"x"}, "x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Tenants: tt.tenants}
			if got := cfg.FirstTenant(); got != tt.want {
				t.Errorf("FirstTenant() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExitError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  ExitError
		want string
	}{
		{
			"message only",
			ExitError{Code: 2, Message: "something failed"},
			"something failed",
		},
		{
			"with wrapped error",
			ExitError{Code: 2, Message: "auth", Err: errors.New("token expired")},
			"auth: token expired",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExitError_Unwrap(t *testing.T) {
	inner := errors.New("inner")
	e := &ExitError{Code: 1, Message: "outer", Err: inner}
	if !errors.Is(e, inner) {
		t.Error("Unwrap() should make inner error accessible via errors.Is")
	}
}

func TestExitError_Unwrap_Nil(t *testing.T) {
	e := &ExitError{Code: 1, Message: "no inner"}
	if e.Unwrap() != nil {
		t.Error("Unwrap() should return nil when no wrapped error")
	}
}
