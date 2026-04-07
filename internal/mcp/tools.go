// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/green/agcm/internal/api"
	"github.com/green/agcm/internal/export"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func registerTools(s *server.MCPServer, client *api.Client) {
	s.AddTool(listCasesTool(), listCasesHandler(client))
	s.AddTool(getCaseTool(), getCaseHandler(client))
	s.AddTool(searchTool(), searchHandler(client))
	s.AddTool(getSolutionTool(), getSolutionHandler(client))
	s.AddTool(getArticleTool(), getArticleHandler(client))
	s.AddTool(exportCaseTool(), exportCaseHandler(client))
}

// splitComma splits a comma-separated string into trimmed, non-empty parts.
func splitComma(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// --- list_cases ---

func listCasesTool() mcp.Tool {
	return mcp.NewTool("list_cases",
		mcp.WithDescription("List Red Hat support cases with optional filtering. Returns a concise summary table. Use get_case for full details on a specific case."),
		mcp.WithString("status", mcp.Description("Filter by status (comma-separated): Open, Closed, Waiting on Red Hat, Waiting on Customer")),
		mcp.WithString("severity", mcp.Description("Filter by severity (comma-separated): 1 (Urgent), 2 (High), 3 (Normal), 4 (Low)")),
		mcp.WithString("product", mcp.Description("Filter by product name (comma-separated)")),
		mcp.WithString("account", mcp.Description("Filter by account number (comma-separated)")),
		mcp.WithString("group", mcp.Description("Filter by case group number")),
		mcp.WithString("keyword", mcp.Description("Search keyword to filter cases")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of cases to return (default 25, max 100)")),
		mcp.WithBoolean("include_closed", mcp.Description("Include closed cases in results (default false)")),
	)
}

func listCasesHandler(client *api.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		filter := &api.CaseFilter{}

		if v := req.GetString("status", ""); v != "" {
			filter.Status = splitComma(v)
			for _, s := range filter.Status {
				if strings.EqualFold(s, "Closed") {
					filter.IncludeClosed = true
				}
			}
		}
		if v := req.GetString("severity", ""); v != "" {
			filter.Severity = splitComma(v)
		}
		if v := req.GetString("product", ""); v != "" {
			filter.Products = splitComma(v)
		}
		if v := req.GetString("account", ""); v != "" {
			filter.Accounts = splitComma(v)
		}
		filter.GroupNumber = req.GetString("group", "")
		filter.Keyword = req.GetString("keyword", "")
		filter.IncludeClosed = req.GetBool("include_closed", filter.IncludeClosed)

		limit := req.GetInt("limit", 25)
		if limit < 1 {
			limit = 25
		}
		if limit > 100 {
			limit = 100
		}
		filter.Count = limit

		result, err := client.ListCases(ctx, filter)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list cases: %v", err)), nil
		}

		if len(result.Items) == 0 {
			return mcp.NewToolResultText("No cases found matching the specified filters."), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "| Case | Severity | Status | Product | Summary |\n")
		fmt.Fprintf(&b, "|------|----------|--------|---------|---------|\n")
		for _, c := range result.Items {
			summary := c.Summary
			if len(summary) > 60 {
				summary = summary[:57] + "..."
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
				c.CaseNumber, c.Severity, c.Status, c.Product, summary)
		}
		fmt.Fprintf(&b, "\nShowing %d of %d cases.", len(result.Items), result.TotalCount)

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- get_case ---

func getCaseTool() mcp.Tool {
	return mcp.NewTool("get_case",
		mcp.WithDescription("Get full details of a Red Hat support case including all comments and attachment metadata."),
		mcp.WithString("case_number", mcp.Required(), mcp.Description("The case number (e.g., 01234567)")),
	)
}

func getCaseHandler(client *api.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		caseNumber, err := req.RequireString("case_number")
		if err != nil {
			return mcp.NewToolResultError("case_number is required"), nil
		}

		caseDetail, err := client.GetCase(ctx, caseNumber)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get case %s: %v", caseNumber, err)), nil
		}

		comments, err := client.GetCaseComments(ctx, caseNumber)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get comments for case %s: %v", caseNumber, err)), nil
		}

		attachments, err := client.GetCaseAttachments(ctx, caseNumber)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get attachments for case %s: %v", caseNumber, err)), nil
		}

		md, err := export.QuickFormat(caseDetail, comments, attachments)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format case: %v", err)), nil
		}

		return mcp.NewToolResultText(md), nil
	}
}

// --- search ---

