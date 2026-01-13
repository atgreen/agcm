package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/green/agcm/internal/tui/styles"
)

// StatusBar displays status information at the bottom of the screen
type StatusBar struct {
	styles     *styles.Styles
	width      int
	connected  bool
	message    string
	messageExp time.Time
	loading    bool
	loadingMsg string
	spinner    spinner.Model
}

// NewStatusBar creates a new status bar component
func NewStatusBar(s *styles.Styles) *StatusBar {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(s.Warning.GetForeground())

	return &StatusBar{
		styles:    s,
		connected: false,
		spinner:   sp,
	}
}

// SetWidth sets the component width
func (s *StatusBar) SetWidth(width int) {
	s.width = width
}

// SetConnected sets the connection status
func (s *StatusBar) SetConnected(connected bool) {
	s.connected = connected
}

// SetMessage sets a temporary message
func (s *StatusBar) SetMessage(msg string, duration time.Duration) {
	s.message = msg
	s.messageExp = time.Now().Add(duration)
}

// SetLoading sets the loading state
func (s *StatusBar) SetLoading(loading bool, msg string) {
	s.loading = loading
	s.loadingMsg = msg
}

// SpinnerTick returns the spinner tick command to kick-start animation
func (s *StatusBar) SpinnerTick() tea.Cmd {
	return s.spinner.Tick
}

// Init returns spinner tick command
func (s *StatusBar) Init() tea.Cmd {
	return s.spinner.Tick
}

// Update implements tea.Model
func (s *StatusBar) Update(msg tea.Msg) (*StatusBar, tea.Cmd) {
	var cmd tea.Cmd

	// Clear expired messages
	if s.message != "" && time.Now().After(s.messageExp) {
		s.message = ""
	}

	// Always update spinner to keep it ticking when loading
	s.spinner, cmd = s.spinner.Update(msg)

	// Only return the tick command if we're actually loading
	if s.loading {
		return s, cmd
	}

	return s, nil
}

// View implements tea.Model
func (s *StatusBar) View() string {
	var left, center, right string

	// Left: Help shortcuts (updated for current keybindings)
	shortcuts := []struct {
		key  string
		desc string
	}{
		{"s", "Sort"},
		{"r", "Refresh"},
		{"?", "Help"},
		{"q", "Quit"},
	}

	var parts []string
	for _, sc := range shortcuts {
		parts = append(parts, fmt.Sprintf("%s %s",
			s.styles.HelpKey.Render("["+sc.key+"]"),
			s.styles.HelpDesc.Render(sc.desc)))
	}
	left = strings.Join(parts, "  ")

	// Center: Message or loading with spinner
	if s.loading {
		center = s.spinner.View() + " " + s.styles.Warning.Render(s.loadingMsg)
	} else if s.message != "" {
		center = s.message
	}

	// Right: Connection status and time
	var connStatus string
	if s.connected {
		connStatus = s.styles.Success.Render("● connected")
	} else {
		connStatus = s.styles.Muted.Render("○ connecting...")
	}
	timeStr := time.Now().Format("15:04")
	right = fmt.Sprintf("%s  %s", connStatus, s.styles.Muted.Render(timeStr))

	// Calculate spacing
	leftLen := len(stripAnsi(left))
	centerLen := len(stripAnsi(center))
	rightLen := len(stripAnsi(right))

	totalLen := leftLen + centerLen + rightLen
	if totalLen >= s.width {
		// Just show left and right
		padding := s.width - leftLen - rightLen
		if padding < 0 {
			padding = 0
		}
		return s.styles.StatusBar.
			Width(s.width).
			Render(left + strings.Repeat(" ", padding) + right)
	}

	// Distribute space
	leftPadding := (s.width - centerLen) / 2 - leftLen
	if leftPadding < 1 {
		leftPadding = 1
	}
	rightPadding := s.width - leftLen - leftPadding - centerLen - rightLen
	if rightPadding < 1 {
		rightPadding = 1
	}

	content := left + strings.Repeat(" ", leftPadding) + center + strings.Repeat(" ", rightPadding) + right

	return s.styles.StatusBar.
		Width(s.width).
		Render(content)
}

// stripAnsi removes ANSI escape codes for length calculation
func stripAnsi(s string) string {
	var result strings.Builder
	inEscape := false

	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		result.WriteRune(r)
	}

	return result.String()
}
