// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/green/agcm/internal/api"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List resources",
	Long:  `List cases, products, or other resources.`,
}

var listCasesCmd = &cobra.Command{
	Use:   "cases [preset]",
	Short: "List support cases",
	Long: `List support cases with optional filtering.

You can use a saved filter preset (0-9) created in the TUI, and/or CLI flags.
If only a preset number is provided with no matching preset saved, nothing is listed.

Status values (case-insensitive):
  Open, Closed, "Waiting on Red Hat", "Waiting on Customer"

Severity values (can use just the number):
  1, 2, 3, 4  or  "1 (Urgent)", "2 (High)", "3 (Normal)", "4 (Low)"

Examples:
  agcm list cases 1                         # List using preset 1
  agcm list cases --status Open
  agcm list cases --status "Waiting on Red Hat"
  agcm list cases --status Closed           # List closed cases
  agcm list cases 1 --severity 1,2          # Preset 1 + severity filter
  agcm list cases --account 12345678`,
	Args: cobra.MaximumNArgs(1),
	RunE: runListCases,
}

var (
	listStatus   string
	listSeverity string
	listProduct  string
	listAccount  string
	listGroup    string
	listOwner    string
	listLimit    int
)

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.AddCommand(listCasesCmd)

	listCasesCmd.Flags().StringVar(&listStatus, "status", "", "filter by status (comma-separated)")
	listCasesCmd.Flags().StringVar(&listSeverity, "severity", "", "filter by severity (comma-separated)")
	listCasesCmd.Flags().StringVar(&listProduct, "product", "", "filter by product")
	listCasesCmd.Flags().StringVarP(&listAccount, "account", "a", "", "filter by account number")
	listCasesCmd.Flags().StringVarP(&listGroup, "group", "g", "", "filter by case group number")
	listCasesCmd.Flags().StringVar(&listOwner, "owner", "", "filter by owner SSO username")
	listCasesCmd.Flags().IntVarP(&listLimit, "limit", "n", 25, "maximum number of cases to show")
}

func runListCases(cmd *cobra.Command, args []string) error {
	client := GetAPIClient()

	filter := &api.CaseFilter{
		Count: listLimit,
	}

	hasCliFilters := listStatus != "" || listSeverity != "" || listProduct != "" ||
		listAccount != "" || listGroup != "" || listOwner != ""

	// Check for preset argument (0-9)
	if len(args) == 1 {
		presetSlot := args[0]
		if len(presetSlot) == 1 && presetSlot[0] >= '0' && presetSlot[0] <= '9' {
			preset := configMgr.GetPreset(presetSlot)
			if preset == nil {
				if !hasCliFilters {
					fmt.Printf("No preset saved in slot %s. Nothing to list.\n", presetSlot)
					return nil
				}
				// Has CLI filters, continue without preset
			} else {
				// Load preset filters as defaults
				fmt.Printf("Using preset %s: %s\n", presetSlot, preset.Name)
				if len(preset.Status) > 0 {
					filter.Status = preset.Status
				}
				if len(preset.Severity) > 0 {
					filter.Severity = preset.Severity
				}
				if len(preset.Products) > 0 {
					filter.Products = preset.Products
				}
				if len(preset.Accounts) > 0 {
					filter.Accounts = preset.Accounts
				}
			}
		} else {
			return fmt.Errorf("invalid preset: %s (must be 0-9)", presetSlot)
		}
	}

	// CLI flags override preset values
	if listStatus != "" {
		filter.Status = strings.Split(listStatus, ",")
		// If user explicitly filters for Closed, set IncludeClosed
		for _, s := range filter.Status {
			if strings.EqualFold(s, "Closed") {
				filter.IncludeClosed = true
				break
			}
		}
	}
	if listSeverity != "" {
		filter.Severity = strings.Split(listSeverity, ",")
	}
	if listProduct != "" {
		products := strings.Split(listProduct, ",")
		for i := range products {
			products[i] = strings.TrimSpace(products[i])
		}
		filter.Products = products
	}
	if listAccount != "" {
		accounts := strings.Split(listAccount, ",")
		for i := range accounts {
			accounts[i] = strings.TrimSpace(accounts[i])
		}
		filter.Accounts = accounts
	}
	if listGroup != "" {
		filter.GroupNumber = listGroup
	}
	if listOwner != "" {
		filter.OwnerSSOName = listOwner
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := client.ListCases(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to list cases: %w", err)
	}

	if len(result.Items) == 0 {
		fmt.Println("No cases found matching the criteria.")
		return nil
	}

	// Get terminal width
	termWidth := 120 // default
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		termWidth = w
	}

	// Calculate column widths based on terminal size
	// Fixed columns: CASE(10) + SEV(4) + STATUS(22) + spacing(12) = ~48
	// Remaining space split between PRODUCT and SUMMARY
	fixedWidth := 48
	remaining := termWidth - fixedWidth
	productWidth := remaining / 4       // 25% for product
	summaryWidth := remaining - productWidth // 75% for summary

	if productWidth < 15 {
		productWidth = 15
	}
	if summaryWidth < 30 {
		summaryWidth = 30
	}

	// Print table
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "CASE\tSEV\tSTATUS\tPRODUCT\tSUMMARY")
	_, _ = fmt.Fprintln(tw, "----\t---\t------\t-------\t-------")

	for _, c := range result.Items {
		summary := c.Summary
		if len(summary) > summaryWidth {
			summary = summary[:summaryWidth-3] + "..."
		}
		product := c.Product
		if len(product) > productWidth {
			product = product[:productWidth-3] + "..."
		}

		// Shorten severity display
		sev := c.Severity
		if len(sev) > 0 {
			sev = string(sev[0]) // Just the number
		}

		// Shorten status
		status := c.Status
		if len(status) > 20 {
			status = status[:17] + "..."
		}

		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			c.CaseNumber,
			sev,
			status,
			product,
			summary,
		)
	}
	_ = tw.Flush()

	fmt.Printf("\nShowing %d of %d cases\n", len(result.Items), result.TotalCount)

	return nil
}
