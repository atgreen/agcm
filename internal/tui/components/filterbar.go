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
	presetSlot  string
	presetName  string
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

// SetPreset sets the active preset info
func (f *FilterBar) SetPreset(slot, name string) {
	f.presetSlot = slot
	f.presetName = name
}

// ClearPreset clears the preset info
func (f *FilterBar) ClearPreset() {
	f.presetSlot = ""
	f.presetName = ""
}

// Clear removes the filter
func (f *FilterBar) Clear() {
	f.filter = nil
	f.caseCount = 0
	f.totalCount = 0
	f.presetSlot = ""
	f.presetName = ""
}

// SetWidth sets the bar width
func (f *FilterBar) SetWidth(width int) {
	f.width = width
}

// HasActiveFilter returns true if there's an active filter or preset
func (f *FilterBar) HasActiveFilter() bool {
	if f.presetSlot != "" {
		return true
	}
	if f.filter == nil {
		return false
	}

	// Check if any filter is actually set
	return len(f.filter.Accounts) > 0 ||
		len(f.filter.Products) > 0 ||
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

	// Preset indicator - when preset is active, only show preset (not individual filters)
	if f.presetSlot != "" {
		presetStyle := lipgloss.NewStyle().
			Background(lipgloss.Color("33")).
			Foreground(lipgloss.Color("255")).
			Bold(true).
			Padding(0, 1)
		presetLabel := fmt.Sprintf("[%s]", f.presetSlot)
		if f.presetName != "" {
			presetLabel = fmt.Sprintf("[%s] %s", f.presetSlot, f.presetName)
		}
		pills = append(pills, presetStyle.Render(presetLabel))
	} else {
		// Only show individual filter pills when no preset is active
		// Account(s)
		if f.filter != nil && len(f.filter.Accounts) > 0 {
			if len(f.filter.Accounts) == 1 {
				pills = append(pills, f.renderPill("Account", f.filter.Accounts[0]))
			} else {
				pills = append(pills, f.renderPill("Accounts", strings.Join(f.filter.Accounts, ",")))
			}
		}

		// Status
		if f.filter != nil && len(f.filter.Status) > 0 && len(f.filter.Status) < 4 {
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
		if f.filter != nil && len(f.filter.Severity) > 0 && len(f.filter.Severity) < 4 {
			var sevs []string
			for _, s := range f.filter.Severity {
				// Extract just the number
				if len(s) > 0 {
					sevs = append(sevs, string(s[0]))
				}
			}
			pills = append(pills, f.renderPill("Sev", strings.Join(sevs, ",")))
		}

		// Product(s)
		if f.filter != nil && len(f.filter.Products) > 0 {
			if len(f.filter.Products) == 1 {
				prod := f.filter.Products[0]
				if len(prod) > 15 {
					prod = prod[:15] + "..."
				}
				pills = append(pills, f.renderPill("Product", prod))
			} else {
				// Show count for multiple products
				pills = append(pills, f.renderPill("Products", fmt.Sprintf("%d selected", len(f.filter.Products))))
			}
		}

		// Keyword
		if f.filter != nil && f.filter.Keyword != "" {
			kw := f.filter.Keyword
			if len(kw) > 15 {
				kw = kw[:15] + "..."
			}
			pills = append(pills, f.renderPill("Keyword", kw))
		}
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
