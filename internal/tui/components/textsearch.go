package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/green/agcm/internal/tui/styles"
)

// TextSearchCloseMsg is sent when text search is closed
type TextSearchCloseMsg struct{}

// TextSearchQueryMsg is sent when search query changes
type TextSearchQueryMsg struct {
	Query string
}

// TextMatch represents a match in the text
type TextMatch struct {
	TabIndex   int    // 0=details, 1=comments, 2=attachments
	LineNumber int    // Line number in content
	Text       string // The line containing the match
}

// TextSearch is a search bar for searching within case content
type TextSearch struct {
	styles    *styles.Styles
	textInput textinput.Model
	width     int
	visible   bool
	query     string
	matches   []TextMatch
	current   int // Current match index
}

// NewTextSearch creates a new text search component
func NewTextSearch(s *styles.Styles) *TextSearch {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.CharLimit = 100
	ti.Width = 30

	return &TextSearch{
		styles:    s,
		textInput: ti,
	}
}

// Show displays the search bar
func (t *TextSearch) Show() tea.Cmd {
	t.visible = true
	t.textInput.SetValue("")
	t.textInput.Focus()
	t.query = ""
	t.matches = nil
	t.current = 0
	return textinput.Blink
}

// Hide hides the search bar
func (t *TextSearch) Hide() {
	t.visible = false
	t.textInput.Blur()
	t.query = ""
	t.matches = nil
	t.current = 0
}

// IsVisible returns whether the search bar is visible
func (t *TextSearch) IsVisible() bool {
	return t.visible
}

// SetWidth sets the bar width
func (t *TextSearch) SetWidth(width int) {
	t.width = width
	t.textInput.Width = min(40, width-30)
}

// GetQuery returns the current search query
func (t *TextSearch) GetQuery() string {
	return t.query
}

// SetMatches sets the search results
func (t *TextSearch) SetMatches(matches []TextMatch) {
	t.matches = matches
	t.current = 0
}

// GetCurrentMatch returns the current match or nil
func (t *TextSearch) GetCurrentMatch() *TextMatch {
	if len(t.matches) == 0 || t.current >= len(t.matches) {
		return nil
	}
	return &t.matches[t.current]
}

// NextMatch moves to the next match
func (t *TextSearch) NextMatch() *TextMatch {
	if len(t.matches) == 0 {
		return nil
	}
	t.current = (t.current + 1) % len(t.matches)
	return &t.matches[t.current]
}

// PrevMatch moves to the previous match
func (t *TextSearch) PrevMatch() *TextMatch {
	if len(t.matches) == 0 {
		return nil
	}
	t.current = (t.current - 1 + len(t.matches)) % len(t.matches)
	return &t.matches[t.current]
}

// Update handles input
func (t *TextSearch) Update(msg tea.Msg) (*TextSearch, tea.Cmd) {
	if !t.visible {
		return t, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			t.Hide()
			return t, func() tea.Msg { return TextSearchCloseMsg{} }
		case "enter":
			// Move to next match on enter
			t.NextMatch()
			return t, nil
		case "ctrl+n":
			t.NextMatch()
			return t, nil
		case "ctrl+p":
			t.PrevMatch()
			return t, nil
		}
	}

	// Update text input
	var cmd tea.Cmd
	oldValue := t.textInput.Value()
	t.textInput, cmd = t.textInput.Update(msg)
	newValue := t.textInput.Value()

	// If query changed, emit a message
	if newValue != oldValue {
		t.query = newValue
		return t, tea.Batch(cmd, func() tea.Msg {
			return TextSearchQueryMsg{Query: newValue}
		})
	}

	return t, cmd
}

// View renders the search bar
func (t *TextSearch) View() string {
	if !t.visible {
		return ""
	}

	var content strings.Builder

	// Search icon and input
	content.WriteString(t.styles.Label.Render("Find: "))
	content.WriteString(t.textInput.View())

	// Match count
	if t.query != "" {
		if len(t.matches) > 0 {
			content.WriteString(t.styles.Muted.Render("  "))
			content.WriteString(t.styles.Success.Render(
				strings.Repeat(" ", 2) + // spacing
					string(rune('0'+((t.current+1)/10)%10)) + string(rune('0'+(t.current+1)%10)) + "/" +
					string(rune('0'+(len(t.matches)/10)%10)) + string(rune('0'+len(t.matches)%10)),
			))
		} else {
			content.WriteString(t.styles.Muted.Render("  No matches"))
		}
	}

	// Help text
	content.WriteString(t.styles.Muted.Render("  Enter/Ctrl+N: Next  Ctrl+P: Prev  Esc: Close"))

	// Bar style
	barStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(lipgloss.Color("240")).
		Width(t.width - 4).
		Padding(0, 1)

	return barStyle.Render(content.String())
}
