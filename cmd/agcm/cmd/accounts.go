// SPDX-License-Identifier: GPL-3.0-or-later
package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get all cases to extract unique account numbers
	// This is a workaround since there's no direct accounts API
	result, err := client.ListCases(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to list cases: %w", err)
	}

	if len(result.Items) == 0 {
		fmt.Println("No cases found. You may not have access to any accounts with cases.")
		fmt.Println("\nTo view cases for a specific account, use:")
		fmt.Println("  agcm list cases --account <account-number>")
		return nil
	}

	// Extract unique accounts
	accounts := make(map[string]string) // number -> name
	for _, c := range result.Items {
		if c.AccountNumber != "" {
			accounts[c.AccountNumber] = c.AccountName
		}
	}

	if len(accounts) == 0 {
		fmt.Println("No accounts found in accessible cases.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ACCOUNT NUMBER\tACCOUNT NAME")
	fmt.Fprintln(w, "--------------\t------------")

	for num, name := range accounts {
		fmt.Fprintf(w, "%s\t%s\n", num, name)
	}
	w.Flush()

	fmt.Printf("\nFound %d account(s)\n", len(accounts))
	fmt.Println("\nTo view cases for a specific account:")
	fmt.Println("  agcm list cases --account <account-number>")

	return nil
}
