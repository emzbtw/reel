package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfigFile(t *testing.T, contents string) {
	t.Helper()
	configDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configDir)

	if contents == "" {
		return
	}
	dir := filepath.Join(configDir, "reel")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_FromFile(t *testing.T) {
	writeConfigFile(t, `
seerr_url = "https://seerr.example.com"
seerr_api_key = "file-key"
`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.SeerrURL != "https://seerr.example.com" {
		t.Errorf("SeerrURL = %q, want %q", cfg.SeerrURL, "https://seerr.example.com")
	}
	if cfg.SeerrAPIKey != "file-key" {
		t.Errorf("SeerrAPIKey = %q, want %q", cfg.SeerrAPIKey, "file-key")
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	writeConfigFile(t, `
seerr_url = "https://seerr.example.com"
seerr_api_key = "file-key"
`)
	t.Setenv("REEL_SEERR_API_KEY", "env-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.SeerrURL != "https://seerr.example.com" {
		t.Errorf("SeerrURL = %q, want %q", cfg.SeerrURL, "https://seerr.example.com")
	}
	if cfg.SeerrAPIKey != "env-key" {
		t.Errorf("SeerrAPIKey = %q, want %q", cfg.SeerrAPIKey, "env-key")
	}
}

func TestLoad_EnvOnly(t *testing.T) {
	writeConfigFile(t, "")
	t.Setenv("REEL_SEERR_URL", "https://seerr.example.com")
	t.Setenv("REEL_SEERR_API_KEY", "env-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.SeerrURL != "https://seerr.example.com" {
		t.Errorf("SeerrURL = %q, want %q", cfg.SeerrURL, "https://seerr.example.com")
	}
	if cfg.SeerrAPIKey != "env-key" {
		t.Errorf("SeerrAPIKey = %q, want %q", cfg.SeerrAPIKey, "env-key")
	}
}

func TestLoad_MissingBoth(t *testing.T) {
	writeConfigFile(t, "")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() returned nil error, want error for missing config")
	}
}

func TestLoad_MissingAPIKeyOnly(t *testing.T) {
	writeConfigFile(t, `seerr_url = "https://seerr.example.com"`)

	_, err := Load()
	if err == nil {
		t.Fatal("Load() returned nil error, want error for missing API key")
	}
}
