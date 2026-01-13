// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/green/agcm/internal/api"
	"github.com/green/agcm/internal/tui/styles"
)

// SortField represents the field to sort by
type SortField int

const (
	SortByLastModified SortField = iota
	SortByCreated
	SortBySeverity
	SortByCaseNumber
)

// CaseList is a component for displaying a list of cases
type CaseList struct {
	cases       []api.Case
	cursor      int
	offset      int
	styles      *styles.Styles
	keys        *styles.KeyMap
	width       int
	height      int
	focused     bool
	sortField   SortField
	sortReverse bool
	maskMode    bool
	debugInfo   string
	totalCount  int
}

// SetMaskMode enables/disables text masking for privacy
func (c *CaseList) SetMaskMode(mask bool) {
	c.maskMode = mask
}

// maskText replaces letters and digits with asterisks
func maskText(s string) string {
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			result.WriteRune('*')
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// NewCaseList creates a new case list component
func NewCaseList(s *styles.Styles, keys *styles.KeyMap) *CaseList {
	return &CaseList{
		styles: s,
		keys:   keys,
	}
}

// SetCases updates the list with new cases
func (c *CaseList) SetCases(cases []api.Case) {
	c.cases = cases
	c.cursor = 0
	c.offset = 0
}

// SelectedCase returns the currently selected case
func (c *CaseList) SelectedCase() *api.Case {
	if c.cursor >= 0 && c.cursor < len(c.cases) {
		return &c.cases[c.cursor]
	}
	return nil
}

// SetSize sets the component dimensions
func (c *CaseList) SetSize(width, height int) {
	c.width = width
	c.height = height
}

// SetFocused sets the focus state
func (c *CaseList) SetFocused(focused bool) {
	c.focused = focused
}

// IsFocused returns the focus state
func (c *CaseList) IsFocused() bool {
	return c.focused
}

// SetSort sets the current sort field and direction
func (c *CaseList) SetSort(field SortField, reverse bool) {
	c.sortField = field
	c.sortReverse = reverse
}

// SetTotalCount sets the total case count for scrollbar sizing.
func (c *CaseList) SetTotalCount(total int) {
	c.totalCount = total
}

// SetDebugInfo sets a debug string to show in the separator line when enabled.
func (c *CaseList) SetDebugInfo(info string) {
	c.debugInfo = info
}

// GetOffset returns the current scroll offset
func (c *CaseList) GetOffset() int {
	return c.offset
}

// SetOffset sets the scroll offset, clamped to valid range.
func (c *CaseList) SetOffset(offset int) {
	maxOffset := len(c.cases) - c.visibleRows()
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	c.offset = offset
}

// SetCursor sets the cursor position and ensures visibility
func (c *CaseList) SetCursor(idx int) {
	if idx >= 0 && idx < len(c.cases) {
		c.cursor = idx
		c.ensureVisible()
	}
}

// ScrollUp moves cursor up by n rows
func (c *CaseList) ScrollUp(n int) {
	c.cursor -= n
	if c.cursor < 0 {
		c.cursor = 0
	}
	c.ensureVisible()
}

// ScrollDown moves cursor down by n rows
func (c *CaseList) ScrollDown(n int) {
	c.cursor += n
	if c.cursor >= len(c.cases) {
		c.cursor = len(c.cases) - 1
	}
	if c.cursor < 0 {
		c.cursor = 0
	}
	c.ensureVisible()
}

// ScrollToRelativeLine sets scroll position based on a relative line in the list area.
func (c *CaseList) ScrollToRelativeLine(line int) {
	// Use loaded cases only - scrollbar represents currently available content
	totalRows := len(c.cases)
	visibleRows := c.visibleRows()
	if totalRows <= visibleRows || visibleRows <= 0 {
		return
	}
	innerHeight := c.height - 2
	if innerHeight <= 0 {
		return
	}

	areaTop := 2 // header + separator
	areaHeight := innerHeight - areaTop
	if areaHeight < 1 {
		areaTop = 0
		areaHeight = innerHeight
	}

	// Calculate thumb size to match renderScrollbar
	thumbSize := 1
	if areaHeight > 0 && totalRows > 0 {
		thumbSize = maxInt(1, areaHeight*visibleRows/totalRows)
	}
	trackHeight := areaHeight - thumbSize
	if trackHeight < 1 {
		trackHeight = 1
	}

	// Clamp line to valid range
	if line < areaTop {
		line = areaTop
	}
	if line > areaTop+trackHeight {
		line = areaTop + trackHeight
	}

	maxScroll := totalRows - visibleRows
	if maxScroll <= 0 {
		c.offset = 0
		return
	}

	// Calculate offset from relative position in track
	rel := line - areaTop
	if rel < 0 {
		rel = 0
	}
	offset := 0
	if trackHeight > 0 {
		offset = rel * maxScroll / trackHeight
	}
	if offset < 0 {
		offset = 0
	}
	if offset > maxScroll {
		offset = maxScroll
	}
	c.offset = offset
}

// visibleRows returns how many case rows can be displayed
func (c *CaseList) visibleRows() int {
	// Account for border (2), header row (1), separator (1)
	rows := c.height - 4
	if rows < 1 {
		rows = 1
	}
	return rows
}

// VisibleRows exposes how many case rows can be displayed.
func (c *CaseList) VisibleRows() int {
	return c.visibleRows()
}

// ensureVisible adjusts offset to keep cursor visible
func (c *CaseList) ensureVisible() {
	visible := c.visibleRows()
	if visible <= 0 {
		return
	}

	if c.cursor < c.offset {
		c.offset = c.cursor
	} else if c.cursor >= c.offset+visible {
		c.offset = c.cursor - visible + 1
	}
}

// Update implements tea.Model
func (c *CaseList) Update(msg tea.Msg) (*CaseList, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, c.keys.Up):
			if c.cursor > 0 {
				c.cursor--
				c.ensureVisible()
			}
		case key.Matches(msg, c.keys.Down):
			if c.cursor < len(c.cases)-1 {
				c.cursor++
				c.ensureVisible()
			}
		case key.Matches(msg, c.keys.Top):
			c.cursor = 0
			c.offset = 0
		case key.Matches(msg, c.keys.Bottom):
			c.cursor = len(c.cases) - 1
			c.ensureVisible()
		case msg.String() == "pgup":
			c.cursor -= c.visibleRows()
			if c.cursor < 0 {
				c.cursor = 0
			}
			c.ensureVisible()
		case msg.String() == "pgdown":
			c.cursor += c.visibleRows()
			if c.cursor >= len(c.cases) {
				c.cursor = len(c.cases) - 1
			}
			c.ensureVisible()
		}
	}

	return c, nil
}

