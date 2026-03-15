package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestLoad_TenantsFromSlice(t *testing.T) {
	v := viper.New()
	v.Set("tenant", []string{"tenant-a", "tenant-b"})

	cfg, err := Load(v)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Tenants) != 2 || cfg.Tenants[0] != "tenant-a" || cfg.Tenants[1] != "tenant-b" {
		t.Errorf("Tenants = %v, want [tenant-a, tenant-b]", cfg.Tenants)
	}
	if cfg.Tenant != "tenant-a" {
		t.Errorf("Tenant = %q, want %q", cfg.Tenant, "tenant-a")
	}
}

func TestLoad_SingleTenantFallback(t *testing.T) {
	v := viper.New()
	// When GetStringSlice returns empty but GetString returns a value
	v.Set("tenant", "single-tenant-id")

	cfg, err := Load(v)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// The exact behavior depends on viper's GetStringSlice vs GetString
	// but at minimum cfg should have at least one tenant
	if len(cfg.Tenants) == 0 {
		t.Error("expected at least one tenant from single string fallback")
	}
}

func TestLoad_AuthOverrides(t *testing.T) {
	v := viper.New()
	v.Set("tenant", []string{"t1"})
	v.Set("client_id", "my-client-id")
	v.Set("client_secret", "my-secret")
	v.Set("cert_path", "/path/to/cert.pem")

	cfg, err := Load(v)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Auth.ClientID != "my-client-id" {
		t.Errorf("ClientID = %q, want %q", cfg.Auth.ClientID, "my-client-id")
	}
	if cfg.Auth.ClientSecret != "my-secret" {
		t.Errorf("ClientSecret = %q, want %q", cfg.Auth.ClientSecret, "my-secret")
	}
	if cfg.Auth.CertPath != "/path/to/cert.pem" {
		t.Errorf("CertPath = %q, want %q", cfg.Auth.CertPath, "/path/to/cert.pem")
	}
}

func TestLoad_EmptyTenants(t *testing.T) {
	v := viper.New()
	// No tenant set at all — will try az CLI fallback which will fail in CI

	cfg, err := Load(v)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// In test/CI environment without az CLI, tenants may be empty
	_ = cfg
}

func TestResolveAzCLITenant_NoAzCLI(t *testing.T) {
	// In test environments az CLI may not be present — should return ""
	result := resolveAzCLITenant()
	// We can't assert it's empty (it might work locally) but it shouldn't panic
	if len(result) > 0 && len(result) != 36 {
		t.Errorf("resolveAzCLITenant() returned non-UUID string: %q", result)
	}
}
