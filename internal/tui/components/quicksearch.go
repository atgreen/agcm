package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/green/agcm/internal/tui/styles"
)

// QuickSearchSubmitMsg is sent when user submits a search
type QuickSearchSubmitMsg struct {
	CaseNumber string
}

// QuickSearchCancelMsg is sent when user cancels search
type QuickSearchCancelMsg struct{}

// QuickSearch is a simple modal for searching by case number
type QuickSearch struct {
	styles    *styles.Styles
	textInput textinput.Model
	width     int
	height    int
	visible   bool
}

// NewQuickSearch creates a new quick search component
func NewQuickSearch(s *styles.Styles) *QuickSearch {
	ti := textinput.New()
	ti.Placeholder = "e.g. 03789456"
	ti.CharLimit = 20
	ti.Width = 20

	return &QuickSearch{
		styles:    s,
		textInput: ti,
	}
}

// Show displays the quick search modal
func (q *QuickSearch) Show() tea.Cmd {
	q.visible = true
	q.textInput.SetValue("")
	q.textInput.Focus()
	return textinput.Blink
}

// Hide hides the quick search modal
func (q *QuickSearch) Hide() {
	q.visible = false
	q.textInput.Blur()
}

// IsVisible returns whether the modal is visible
func (q *QuickSearch) IsVisible() bool {
	return q.visible
}

// SetSize sets the container size for centering
func (q *QuickSearch) SetSize(width, height int) {
	q.width = width
	q.height = height
}

// Update handles input
func (q *QuickSearch) Update(msg tea.Msg) (*QuickSearch, tea.Cmd) {
	if !q.visible {
		return q, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			value := strings.TrimSpace(q.textInput.Value())
			q.Hide()
			if value != "" {
				return q, func() tea.Msg {
					return QuickSearchSubmitMsg{CaseNumber: value}
				}
			}
			return q, nil
		case "esc":
			q.Hide()
			return q, func() tea.Msg {
				return QuickSearchCancelMsg{}
			}
		}
	}

	var cmd tea.Cmd
	q.textInput, cmd = q.textInput.Update(msg)
	return q, cmd
}

// View renders the quick search modal
func (q *QuickSearch) View() string {
	if !q.visible {
		return ""
	}

	var content strings.Builder

	// Title
	content.WriteString(q.styles.Title.Render("Search Case Number"))
	content.WriteString("\n\n")

	// Input field
	content.WriteString(q.textInput.View())
	content.WriteString("\n\n")

	// Help text
	content.WriteString(q.styles.Muted.Render("Enter: Search • Esc: Cancel"))

	// Modal box style
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(q.styles.Header.GetBackground()).
		Background(lipgloss.Color("248")).
		Padding(1, 3).
		Width(40)

	return boxStyle.Render(content.String())
}