// View implements tea.Model
func (c *CaseList) View() string {
	style := c.styles.Border
	if c.focused {
		style = c.styles.Focused
	}

	// Calculate available width for content (minus border + scrollbar)
	contentWidth := c.width - 5
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Build header
	header := c.renderHeader(contentWidth)

	// Build rows
	var rows []string
	rows = append(rows, header)
	rows = append(rows, c.renderSeparator(contentWidth))

	visible := c.visibleRows()
	if len(c.cases) == 0 {
		rows = append(rows, c.styles.Muted.Render("  No cases loaded"))
	} else {
		for i := c.offset; i < len(c.cases) && i < c.offset+visible; i++ {
			selected := i == c.cursor
			rows = append(rows, c.renderRow(&c.cases[i], contentWidth, selected))
		}
	}

	// Build scrollbar for list rows
	innerHeight := c.height - 2
	scrollbar := c.renderScrollbar(innerHeight)
	scrollLines := strings.Split(scrollbar, "\n")

	// Normalize line widths and combine with scrollbar
	var outLines []string
	for i, line := range rows {
		if lipgloss.Width(line) > contentWidth {
			line = ansiCutWidth(line, contentWidth)
		}
		if lipgloss.Width(line) < contentWidth {
			line = padRight(line, contentWidth)
		}
		sbLine := "  "
		if i < len(scrollLines) {
			sbLine = scrollLines[i]
		}
		outLines = append(outLines, line+" "+sbLine)
	}
	// Pad remaining inner height (if any)
	for len(outLines) < innerHeight {
		outLines = append(outLines, padRight("", contentWidth)+"  ")
	}

	content := strings.Join(outLines, "\n")

	// Apply border style with explicit height to ensure exact sizing
	height := c.height - 2
	if height < 1 {
		height = 1
	}
	return style.
		Width(c.width).
		Height(height).
		Render(content)
}

// Column widths (fixed)
const (
	colCase   = 10
	colDate   = 12
	colStatus = 20
	colSev    = 4
)

func (c *CaseList) renderHeader(width int) string {
	// Sort arrow
	arrow := "▲"
	if c.sortReverse {
		arrow = "▼"
	}

	// Build each column with proper padding (pad first, then style)
	var caseHdr, modHdr, sevHdr, statusHdr string

	switch c.sortField {
	case SortByCaseNumber:
		caseHdr = c.styles.HelpKey.Render(padRight("CASE"+arrow, colCase))
		modHdr = c.styles.Label.Render(padRight("MODIFIED", colDate))
		sevHdr = c.styles.Label.Render(padRight("SEV", colSev))
	case SortByLastModified:
		caseHdr = c.styles.Label.Render(padRight("CASE", colCase))
		modHdr = c.styles.HelpKey.Render(padRight("MODIFIED"+arrow, colDate))
		sevHdr = c.styles.Label.Render(padRight("SEV", colSev))
	case SortByCreated:
		caseHdr = c.styles.Label.Render(padRight("CASE", colCase))
		modHdr = c.styles.HelpKey.Render(padRight("CREATED"+arrow, colDate))
		sevHdr = c.styles.Label.Render(padRight("SEV", colSev))
	case SortBySeverity:
		caseHdr = c.styles.Label.Render(padRight("CASE", colCase))
		modHdr = c.styles.Label.Render(padRight("MODIFIED", colDate))
		sevHdr = c.styles.HelpKey.Render(padRight("SEV"+arrow, colSev))
	default:
		caseHdr = c.styles.Label.Render(padRight("CASE", colCase))
		modHdr = c.styles.Label.Render(padRight("MODIFIED", colDate))
		sevHdr = c.styles.Label.Render(padRight("SEV", colSev))
	}

	statusHdr = c.styles.Label.Render(padRight("STATUS", colStatus))
	summaryHdr := c.styles.Label.Render("SUMMARY")

	return caseHdr + " " + modHdr + " " + sevHdr + " " + statusHdr + " " + summaryHdr
}

