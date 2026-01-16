// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/green/agcm/internal/api"
	"github.com/spf13/cobra"
)

var listAccountsCmd = &cobra.Command{
	Use:   "accounts",
	Short: "List accessible accounts",
	Long: `List Red Hat accounts you have access to.

This shows accounts based on cases you can view. To see cases for a specific
account, use:

  agcm list cases --account <account-number>`,
	RunE: runListAccounts,
}

func init() {
	listCmd.AddCommand(listAccountsCmd)
}

func runListAccounts(cmd *cobra.Command, args []string) error {
	client := GetAPIClient()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Get all cases to extract unique account numbers
	// This is a workaround since there's no direct accounts API
	// Paginate through all results
	accounts := make(map[string]string) // number -> name
	startIndex := 0
	pageSize := 100

	fmt.Println("Fetching accounts from cases...")

	for {
		filter := &api.CaseFilter{
			StartIndex:    startIndex,
			Count:         pageSize,
			IncludeClosed: true, // Include all cases to get all accounts
		}
		result, err := client.ListCases(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to list cases: %w", err)
		}

		for _, c := range result.Items {
			if c.AccountNumber != "" {
				accounts[c.AccountNumber] = c.AccountName
			}
		}

		// Check if we've fetched all cases
		if len(result.Items) < pageSize || startIndex+len(result.Items) >= result.TotalCount {
			break
		}
		startIndex += len(result.Items)
	}

	if len(accounts) == 0 {
		fmt.Println("No accounts found in accessible cases.")
		fmt.Println("\nTo view cases for a specific account, use:")
		fmt.Println("  agcm list cases --account <account-number>")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ACCOUNT NUMBER\tACCOUNT NAME")
	_, _ = fmt.Fprintln(w, "--------------\t------------")

	for num, name := range accounts {
		_, _ = fmt.Fprintf(w, "%s\t%s\n", num, name)
	}
	_ = w.Flush()

	fmt.Printf("\nFound %d account(s)\n", len(accounts))
	fmt.Println("\nTo view cases for a specific account:")
	fmt.Println("  agcm list cases --account <account-number>")

	return nil
}
