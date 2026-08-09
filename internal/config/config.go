// Package config loads reel's configuration from a TOML file and/or
// environment variables.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Config holds the settings reel needs to talk to a Seerr instance.
type Config struct {
	SeerrURL    string `toml:"seerr_url"`
	SeerrAPIKey string `toml:"seerr_api_key"`
}

// Load reads configuration from the config file, then applies environment
// variable overrides. It returns an error if the Seerr URL or API key are
// not set by either source.
func Load() (*Config, error) {
	cfg := &Config{}

	path, err := configPath()
	if err == nil {
		if data, err := os.ReadFile(path); err == nil {
			if err := toml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("parsing config file %s: %w", path, err)
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading config file %s: %w", path, err)
		}
	}

	if v := os.Getenv("REEL_SEERR_URL"); v != "" {
		cfg.SeerrURL = v
	}
	if v := os.Getenv("REEL_SEERR_API_KEY"); v != "" {
		cfg.SeerrAPIKey = v
	}

	var missing []string
	if cfg.SeerrURL == "" {
		missing = append(missing, fmt.Sprintf("Seerr URL (set REEL_SEERR_URL or seerr_url in %s)", path))
	}
	if cfg.SeerrAPIKey == "" {
		missing = append(missing, fmt.Sprintf("Seerr API key (set REEL_SEERR_API_KEY or seerr_api_key in %s)", path))
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required configuration: %s", strings.Join(missing, "; "))
	}

	return cfg, nil
}

// configPath returns the path to reel's config file, honoring
// $XDG_CONFIG_HOME on Linux via os.UserConfigDir.
func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "reel", "config.toml"), nil
}
