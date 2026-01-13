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
	Use:   "cases",
	Short: "List support cases",
	Long: `List support cases with optional filtering.

By default, closed cases are excluded. Use --status to filter by specific statuses.

Status values (case-insensitive):
  Open, Closed, "Waiting on Red Hat", "Waiting on Customer"

Severity values (can use just the number):
  1, 2, 3, 4  or  "1 (Urgent)", "2 (High)", "3 (Normal)", "4 (Low)"

Examples:
  agcm list cases                           # List open cases
  agcm list cases --status Open
  agcm list cases --status "Waiting on Red Hat"
  agcm list cases --status Closed           # List closed cases
  agcm list cases --severity 1,2 --limit 10
  agcm list cases --account 12345678
  agcm list cases --debug                   # Show API debug info`,
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
		filter.Product = listProduct
	}
	if listAccount != "" {
		filter.AccountNumber = listAccount
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
	fmt.Fprintln(tw, "CASE\tSEV\tSTATUS\tPRODUCT\tSUMMARY")
	fmt.Fprintln(tw, "----\t---\t------\t-------\t-------")

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

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			c.CaseNumber,
			sev,
			status,
			product,
			summary,
		)
	}
	tw.Flush()

	fmt.Printf("\nShowing %d of %d cases\n", len(result.Items), result.TotalCount)

	return nil
}
