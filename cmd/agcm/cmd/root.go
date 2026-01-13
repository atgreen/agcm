// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/green/agcm/internal/api"
	"github.com/green/agcm/internal/auth"
	"github.com/green/agcm/internal/config"
	"github.com/green/agcm/internal/tui"
	"github.com/spf13/cobra"
)

var (
	cfgDir        string
	debugMode     bool
	maskMode      bool
	tuiAccount    string
	tuiGroup      string
	configMgr     *config.Manager
	tokenMgr      *auth.TokenManager
	storage       *auth.Storage
	apiClient     *api.Client
	version       string
)

// SetVersion sets the application version (called from main)
func SetVersion(v string) {
	version = v
	rootCmd.Version = v
}

var rootCmd = &cobra.Command{
	Use:     "agcm",
	Short:   "TUI for the Red Hat Support Portal",
	Version: "dev", // Will be overridden by SetVersion
	Long: `A terminal user interface for browsing Red Hat support cases.
Filter, sort, search, and export cases to markdown.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip initialization for auth commands and update (doesn't need auth)
		if cmd.Name() == "login" || cmd.Name() == "logout" || cmd.Name() == "status" || cmd.Name() == "update" {
			return nil
		}

		return initApp()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Build TUI options from flags and config defaults
		opts := tui.Options{
			AccountNumber: tuiAccount,
			GroupNumber:   tuiGroup,
			MaskMode:      maskMode,
			Version:       version,
		}

		// Use config defaults if not specified on command line
		if opts.AccountNumber == "" {
			opts.AccountNumber = configMgr.Get().Defaults.AccountNumber
		}
		if opts.GroupNumber == "" {
			opts.GroupNumber = configMgr.Get().Defaults.GroupNumber
		}

		// Launch TUI
		return tui.Run(apiClient, opts)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Determine config directory
	defaultCfgDir, err := config.DefaultConfigDir()
	if err != nil {
		defaultCfgDir = "~/.config/agcm"
	}

	rootCmd.PersistentFlags().StringVar(&cfgDir, "config", defaultCfgDir, "config directory")
	rootCmd.PersistentFlags().BoolVar(&debugMode, "debug", false, "enable debug output")

	// TUI-specific flags (on root command, not persistent)
	rootCmd.Flags().StringVarP(&tuiAccount, "account", "a", "", "filter by account number")
	rootCmd.Flags().StringVarP(&tuiGroup, "group", "g", "", "filter by case group number")
	rootCmd.Flags().BoolVar(&maskMode, "mask", false, "mask sensitive text for screenshots")
}

func initApp() error {
	var err error

	// Initialize config manager
	if cfgDir == "" {
		cfgDir, err = config.DefaultConfigDir()
		if err != nil {
			return fmt.Errorf("failed to get config directory: %w", err)
		}
	}

	configMgr = config.NewManager(cfgDir)
	if err := configMgr.Load(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize auth storage
	storage = auth.NewStorage(cfgDir)

	// Load token
	token, err := storage.LoadToken()
	if err != nil {
		return fmt.Errorf("failed to load token: %w", err)
	}

	if token == "" {
		return fmt.Errorf("not authenticated. Run 'agcm auth login' first")
	}

	// Initialize token manager
	tokenMgr = auth.NewTokenManager(token)

	// Initialize API client
	apiClient = api.NewClient(
		api.WithBaseURL(configMgr.GetBaseURL()),
		api.WithTokenRefresher(func(ctx context.Context) (string, error) {
			return tokenMgr.GetAccessToken(ctx)
		}),
		api.WithDebug(debugMode),
	)

	return nil
}

// GetAPIClient returns the initialized API client
func GetAPIClient() *api.Client {
	return apiClient
}

// GetConfigDir returns the configuration directory
func GetConfigDir() string {
	return cfgDir
}

// IsDebugMode returns whether debug mode is enabled
func IsDebugMode() bool {
	return debugMode
}

// GetStorage returns the auth storage
func GetStorage() *auth.Storage {
	return storage
}
