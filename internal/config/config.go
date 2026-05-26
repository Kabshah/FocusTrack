package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds all user-persisted settings for FocusTrack.
type Config struct {
	StartWithWindows bool           `json:"startWithWindows"`
	NotifyDaily      bool           `json:"notifyDaily"`
	LimitAlerts      bool           `json:"limitAlerts"`
	AppLimits        map[string]int `json:"app_limits"` // app name → daily minutes
}

// ConfigDir returns the %APPDATA%\FocusTrack directory path.
func ConfigDir() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData, _ = os.UserConfigDir()
	}
	return filepath.Join(appData, "FocusTrack")
}

// configPath returns the full path to settings.json.
func configPath() string {
	return filepath.Join(ConfigDir(), "settings.json")
}

// Load reads settings from %APPDATA%\FocusTrack\settings.json.
// Returns a default config if the file does not exist yet.
func Load() (*Config, error) {
	cfg := &Config{
		NotifyDaily: true,
		AppLimits:   make(map[string]int),
	}

	data, err := os.ReadFile(configPath())
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if cfg.AppLimits == nil {
		cfg.AppLimits = make(map[string]int)
	}
	return cfg, nil
}

// Save writes settings to %APPDATA%\FocusTrack\settings.json.
func Save(cfg *Config) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	if err := os.WriteFile(configPath(), data, 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}