func searchTool() mcp.Tool {
	return mcp.NewTool("search",
		mcp.WithDescription("Search across Red Hat support cases, solutions, and knowledge base articles."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query string")),
		mcp.WithNumber("limit", mcp.Description("Maximum results per category (default 10)")),
	)
}

func searchHandler(client *api.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := req.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query is required"), nil
		}

		limit := req.GetInt("limit", 10)
		if limit < 1 {
			limit = 10
		}

		var b strings.Builder
		var hasResults bool

		// Search cases
		caseResults, caseErr := client.SearchCases(ctx, query, limit)
		if caseErr == nil && len(caseResults) > 0 {
			hasResults = true
			fmt.Fprintf(&b, "## Cases\n\n")
			for _, r := range caseResults {
				fmt.Fprintf(&b, "- **%s** %s\n", r.ID, r.Title)
				if r.Abstract != "" {
					fmt.Fprintf(&b, "  %s\n", r.Abstract)
				}
			}
			b.WriteString("\n")
		}

		// Search KB (solutions + articles)
		kbResults, kbErr := client.Search(ctx, query, limit)
		if kbErr == nil && len(kbResults) > 0 {
			// Group by type
			var solutions, articles []api.SearchResult
			for _, r := range kbResults {
				switch r.Type {
				case "solution":
					solutions = append(solutions, r)
				default:
					articles = append(articles, r)
				}
			}

			if len(solutions) > 0 {
				hasResults = true
				fmt.Fprintf(&b, "## Solutions\n\n")
				for _, r := range solutions {
					fmt.Fprintf(&b, "- **%s** %s\n", r.ID, r.Title)
					if r.Abstract != "" {
						fmt.Fprintf(&b, "  %s\n", r.Abstract)
					}
				}
				b.WriteString("\n")
			}

			if len(articles) > 0 {
				hasResults = true
				fmt.Fprintf(&b, "## Articles\n\n")
				for _, r := range articles {
					fmt.Fprintf(&b, "- **%s** %s\n", r.ID, r.Title)
					if r.Abstract != "" {
						fmt.Fprintf(&b, "  %s\n", r.Abstract)
					}
				}
				b.WriteString("\n")
			}
		}

		if !hasResults {
			msg := fmt.Sprintf("No results found for %q.", query)
			if caseErr != nil {
				msg += fmt.Sprintf("\nCase search error: %v", caseErr)
			}
			if kbErr != nil {
				msg += fmt.Sprintf("\nKB search error: %v", kbErr)
			}
			return mcp.NewToolResultText(msg), nil
		}

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- get_solution ---

func getSolutionTool() mcp.Tool {
	return mcp.NewTool("get_solution",
		mcp.WithDescription("Get a Red Hat knowledge base solution by ID."),
		mcp.WithString("solution_id", mcp.Required(), mcp.Description("The solution ID")),
	)
}

func getSolutionHandler(client *api.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("solution_id")
		if err != nil {
			return mcp.NewToolResultError("solution_id is required"), nil
		}

		sol, err := client.GetSolution(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get solution %s: %v", id, err)), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "# %s\n\n", sol.Title)
		fmt.Fprintf(&b, "**ID:** %s\n\n", sol.ID)

		if sol.Issue != "" {
			fmt.Fprintf(&b, "## Issue\n\n%s\n\n", export.CleanHTML(sol.Issue))
		}
		if sol.Environment != "" {
			fmt.Fprintf(&b, "## Environment\n\n%s\n\n", export.CleanHTML(sol.Environment))
		}
		if sol.Resolution != "" {
			fmt.Fprintf(&b, "## Resolution\n\n%s\n\n", export.CleanHTML(sol.Resolution))
		}
		if sol.RootCause != "" {
			fmt.Fprintf(&b, "## Root Cause\n\n%s\n\n", export.CleanHTML(sol.RootCause))
		}
		if sol.Body != "" && sol.Issue == "" && sol.Resolution == "" {
			fmt.Fprintf(&b, "%s\n", export.CleanHTML(sol.Body))
		}

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- get_article ---

func getArticleTool() mcp.Tool {
	return mcp.NewTool("get_article",
		mcp.WithDescription("Get a Red Hat knowledge base article by ID."),
		mcp.WithString("article_id", mcp.Required(), mcp.Description("The article ID")),
	)
}

func getArticleHandler(client *api.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("article_id")
		if err != nil {
			return mcp.NewToolResultError("article_id is required"), nil
		}

		article, err := client.GetArticle(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get article %s: %v", id, err)), nil
		}

		var b strings.Builder
		fmt.Fprintf(&b, "# %s\n\n", article.Title)
		fmt.Fprintf(&b, "**ID:** %s\n\n", article.ID)

		if article.Abstract != "" {
			fmt.Fprintf(&b, "## Summary\n\n%s\n\n", export.CleanHTML(article.Abstract))
		}
		if article.Body != "" {
			fmt.Fprintf(&b, "%s\n", export.CleanHTML(article.Body))
		}

		return mcp.NewToolResultText(b.String()), nil
	}
}

// --- export_case ---

func exportCaseTool() mcp.Tool {
	return mcp.NewTool("export_case",
		mcp.WithDescription("Export a Red Hat support case to a complete standalone markdown document with all metadata, comments, and attachment info."),
		mcp.WithString("case_number", mcp.Required(), mcp.Description("The case number to export")),
	)
}

func exportCaseHandler(client *api.Client) server.ToolHandlerFunc {
	return getCaseHandler(client)
}
