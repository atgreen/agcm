// SPDX-License-Identifier: GPL-3.0-or-later
package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search for cases, solutions, and articles",
	Long: `Search across cases, solutions, and articles.

Examples:
  agcm search "kernel panic"
  agcm search "NVMe driver" --limit 20`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSearch,
}

var searchLimit int

func init() {
	rootCmd.AddCommand(searchCmd)
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "n", 10, "maximum number of results")
}

func runSearch(cmd *cobra.Command, args []string) error {
	client := GetAPIClient()
	query := strings.Join(args, " ")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := client.Search(ctx, query, searchLimit)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	fmt.Printf("Found %d results for '%s':\n\n", len(results), query)

	// Group by type
	var cases, solutions, articles []struct {
		id    string
		title string
	}

	for _, r := range results {
		item := struct {
			id    string
			title string
		}{r.ID, r.Title}

		switch r.Type {
		case "case":
			cases = append(cases, item)
		case "solution":
			solutions = append(solutions, item)
		case "article":
			articles = append(articles, item)
		}
	}

	if len(cases) > 0 {
		fmt.Println("CASES")
		for _, c := range cases {
			title := c.title
			if len(title) > 60 {
				title = title[:57] + "..."
			}
			fmt.Printf("  [C] %s - %s\n", c.id, title)
		}
		fmt.Println()
	}

	if len(solutions) > 0 {
		fmt.Println("SOLUTIONS")
		for _, s := range solutions {
			title := s.title
			if len(title) > 60 {
				title = title[:57] + "..."
			}
			fmt.Printf("  [S] %s - %s\n", s.id, title)
		}
		fmt.Println()
	}

	if len(articles) > 0 {
		fmt.Println("ARTICLES")
		for _, a := range articles {
			title := a.title
			if len(title) > 60 {
				title = title[:57] + "..."
			}
			fmt.Printf("  [A] %s - %s\n", a.id, title)
		}
		fmt.Println()
	}

	return nil
}
