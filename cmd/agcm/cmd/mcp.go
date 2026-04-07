// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package cmd

import (
	mcpserver "github.com/green/agcm/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for AI assistant integration",
	Long: `Start a Model Context Protocol (MCP) server over stdio.

This allows AI assistants (Claude, Cursor, etc.) to browse support cases,
search the knowledge base, and export case data.

To use with Claude Code, add to your MCP settings:

  {
    "mcpServers": {
      "agcm": {
        "command": "agcm",
        "args": ["mcp"]
      }
    }
  }

Available tools: list_cases, get_case, search, get_solution, get_article, export_case`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return mcpserver.Run(GetAPIClient(), version)
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
