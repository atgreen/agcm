// SPDX-License-Identifier: GPL-3.0-or-later
package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/green/agcm/internal/auth"
	"github.com/green/agcm/internal/config"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication management",
	Long:  `Manage authentication with Red Hat Customer Portal.`,
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Configure authentication",
	Long: `Configure authentication with your Red Hat offline token.

To generate an offline token:
1. Go to https://access.redhat.com/management/api
2. Click "Generate Token"
3. Copy the token and paste it when prompted`,
	RunE: runLogin,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove stored credentials",
	Long:  `Remove stored authentication credentials.`,
	RunE:  runLogout,
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check authentication status",
	Long:  `Check if authentication is configured and valid.`,
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(logoutCmd)
	authCmd.AddCommand(statusCmd)
}

func runLogin(cmd *cobra.Command, args []string) error {
	// Get config directory
	cfgDir := GetConfigDir()
	if cfgDir == "" {
		var err error
		cfgDir, err = config.DefaultConfigDir()
		if err != nil {
			return fmt.Errorf("failed to get config directory: %w", err)
		}
	}

	storage := auth.NewStorage(cfgDir)

	fmt.Println("Red Hat Support Portal Authentication Setup")
	fmt.Println("==========================================")
	fmt.Println()
	fmt.Println("To authenticate, you need an offline token from Red Hat.")
	fmt.Println()
	fmt.Println("1. Go to: https://access.redhat.com/management/api")
	fmt.Println("2. Log in with your Red Hat account")
	fmt.Println("3. Click 'Generate Token'")
	fmt.Println("4. Copy the token")
	fmt.Println()
	fmt.Print("Enter your offline token: ")

	reader := bufio.NewReader(os.Stdin)
	token, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read token: %w", err)
	}
	token = strings.TrimSpace(token)

	if token == "" {
		return fmt.Errorf("token cannot be empty")
	}

	// Validate the token
	fmt.Print("Validating token... ")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := auth.ValidateOfflineToken(ctx, token); err != nil {
		fmt.Println("FAILED")
		return fmt.Errorf("token validation failed: %w", err)
	}
	fmt.Println("OK")

	// Save the token
	fmt.Print("Saving token... ")
	if err := storage.SaveToken(token); err != nil {
		fmt.Println("FAILED")
		return fmt.Errorf("failed to save token: %w", err)
	}
	fmt.Println("OK")

	fmt.Println()
	fmt.Println("Authentication configured successfully!")
	fmt.Println("You can now use agcm to access the Red Hat Customer Portal.")

	return nil
}

func runLogout(cmd *cobra.Command, args []string) error {
	cfgDir := GetConfigDir()
	if cfgDir == "" {
		var err error
		cfgDir, err = config.DefaultConfigDir()
		if err != nil {
			return fmt.Errorf("failed to get config directory: %w", err)
		}
	}

	storage := auth.NewStorage(cfgDir)

	if !storage.HasToken() {
		fmt.Println("No stored credentials found.")
		return nil
	}

	if err := storage.DeleteToken(); err != nil {
		return fmt.Errorf("failed to remove credentials: %w", err)
	}

	fmt.Println("Credentials removed successfully.")
	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfgDir := GetConfigDir()
	if cfgDir == "" {
		var err error
		cfgDir, err = config.DefaultConfigDir()
		if err != nil {
			return fmt.Errorf("failed to get config directory: %w", err)
		}
	}

	storage := auth.NewStorage(cfgDir)

	if !storage.HasToken() {
		fmt.Println("Status: Not authenticated")
		fmt.Println()
		fmt.Println("Run 'agcm auth login' to configure authentication.")
		return nil
	}

	// Try to load and validate the token
	token, err := storage.LoadToken()
	if err != nil {
		fmt.Println("Status: Error reading stored token")
		return fmt.Errorf("failed to load token: %w", err)
	}

	fmt.Print("Status: Validating token... ")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := auth.ValidateOfflineToken(ctx, token); err != nil {
		fmt.Println("INVALID")
		fmt.Println()
		fmt.Println("Your stored token is no longer valid.")
		fmt.Println("Run 'agcm auth login' to configure a new token.")
		return nil
	}

	fmt.Println("OK")
	fmt.Println()
	fmt.Println("Status: Authenticated")
	fmt.Printf("Config: %s\n", cfgDir)

	return nil
}
