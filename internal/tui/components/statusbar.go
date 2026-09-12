// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
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
	keys       *styles.KeyMap
	width      int
	connected  bool
	message    string
	messageExp time.Time
	loading    bool
	loadingMsg string
	spinner    spinner.Model
}

// NewStatusBar creates a new status bar component; the shortcut hints are
// rendered from the KeyMap's ShortHelp so they can't drift from the bindings
func NewStatusBar(s *styles.Styles, keys *styles.KeyMap) *StatusBar {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(s.Warning.GetForeground())

	return &StatusBar{
		styles:    s,
		keys:      keys,
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

	// Left: shortcut hints from the keymap
	var parts []string
	for _, b := range s.keys.ShortHelp() {
		h := b.Help()
		parts = append(parts, fmt.Sprintf("%s %s",
			s.styles.HelpKey.Render("["+h.Key+"]"),
			s.styles.HelpDesc.Render(h.Desc)))
	}
	left = strings.Join(parts, "  ")

	// Center: Message or loading with spinner
	if s.loading {
		// Restyle each render so the spinner follows theme changes
		s.spinner.Style = lipgloss.NewStyle().Foreground(s.styles.Warning.GetForeground())
		center = s.spinner.View() + " " + s.styles.Warning.Render(s.loadingMsg)
	} else if s.message != "" {
		center = s.message
	}

	// Right: Connection status
	if s.connected {
		right = s.styles.Success.Render("● connected")
	} else {
		right = s.styles.Muted.Render("○ offline")
	}

	// Calculate spacing
	leftLen := len(stripANSI(left))
	centerLen := len(stripANSI(center))
	rightLen := len(stripANSI(right))

	totalLen := leftLen + centerLen + rightLen
	if totalLen >= s.width {
		// Just show left and right, truncated to fit
		padding := s.width - leftLen - rightLen
		if padding < 0 {
			padding = 0
		}
		return s.styles.StatusBar.
			Width(s.width).
			Render(ansiCutWidth(left+strings.Repeat(" ", padding)+right, s.width))
	}

	// Distribute space
	leftPadding := (s.width-centerLen)/2 - leftLen
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
		Render(ansiCutWidth(content, s.width))
}

