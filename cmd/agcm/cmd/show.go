// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/green/agcm/internal/export"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show resource details",
	Long:  `Show detailed information about a case or other resource.`,
}

var showCaseCmd = &cobra.Command{
	Use:   "case [case-number]",
	Short: "Show case details",
	Long: `Show detailed information about a specific case.

Examples:
  agcm show case 01234567`,
	Args: cobra.ExactArgs(1),
	RunE: runShowCase,
}

var showComments bool

func init() {
	rootCmd.AddCommand(showCmd)
	showCmd.AddCommand(showCaseCmd)

	showCaseCmd.Flags().BoolVar(&showComments, "comments", true, "include comments")
}

func runShowCase(cmd *cobra.Command, args []string) error {
	client := GetAPIClient()
	caseNumber := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get case details
	c, err := client.GetCase(ctx, caseNumber)
	if err != nil {
		return fmt.Errorf("failed to get case: %w", err)
	}

	// Get comments
	var comments []any
	if showComments {
		commentsResult, err := client.GetCaseComments(ctx, caseNumber)
		if err == nil {
			for _, c := range commentsResult {
				comments = append(comments, c)
			}
		}
	}

	// Get attachments
	attachments, _ := client.GetCaseAttachments(ctx, caseNumber)

	// Use the export formatter to generate markdown
	md, err := export.QuickFormat(c, nil, attachments)
	if err != nil {
		return fmt.Errorf("failed to format case: %w", err)
	}

	fmt.Println(md)

	// Print comments separately for better CLI output
	if showComments && len(comments) > 0 {
		commentsResult, _ := client.GetCaseComments(ctx, caseNumber)
		fmt.Println("\n## Comments\n")
		for i, comment := range commentsResult {
			fmt.Printf("### Comment %d\n", i+1)
			fmt.Printf("**From:** %s\n", comment.Author)
			fmt.Printf("**Date:** %s\n", comment.CreatedDate.Format("2006-01-02 15:04"))
			fmt.Printf("**Public:** %v\n\n", comment.Public)

			text := comment.Text
			if len(text) > 2000 {
				text = text[:2000] + "\n... (truncated)"
			}
			fmt.Println(text)
			fmt.Println("\n---")
		}
	}

	return nil
}