func padRight(s string, width int) string {
	if lipgloss.Width(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-lipgloss.Width(s))
}

func trimWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if len(s) <= width {
		return s
	}
	return s[:width]
}

func ansiCutWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	result := ""
	visualPos := 0
	inEscape := false

	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			if visualPos < width {
				result += string(r)
			}
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			if visualPos < width {
				result += string(r)
			}
			continue
		}
		if visualPos >= width {
			break
		}
		result += string(r)
		visualPos++
	}

	return result
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (c *CaseList) renderSeparator(width int) string {
	if c.debugInfo != "" {
		text := c.debugInfo
		if lipgloss.Width(text) > width {
			text = trimWidth(text, width)
		}
		return c.styles.Muted.Render(padRight(text, width))
	}
	return c.styles.Muted.Render(strings.Repeat("─", width))
}

// renderScrollbar renders a vertical scrollbar aligned to list rows.
func (c *CaseList) renderScrollbar(height int) string {
	if height < 1 {
		return ""
	}

	// Use loaded cases only - scrollbar represents currently available content
	totalRows := len(c.cases)
	visibleRows := c.visibleRows()
	if totalRows <= visibleRows || visibleRows <= 0 {
		return strings.Repeat("  \n", height-1) + "  "
	}

	areaTop := 2 // header + separator
	areaHeight := height - areaTop
	if areaHeight < 1 {
		areaTop = 0
		areaHeight = height
	}

	thumbSize := maxInt(1, areaHeight*visibleRows/totalRows)
	maxScroll := totalRows - visibleRows
	scrollPos := c.offset
	thumbPos := 0
	if maxScroll > 0 {
		thumbPos = scrollPos * (areaHeight - thumbSize) / maxScroll
	}

	thumb := c.styles.HelpKey.Render("██")
	track := c.styles.Muted.Render("▒▒")

	lines := make([]string, 0, height)
	for i := 0; i < height; i++ {
		if i < areaTop {
			lines = append(lines, "  ")
			continue
		}
		areaIdx := i - areaTop
		if areaIdx >= thumbPos && areaIdx < thumbPos+thumbSize {
			lines = append(lines, thumb)
		} else {
			lines = append(lines, track)
		}
	}

	return strings.Join(lines, "\n")
}

func (c *CaseList) renderRow(cs *api.Case, width int, selected bool) string {
	// Format date
	date := cs.LastModified.Format("Jan 02 2006")

	// Extract severity number
	sev := cs.Severity
	if len(sev) > 0 {
		sev = string(sev[0])
	}

	// Truncate status if needed
	status := cs.Status
	if len(status) > colStatus {
		status = status[:colStatus-1] + "…"
	}

	// Calculate remaining width for summary
	descWidth := width - colCase - colDate - colStatus - colSev - 4
	if descWidth < 10 {
		descWidth = 10
	}

	summary := cs.Summary
	if c.maskMode {
		summary = maskText(summary)
	}
	if len(summary) > descWidth {
		summary = summary[:descWidth-1] + "…"
	}

	// Build the row
	row := fmt.Sprintf("%-*s %-*s %-*s %-*s %s",
		colCase, cs.CaseNumber,
		colDate, date,
		colSev, sev,
		colStatus, status,
		summary,
	)

	if selected {
		return c.styles.ListItemSelected.Width(width).Render(row)
	}

	// Apply severity color to the severity column only
	caseNum := c.styles.CaseNumber.Render(fmt.Sprintf("%-*s", colCase, cs.CaseNumber))
	dateStr := fmt.Sprintf("%-*s", colDate, date)
	sevStr := c.styles.SeverityStyle(cs.Severity).Render(fmt.Sprintf("%-*s", colSev, sev))
	statusStr := c.styles.StatusStyle(cs.Status).Render(fmt.Sprintf("%-*s", colStatus, status))

	return fmt.Sprintf("%s %s %s %s %s", caseNum, dateStr, sevStr, statusStr, summary)
}
