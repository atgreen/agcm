// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package components

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/green/agcm/internal/api"
	"github.com/green/agcm/internal/tui/styles"
)

// URL regex pattern
var urlRegex = regexp.MustCompile(`https?://[^\s<>"{}|\\^` + "`" + `\[\]]+`)

// CaseDetail displays detailed information about a case
type CaseDetail struct {
	viewport        viewport.Model
	styles          *styles.Styles
	keys            *styles.KeyMap
	case_           *api.Case
	comments        []api.Comment
	attachments     []api.Attachment
	width           int
	height          int
	focused         bool
	activeTab       int    // 0=details, 1=comments, 2=attachments
	currentComment  int    // Current comment index for n/p navigation
	commentOffsets  []int  // Line offsets for each comment
	searchHighlight string // Current search highlight term
	maskMode        bool   // Mask sensitive text for screenshots
}

// SetMaskMode enables/disables text masking for privacy
func (c *CaseDetail) SetMaskMode(mask bool) {
	c.maskMode = mask
}

// NewCaseDetail creates a new case detail component
func NewCaseDetail(styles *styles.Styles, keys *styles.KeyMap) *CaseDetail {
	vp := viewport.New(0, 0)
	vp.SetContent("")

	return &CaseDetail{
		viewport: vp,
		styles:   styles,
		keys:     keys,
	}
}

// SetCase updates the case being displayed
func (c *CaseDetail) SetCase(cs *api.Case) {
	c.case_ = cs
	c.currentComment = 0
	c.updateContent()
	c.viewport.GotoTop()
}

// SetComments updates the comments
func (c *CaseDetail) SetComments(comments []api.Comment) {
	c.comments = comments
	c.currentComment = 0
	c.updateContent()
}

// SetAttachments updates the attachments
func (c *CaseDetail) SetAttachments(attachments []api.Attachment) {
	c.attachments = attachments
	c.updateContent()
}

// SetSize sets the component dimensions
func (c *CaseDetail) SetSize(width, height int) {
	c.width = width
	c.height = height
	c.viewport.Width = width - 5 // Account for border (2) + space (1) + scrollbar (2)
	c.viewport.Height = height - 4 // Account for border (2) + tabs (1) + separator (1)
	c.updateContent()
}

// SetFocused sets the focus state
func (c *CaseDetail) SetFocused(focused bool) {
	c.focused = focused
}

// IsFocused returns the focus state
func (c *CaseDetail) IsFocused() bool {
	return c.focused
}

// GetCase returns the current case
func (c *CaseDetail) GetCase() *api.Case {
	return c.case_
}

// SetActiveTab sets the active tab by index
func (c *CaseDetail) SetActiveTab(tab int) {
	if tab >= 0 && tab <= 2 {
		c.activeTab = tab
		c.updateContent()
		c.viewport.GotoTop()
	}
}

// ActiveTab returns the current active tab index.
func (c *CaseDetail) ActiveTab() int {
	return c.activeTab
}

// SetSearchHighlight sets the search highlight term
func (c *CaseDetail) SetSearchHighlight(query string) {
	c.searchHighlight = query
	c.updateContent()
}

// ClearSearchHighlight clears the search highlight
func (c *CaseDetail) ClearSearchHighlight() {
	c.searchHighlight = ""
	c.updateContent()
}

// ScrollUp scrolls the viewport up by n lines
func (c *CaseDetail) ScrollUp(n int) {
	c.viewport.SetYOffset(c.viewport.YOffset - n)
}

// ScrollDown scrolls the viewport down by n lines
func (c *CaseDetail) ScrollDown(n int) {
	c.viewport.SetYOffset(c.viewport.YOffset + n)
}

// ScrollToLine scrolls the viewport to show the given line number
func (c *CaseDetail) ScrollToLine(line int) {
	// Center the line in the viewport if possible
	targetOffset := line - c.viewport.Height/2
	if targetOffset < 0 {
		targetOffset = 0
	}
	c.viewport.SetYOffset(targetOffset)
}

// LinkAt returns the URL at the given viewport-relative coordinates, if any.
func (c *CaseDetail) LinkAt(x, y int) (string, bool) {
	if y < 0 || y >= c.viewport.Height {
		return "", false
	}
	lines := strings.Split(c.viewport.View(), "\n")
	if y >= len(lines) {
		return "", false
	}
	line := stripAnsiOSC(lines[y])
	if line == "" {
		return "", false
	}

	byteIdx := columnToByteIndex(line, x)
	if byteIdx < 0 || byteIdx > len(line) {
		return "", false
	}

	matches := urlRegex.FindAllStringIndex(line, -1)
	for _, m := range matches {
		if byteIdx >= m[0] && byteIdx < m[1] {
			return line[m[0]:m[1]], true
		}
	}
	return "", false
}

