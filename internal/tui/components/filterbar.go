// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package components

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/green/agcm/internal/api"
	"github.com/green/agcm/internal/tui/styles"
)

// FilterBar shows active filters as pills
type FilterBar struct {
	styles      *styles.Styles
	width       int
	filter      *api.CaseFilter
	caseCount   int
	totalCount  int
}

// NewFilterBar creates a new filter bar
func NewFilterBar(s *styles.Styles) *FilterBar {
	return &FilterBar{
		styles: s,
	}
}

// SetFilter updates the active filter
func (f *FilterBar) SetFilter(filter *api.CaseFilter, caseCount, totalCount int) {
	f.filter = filter
	f.caseCount = caseCount
	f.totalCount = totalCount
}

// Clear removes the filter
func (f *FilterBar) Clear() {
	f.filter = nil
	f.caseCount = 0
	f.totalCount = 0
}

// SetWidth sets the bar width
func (f *FilterBar) SetWidth(width int) {
	f.width = width
}

// HasActiveFilter returns true if there's an active filter
func (f *FilterBar) HasActiveFilter() bool {
	if f.filter == nil {
		return false
	}

	// Check if any filter is actually set
	return f.filter.AccountNumber != "" ||
		f.filter.Product != "" ||
		f.filter.Keyword != "" ||
		len(f.filter.Status) > 0 ||
		len(f.filter.Severity) > 0
}

// Update handles input (not much to do for display-only component)
func (f *FilterBar) Update(msg tea.Msg) (*FilterBar, tea.Cmd) {
	return f, nil
}

func (f *FilterBar) renderPill(label, value string) string {
	pillStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("240")).
		Foreground(lipgloss.Color("255")).
		Padding(0, 1)

	return pillStyle.Render(label + ": " + value)
}

// View renders the filter bar
func (f *FilterBar) View() string {
	if !f.HasActiveFilter() {
		return ""
	}

	var pills []string

	// Account
	if f.filter.AccountNumber != "" {
		pills = append(pills, f.renderPill("Account", f.filter.AccountNumber))
	}

	// Status
	if len(f.filter.Status) > 0 && len(f.filter.Status) < 4 {
		// Abbreviate status names
		var abbrev []string
		for _, s := range f.filter.Status {
			switch s {
			case "Open":
				abbrev = append(abbrev, "Open")
			case "Waiting on Red Hat":
				abbrev = append(abbrev, "WaitRH")
			case "Waiting on Customer":
				abbrev = append(abbrev, "WaitCust")
			case "Closed":
				abbrev = append(abbrev, "Closed")
			default:
				abbrev = append(abbrev, s)
			}
		}
		pills = append(pills, f.renderPill("Status", strings.Join(abbrev, ",")))
	}

	// Severity
	if len(f.filter.Severity) > 0 && len(f.filter.Severity) < 4 {
		var sevs []string
		for _, s := range f.filter.Severity {
			// Extract just the number
			if len(s) > 0 {
				sevs = append(sevs, string(s[0]))
			}
		}
		pills = append(pills, f.renderPill("Sev", strings.Join(sevs, ",")))
	}

	// Product
	if f.filter.Product != "" {
		prod := f.filter.Product
		if len(prod) > 15 {
			prod = prod[:15] + "..."
		}
		pills = append(pills, f.renderPill("Product", prod))
	}

	// Keyword
	if f.filter.Keyword != "" {
		kw := f.filter.Keyword
		if len(kw) > 15 {
			kw = kw[:15] + "..."
		}
		pills = append(pills, f.renderPill("Keyword", kw))
	}

	// Join pills
	pillsStr := strings.Join(pills, " ")

	// Count display
	countStr := ""
	if f.totalCount > 0 {
		countStr = f.styles.Muted.Render(fmt.Sprintf("  Showing %d of %d", f.caseCount, f.totalCount))
	} else if f.caseCount > 0 {
		countStr = f.styles.Muted.Render(fmt.Sprintf("  %d cases", f.caseCount))
	}

	// Clear hint
	clearHint := f.styles.Muted.Render("  [f to edit, F to clear]")

	// Build bar
	content := f.styles.Label.Render("Filters: ") + pillsStr + countStr + clearHint

	// Style the bar
	barStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color("240")).
		Width(f.width).
		Padding(0, 1)

	return barStyle.Render(content)
}
