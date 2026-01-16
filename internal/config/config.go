// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	configFileName = "config.yaml"
	dirPerms       = 0700
	filePerms      = 0644
)

// FilterPreset represents a saved filter configuration
type FilterPreset struct {
	Name     string   `yaml:"name"`
	Accounts []string `yaml:"accounts,omitempty"`
	Status   []string `yaml:"status,omitempty"`
	Severity []string `yaml:"severity,omitempty"`
	Products []string `yaml:"products,omitempty"`
	Keyword  string   `yaml:"keyword,omitempty"`
}

// Config represents the application configuration
type Config struct {
	API      APIConfig              `yaml:"api"`
	UI       UIConfig               `yaml:"ui"`
	Defaults DefaultsConfig         `yaml:"defaults"`
	Presets  map[string]*FilterPreset `yaml:"presets,omitempty"`
}

// APIConfig contains API-related settings
type APIConfig struct {
	BaseURL string        `yaml:"base_url"`
	Timeout time.Duration `yaml:"timeout"`
}

// DefaultsConfig contains default filter values
type DefaultsConfig struct {
	AccountNumber string `yaml:"account_number"` // Default account to filter by
	GroupNumber   string `yaml:"group_number"`   // Default group to filter by
}

// UIConfig contains UI-related settings
type UIConfig struct {
	Theme    string `yaml:"theme"`
	PageSize int    `yaml:"page_size"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		API: APIConfig{
			BaseURL: "https://api.access.redhat.com",
			Timeout: 30 * time.Second,
		},
		UI: UIConfig{
			Theme:    "dark",
			PageSize: 25,
		},
	}
}

// Manager handles configuration loading and saving
type Manager struct {
	configDir string
	config    *Config
}

// NewManager creates a new configuration manager
func NewManager(configDir string) *Manager {
	return &Manager{
		configDir: configDir,
		config:    DefaultConfig(),
	}
}

// DefaultConfigDir returns the default configuration directory
func DefaultConfigDir() (string, error) {
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "agcm"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".config", "agcm"), nil
}

// Load reads the configuration from disk
func (m *Manager) Load() error {
	path := filepath.Join(m.configDir, configFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Use defaults if no config file exists
			return nil
		}
		return fmt.Errorf("failed to read config: %w", err)
	}

	if err := yaml.Unmarshal(data, m.config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	return nil
}

// Save writes the configuration to disk
func (m *Manager) Save() error {
	if err := os.MkdirAll(m.configDir, dirPerms); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(m.config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	path := filepath.Join(m.configDir, configFileName)
	if err := os.WriteFile(path, data, filePerms); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// Get returns the current configuration
func (m *Manager) Get() *Config {
	return m.config
}

// Set updates the configuration
func (m *Manager) Set(cfg *Config) {
	m.config = cfg
}

// GetBaseURL returns the API base URL
func (m *Manager) GetBaseURL() string {
	return m.config.API.BaseURL
}

// GetTimeout returns the API timeout
func (m *Manager) GetTimeout() time.Duration {
	return m.config.API.Timeout
}

// GetTheme returns the UI theme
func (m *Manager) GetTheme() string {
	return m.config.UI.Theme
}

// GetPageSize returns the UI page size
func (m *Manager) GetPageSize() int {
	return m.config.UI.PageSize
}

// GetPreset returns a filter preset by slot key (1-9, 0)
func (m *Manager) GetPreset(slot string) *FilterPreset {
	if m.config.Presets == nil {
		return nil
	}
	return m.config.Presets[slot]
}

// SetPreset saves a filter preset to a slot
func (m *Manager) SetPreset(slot string, preset *FilterPreset) {
	if m.config.Presets == nil {
		m.config.Presets = make(map[string]*FilterPreset)
	}
	m.config.Presets[slot] = preset
}

// GetPresets returns all presets
func (m *Manager) GetPresets() map[string]*FilterPreset {
	return m.config.Presets
}