// ScrollToRelativeLine sets scroll position based on a relative line in the viewport.
func (c *CaseDetail) ScrollToRelativeLine(line int) {
	totalLines := c.viewport.TotalLineCount()
	visibleLines := c.viewport.Height
	if totalLines <= visibleLines || visibleLines <= 1 {
		return
	}
	if line < 0 {
		line = 0
	}
	if line > visibleLines-1 {
		line = visibleLines - 1
	}
	maxScroll := totalLines - visibleLines
	scrollPos := line * maxScroll / (visibleLines - 1)
	c.viewport.SetYOffset(scrollPos)
}

func (c *CaseDetail) updateContent() {
	if c.case_ == nil {
		c.viewport.SetContent("No case selected")
		return
	}

	var content string
	switch c.activeTab {
	case 0:
		content = c.renderDetails()
	case 1:
		content = c.renderComments()
	case 2:
		content = c.renderAttachments()
	}

	c.viewport.SetContent(content)
}

func (c *CaseDetail) renderDetails() string {
	var sb strings.Builder
	cs := c.case_

	// Header
	title := c.styles.Title.Render(fmt.Sprintf("Case %s", cs.CaseNumber))
	sb.WriteString(title)
	sb.WriteString("\n\n")

	// Summary (with highlighting)
	summary := cs.Summary
	if c.maskMode {
		summary = maskText(summary)
	}
	sb.WriteString(c.styles.Label.Render("Summary: "))
	sb.WriteString(c.highlightMatches(summary))
	sb.WriteString("\n\n")

	// Contact and account info (possibly masked)
	contactName := cs.ContactName
	contactEmail := cs.ContactEmail
	accountName := cs.AccountName
	if c.maskMode {
		contactName = maskText(contactName)
		contactEmail = maskText(contactEmail)
		accountName = maskText(accountName)
	}

	// Metadata table
	rows := []struct {
		label string
		value string
		style func(string) string
	}{
		{"Status", cs.Status, func(s string) string { return c.styles.StatusStyle(s).Render(s) }},
		{"Severity", cs.Severity, func(s string) string { return c.styles.SeverityStyle(s).Render(s) }},
		{"Product", fmt.Sprintf("%s %s", cs.Product, cs.Version), nil},
		{"Type", cs.Type, nil},
		{"Created", cs.CreatedDate.Format("2006-01-02 15:04"), nil},
		{"Updated", cs.LastModified.Format("2006-01-02 15:04"), nil},
		{"Contact", fmt.Sprintf("%s (%s)", contactName, contactEmail), nil},
		{"Account", fmt.Sprintf("%s (%s)", accountName, cs.AccountNumber), nil},
	}

	for _, row := range rows {
		sb.WriteString(c.styles.Label.Render(fmt.Sprintf("%-12s", row.label+":")))
		sb.WriteString(" ")
		if row.style != nil {
			sb.WriteString(row.style(row.value))
		} else {
			sb.WriteString(c.styles.Value.Render(row.value))
		}
		sb.WriteString("\n")
	}

	// Description
	sb.WriteString("\n")
	sb.WriteString(c.styles.Subtitle.Render("Description"))
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", min(c.width-6, 60)))
	sb.WriteString("\n")
	// Apply highlighting first, then linkify
	description := cs.Description
	if c.maskMode {
		description = maskText(description)
	}
	description = c.highlightMatches(description)
	sb.WriteString(linkify(description, c.styles.Subtitle))

	return sb.String()
}

