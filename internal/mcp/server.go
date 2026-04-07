// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package mcp

import (
	"github.com/green/agcm/internal/api"
	"github.com/mark3labs/mcp-go/server"
)

// Run creates and starts the MCP server over stdio.
func Run(client *api.Client, version string) error {
	s := server.NewMCPServer("agcm", version)

	registerTools(s, client)

	return server.ServeStdio(s)
}
