// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package cmd

import (
	"fmt"
	"strings"

	"github.com/blang/semver"
	"github.com/rhysd/go-github-selfupdate/selfupdate"
	"github.com/spf13/cobra"
)

const repoSlug = "atgreen/agcm"

var checkOnly bool

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update agcm to the latest version",
	Long:  `Check for and install updates from GitHub releases.`,
	RunE:  runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().BoolVar(&checkOnly, "check", false, "only check for updates, don't install")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	current := version
	if current == "" || current == "dev" {
		return fmt.Errorf("cannot update development build; install a release version")
	}

	// Strip 'v' prefix if present for semver parsing
	current = strings.TrimPrefix(current, "v")

	// Handle dirty builds
	if strings.Contains(current, "+") {
		current = strings.Split(current, "+")[0]
	}

	v, err := semver.Parse(current)
	if err != nil {
		return fmt.Errorf("invalid version format %q: %w", current, err)
	}

	fmt.Printf("Current version: %s\n", version)
	fmt.Println("Checking for updates...")

	// Use updater without authentication for public repo
	updater, _ := selfupdate.NewUpdater(selfupdate.Config{})
	latest, found, err := updater.DetectLatest(repoSlug)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if !found {
		fmt.Println("No releases found.")
		return nil
	}

	if latest.Version.LTE(v) {
		fmt.Printf("Already up to date (latest: v%s)\n", latest.Version)
		return nil
	}

	fmt.Printf("New version available: v%s\n", latest.Version)

	if checkOnly {
		fmt.Println("\nRun 'agcm update' to install.")
		return nil
	}

	fmt.Println("Downloading and installing...")

	release, err := updater.UpdateSelf(v, repoSlug)
	if err != nil {
		return fmt.Errorf("failed to update: %w", err)
	}

	fmt.Printf("Successfully updated to v%s\n", release.Version)

	if latest.ReleaseNotes != "" {
		fmt.Println("\nRelease notes:")
		fmt.Println(latest.ReleaseNotes)
	}

	return nil
}