func (c *CaseDetail) renderComments() string {
	if len(c.comments) == 0 {
		return c.styles.Muted.Render("No comments")
	}

	var sb strings.Builder
	c.commentOffsets = make([]int, 0, len(c.comments))
	lineCount := 0

	sb.WriteString(c.styles.Subtitle.Render(fmt.Sprintf("Comments (%d)", len(c.comments))))
	sb.WriteString("\n\n")
	lineCount += 2

	indent := "      " // 6-space indent for comment content
	lineWidth := c.width - 10 // Full width for separator lines

	for i, comment := range c.comments {
		// Track comment start position
		c.commentOffsets = append(c.commentOffsets, lineCount)

		// Horizontal separator at start of each comment
		sb.WriteString(c.styles.Muted.Render(strings.Repeat("━", lineWidth)))
		sb.WriteString("\n")
		lineCount++

		// Comment number (bold, on left margin) - oldest comment is #1
		commentNum := len(c.comments) - i
		numStr := c.styles.Title.Render(fmt.Sprintf("#%-3d", commentNum))
		authorName := comment.Author
		if c.maskMode {
			authorName = maskText(authorName)
		}
		author := c.styles.CommentAuthor.Render(authorName)
		date := c.styles.CommentDate.Render(comment.CreatedDate.Format("2006-01-02 15:04"))
		visibility := "Public"
		visStyle := c.styles.Success
		if !comment.IsPublicComment() {
			visibility = "Internal"
			visStyle = c.styles.Muted
		}

		sb.WriteString(fmt.Sprintf("%s %s • %s • %s\n", numStr, author, date, visStyle.Render(visibility)))
		lineCount++

		// Comment text (with highlighting)
		commentText := comment.GetText()
		if c.maskMode {
			commentText = maskText(commentText)
		}
		text := c.highlightMatches(commentText)

		// Indent each line of the comment text
		lines := strings.Split(linkify(text, c.styles.Subtitle), "\n")
		for _, line := range lines {
			sb.WriteString(indent + line + "\n")
			lineCount++
		}

		sb.WriteString("\n")
		lineCount++
	}

	return sb.String()
}

func (c *CaseDetail) renderAttachments() string {
	if len(c.attachments) == 0 {
		return c.styles.Muted.Render("No attachments")
	}

	var sb strings.Builder
	sb.WriteString(c.styles.Subtitle.Render(fmt.Sprintf("Attachments (%d)", len(c.attachments))))
	sb.WriteString("\n\n")

	// Table header (left-aligned, simple spacing)
	sizeCol := 10
	uploadedCol := 10 // YYYY-MM-DD
	gap := 2
	tableWidth := c.width - 6
	if tableWidth < 30 {
		tableWidth = 30
	}
	filenameCol := tableWidth - sizeCol - uploadedCol - (gap * 2)
	if filenameCol < 10 {
		filenameCol = 10
	}
	sb.WriteString(c.styles.Label.Render(fmt.Sprintf("%-*s%*s%-*s%*s%-*s\n",
		filenameCol, "Filename",
		gap, "",
		sizeCol, "Size",
		gap, "",
		uploadedCol, "Uploaded",
	)))
	sep := strings.Repeat("─", filenameCol) + strings.Repeat(" ", gap) +
		strings.Repeat("─", sizeCol) + strings.Repeat(" ", gap) +
		strings.Repeat("─", uploadedCol)
	sb.WriteString(c.styles.Muted.Render(padRightSimple(sep, tableWidth)))
	sb.WriteString("\n")

	for _, att := range c.attachments {
		filename := att.Filename
		if c.maskMode {
			filename = maskText(filename)
		}
		filename = truncateSimple(filename, filenameCol)

		size := "n/a"
		if attSize, ok := attachmentSize(att); ok {
			size = formatSize(attSize)
		}
		date := att.CreatedDate.Format("2006-01-02")

		line := fmt.Sprintf("%-*s%*s%-*s%*s%-*s",
			filenameCol, filename,
			gap, "",
			sizeCol, size,
			gap, "",
			uploadedCol, date,
		)
		sb.WriteString(c.styles.Attachment.Render(line))
		sb.WriteString("\n")
	}

	return sb.String()
}

func attachmentSize(att api.Attachment) (int64, bool) {
	if att.Length > 0 {
		return att.Length, true
	}
	if att.Size > 0 {
		return att.Size, true
	}
	if att.FileSize > 0 {
		return att.FileSize, true
	}
	if att.ContentLength > 0 {
		return att.ContentLength, true
	}
	return 0, false
}

