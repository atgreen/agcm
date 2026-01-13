// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package auth

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
)

const (
	tokenFileName  = "token"
	dirPerms       = 0700
	filePerms      = 0600
	keyringService = "agcm"
	keyringUser    = "offline-token"
)

// Storage handles credential persistence
type Storage struct {
	configDir      string
	keyringEnabled bool
}

// NewStorage creates a new Storage instance
func NewStorage(configDir string) *Storage {
	s := &Storage{configDir: configDir}
	// Test if keyring is available
	s.keyringEnabled = s.testKeyring()
	return s
}

// testKeyring checks if the system keyring is available
func (s *Storage) testKeyring() bool {
	// Try to access keyring - if it fails, fall back to file storage
	err := keyring.Set(keyringService, "test", "test")
	if err != nil {
		return false
	}
	// Clean up test entry
	_ = keyring.Delete(keyringService, "test")
	return true
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
func (s *Storage) SaveToken(token string) error {
	if s.keyringEnabled {
		err := keyring.Set(keyringService, keyringUser, token)
		if err == nil {
			// Remove any old file-based token
			_ = s.deleteFileToken()
			return nil
		}
		// Fall through to file storage if keyring fails
	}

	return s.saveFileToken(token)
}

// LoadToken retrieves the stored offline token
func (s *Storage) LoadToken() (string, error) {
	if s.keyringEnabled {
		token, err := keyring.Get(keyringService, keyringUser)
		if err == nil && token != "" {
			return token, nil
		}
		// Fall through to file storage if keyring fails or is empty
	}

	return s.loadFileToken()
}

// DeleteToken removes the stored token
func (s *Storage) DeleteToken() error {
	var keyringErr, fileErr error

	if s.keyringEnabled {
		keyringErr = keyring.Delete(keyringService, keyringUser)
	}

	fileErr = s.deleteFileToken()

	// Return error only if both fail and at least one had a token
	if keyringErr != nil && fileErr != nil {
		return fmt.Errorf("failed to delete token")
	}
	return nil
}

// HasToken checks if a token is stored
func (s *Storage) HasToken() bool {
	if s.keyringEnabled {
		token, err := keyring.Get(keyringService, keyringUser)
		if err == nil && token != "" {
			return true
		}
	}

	path := filepath.Join(s.configDir, tokenFileName)
	_, err := os.Stat(path)
	return err == nil
}

// ConfigDir returns the configuration directory path
func (s *Storage) ConfigDir() string {
	return s.configDir
}

// UsingKeyring returns true if keyring storage is being used
func (s *Storage) UsingKeyring() bool {
	return s.keyringEnabled
}

// File-based storage fallback methods

func (s *Storage) saveFileToken(token string) error {
	if err := s.EnsureDir(); err != nil {
		return err
	}

	// Basic obfuscation - not secure, just prevents casual viewing
	encoded := base64.StdEncoding.EncodeToString([]byte(token))

	path := filepath.Join(s.configDir, tokenFileName)
	return os.WriteFile(path, []byte(encoded), filePerms)
}

func (s *Storage) loadFileToken() (string, error) {
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

func (s *Storage) deleteFileToken() error {
	path := filepath.Join(s.configDir, tokenFileName)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete token: %w", err)
	}
	return nil
}
