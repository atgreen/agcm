// SPDX-License-Identifier: GPL-3.0-or-later
package auth

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	tokenFileName = "token"
	dirPerms      = 0700
	filePerms     = 0600
)

// Storage handles credential persistence
type Storage struct {
	configDir string
}

// NewStorage creates a new Storage instance
func NewStorage(configDir string) *Storage {
	return &Storage{configDir: configDir}
}

// DefaultConfigDir returns the default configuration directory
func DefaultConfigDir() (string, error) {
	// Check XDG_CONFIG_HOME first
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		return filepath.Join(xdgConfig, "agcm"), nil
	}

	// Fall back to ~/.config
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".config", "agcm"), nil
}

// EnsureDir creates the config directory if it doesn't exist
func (s *Storage) EnsureDir() error {
	return os.MkdirAll(s.configDir, dirPerms)
}

// SaveToken stores the offline token
// Note: This uses basic obfuscation, not encryption. For production,
// consider using OS keychain (keyring, keyctl, etc.)
func (s *Storage) SaveToken(token string) error {
	if err := s.EnsureDir(); err != nil {
		return err
	}

	// Basic obfuscation - not secure, just prevents casual viewing
	encoded := base64.StdEncoding.EncodeToString([]byte(token))

	path := filepath.Join(s.configDir, tokenFileName)
	return os.WriteFile(path, []byte(encoded), filePerms)
}

// LoadToken retrieves the stored offline token
func (s *Storage) LoadToken() (string, error) {
	path := filepath.Join(s.configDir, tokenFileName)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read token file: %w", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		return "", fmt.Errorf("failed to decode token: %w", err)
	}

	return string(decoded), nil
}

// DeleteToken removes the stored token
func (s *Storage) DeleteToken() error {
	path := filepath.Join(s.configDir, tokenFileName)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete token: %w", err)
	}
	return nil
}

// HasToken checks if a token is stored
func (s *Storage) HasToken() bool {
	path := filepath.Join(s.configDir, tokenFileName)
	_, err := os.Stat(path)
	return err == nil
}

// ConfigDir returns the configuration directory path
func (s *Storage) ConfigDir() string {
	return s.configDir
}