func stripAnsiOSC(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			if i+1 < len(s) && s[i+1] == '[' {
				end := i + 2
				for end < len(s) && (s[end] < 0x40 || s[end] > 0x7e) {
					end++
				}
				if end < len(s) {
					end++
				}
				i = end
				continue
			}
			if i+1 < len(s) && s[i+1] == ']' {
				end := i + 2
				for end < len(s) {
					if s[end] == 0x07 {
						end++
						break
					}
					if s[end] == 0x1b && end+1 < len(s) && s[end+1] == '\\' {
						end += 2
						break
					}
					end++
				}
				i = end
				continue
			}
			i++
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

func columnToByteIndex(s string, col int) int {
	if col <= 0 {
		return 0
	}
	idx := 0
	for _, r := range s {
		if col == 0 {
			break
		}
		idx += utf8.RuneLen(r)
		col--
	}
	if idx > len(s) {
		return len(s)
	}
	return idx
}

func truncateSimple(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if len(s) <= width {
		return s
	}
	if width <= 3 {
		return s[:width]
	}
	return s[:width-3] + "..."
}

func padRightSimple(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// Init implements tea.Model
func (c *CaseDetail) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (c *CaseDetail) Update(msg tea.Msg) (*CaseDetail, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, c.keys.Left):
			if c.activeTab > 0 {
				c.activeTab--
				c.updateContent()
				c.viewport.GotoTop()
			}
		case key.Matches(msg, c.keys.Right):
			if c.activeTab < 2 {
				c.activeTab++
				c.updateContent()
				c.viewport.GotoTop()
			}
		case key.Matches(msg, c.keys.Top):
			c.viewport.GotoTop()
		case key.Matches(msg, c.keys.Bottom):
			c.viewport.GotoBottom()
		case msg.String() == "n":
			// Next comment (relative to current scroll position)
			if c.activeTab == 1 && len(c.commentOffsets) > 0 {
				y := c.viewport.YOffset
				nextIdx := 0
				found := false
				for i, offset := range c.commentOffsets {
					if offset > y {
						nextIdx = i
						found = true
						break
					}
				}
				if !found {
					nextIdx = 0 // Wrap around
				}
				c.currentComment = nextIdx
				c.viewport.SetYOffset(c.commentOffsets[c.currentComment])
			}
		case msg.String() == "p":
			// Previous comment (relative to current scroll position)
			if c.activeTab == 1 && len(c.commentOffsets) > 0 {
				y := c.viewport.YOffset
				prevIdx := len(c.commentOffsets) - 1
				for i := len(c.commentOffsets) - 1; i >= 0; i-- {
					if c.commentOffsets[i] < y {
						prevIdx = i
						break
					}
				}
				c.currentComment = prevIdx
				c.viewport.SetYOffset(c.commentOffsets[c.currentComment])
			}
		default:
			c.viewport, cmd = c.viewport.Update(msg)
		}
	default:
		c.viewport, cmd = c.viewport.Update(msg)
	}

	return c, cmd
}

// View implements tea.Model
func (c *CaseDetail) View() string {
	if c.case_ == nil {
		style := c.styles.Border
		if c.focused {
			style = c.styles.Focused
		}
		height := c.height - 2
		if height < 1 {
			height = 1
		}
		return style.
			Width(c.width).
			Height(height).
			Render(c.styles.Muted.Render("Select a case to view details"))
	}

	var sb strings.Builder

	// Tabs
	tabs := []string{"Details", "Comments", "Attachments"}
	var tabViews []string
	for i, tab := range tabs {
		if i == c.activeTab {
			tabViews = append(tabViews, c.styles.Selected.Render(" "+tab+" "))
		} else {
			tabViews = append(tabViews, c.styles.Muted.Render(" "+tab+" "))
		}
	}
	sb.WriteString(strings.Join(tabViews, " │ "))
	sb.WriteString("\n")
	sb.WriteString(strings.Repeat("─", c.width-2))
	sb.WriteString("\n")

	// Viewport with scrollbar
	viewportContent := c.viewport.View()
	scrollbar := c.renderScrollbar()

	// Combine viewport and scrollbar line by line
	vpLines := strings.Split(viewportContent, "\n")
	sbLines := strings.Split(scrollbar, "\n")

	// Use exactly viewport.Height lines to avoid overflow
	for i := 0; i < c.viewport.Height; i++ {
		vpLine := ""
		sbLine := ""
		if i < len(vpLines) {
			vpLine = vpLines[i]
		}
		if i < len(sbLines) {
			sbLine = sbLines[i]
		}
		// Trim overly long lines to avoid terminal wrapping
		if lipgloss.Width(vpLine) > c.viewport.Width {
			vpLine = ansiCut(vpLine, c.viewport.Width)
		}
		// Pad viewport line to consistent width
		vpLineWidth := lipgloss.Width(vpLine)
		if vpLineWidth < c.viewport.Width {
			vpLine += strings.Repeat(" ", c.viewport.Width-vpLineWidth)
		}
		if i < c.viewport.Height-1 {
			sb.WriteString(vpLine + " " + sbLine + "\n")
		} else {
			sb.WriteString(vpLine + " " + sbLine) // No trailing newline on last line
		}
	}

	style := c.styles.Border
	if c.focused {
		style = c.styles.Focused
	}

	height := c.height - 2
	if height < 1 {
		height = 1
	}
	return style.
		Width(c.width).
		Height(height).
		Render(sb.String())
}

// renderScrollbar renders a vertical scrollbar
func (c *CaseDetail) renderScrollbar() string {
	totalLines := c.viewport.TotalLineCount()
	visibleLines := c.viewport.Height
	scrollPos := c.viewport.YOffset

	if totalLines <= visibleLines {
		// No scrollbar needed, just return empty space
		var lines []string
		for i := 0; i < visibleLines; i++ {
			lines = append(lines, "  ")
		}
		return strings.Join(lines, "\n")
	}

	// Calculate thumb size and position
	thumbSize := max(1, visibleLines*visibleLines/totalLines)
	maxScroll := totalLines - visibleLines
	thumbPos := 0
	if maxScroll > 0 {
		thumbPos = scrollPos * (visibleLines - thumbSize) / maxScroll
	}

	// Use wider, more visible characters
	thumb := c.styles.HelpKey.Render("██")
	track := c.styles.Muted.Render("▒▒")

	var lines []string
	for i := 0; i < visibleLines; i++ {
		if i >= thumbPos && i < thumbPos+thumbSize {
			lines = append(lines, thumb)
		} else {
			lines = append(lines, track)
		}
	}
	return strings.Join(lines, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ansiCut trims a string with ANSI codes to the given visual width.
// It preserves CSI and OSC sequences so hyperlinks don't get broken.
func ansiCut(s string, width int) string {
	if width <= 0 {
		return ""
	}

	var result strings.Builder
	visualPos := 0

	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			if i+1 < len(s) && s[i+1] == '[' {
				// CSI sequence: ESC [ ... final (0x40-0x7E)
				end := i + 2
				for end < len(s) && (s[end] < 0x40 || s[end] > 0x7e) {
					end++
				}
				if end < len(s) {
					end++
				}
				if visualPos < width {
					result.WriteString(s[i:end])
				}
				i = end
				continue
			}
			if i+1 < len(s) && s[i+1] == ']' {
				// OSC sequence: ESC ] ... BEL or ST (ESC \)
				end := i + 2
				for end < len(s) {
					if s[end] == 0x07 {
						end++
						break
					}
					if s[end] == 0x1b && end+1 < len(s) && s[end+1] == '\\' {
						end += 2
						break
					}
					end++
				}
				if visualPos < width {
					result.WriteString(s[i:end])
				}
				i = end
				continue
			}
			// Fallback: include ESC + next byte if present
			if visualPos < width {
				result.WriteByte(s[i])
				if i+1 < len(s) {
					result.WriteByte(s[i+1])
				}
			}
			i += 2
			continue
		}
		if visualPos >= width {
			break
		}
		result.WriteByte(s[i])
		visualPos++
		i++
	}

	return result.String()
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// linkify finds URLs in text and makes them clickable using OSC 8 hyperlinks
func linkify(text string, style lipgloss.Style) string {
	return urlRegex.ReplaceAllStringFunc(text, func(url string) string {
		// OSC 8 hyperlink format: \x1b]8;;URL\x1b\\TEXT\x1b]8;;\x1b\\
		return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", url, style.Render(url))
	})
}

// highlightMatches highlights all occurrences of query in text (case-insensitive)
func (c *CaseDetail) highlightMatches(text string) string {
	if c.searchHighlight == "" {
		return text
	}

	// Create highlight style - bright yellow background with black text
	highlightStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("226")). // Bright yellow
		Foreground(lipgloss.Color("0")).   // Black text
		Bold(true)

	// Case-insensitive replacement
	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(c.searchHighlight)
	queryLen := len(c.searchHighlight)

	if queryLen == 0 {
		return text
	}

	var result strings.Builder
	lastEnd := 0

	for {
		idx := strings.Index(lowerText[lastEnd:], lowerQuery)
		if idx == -1 {
			result.WriteString(text[lastEnd:])
			break
		}

		// Add text before match
		result.WriteString(text[lastEnd : lastEnd+idx])
		// Add highlighted match (preserve original case)
		result.WriteString(highlightStyle.Render(text[lastEnd+idx : lastEnd+idx+queryLen]))
		lastEnd = lastEnd + idx + queryLen
	}

	return result.String()
}
