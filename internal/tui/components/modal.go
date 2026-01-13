// SPDX-License-Identifier: GPL-3.0-or-later
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/green/agcm/internal/tui/styles"
)

// ModalType represents the type of modal
type ModalType int

const (
	ModalNone ModalType = iota
	ModalTextInput
	ModalProgress
)

// Modal is a dialog component
type Modal struct {
	styles      *styles.Styles
	modalType   ModalType
	title       string
	message     string
	textInput   textinput.Model
	progress    float64
	progressMsg string
	width       int
	height      int
	visible     bool
	onConfirm   func(string)
	onCancel    func()
}

// NewModal creates a new modal component
func NewModal(s *styles.Styles) *Modal {
	ti := textinput.New()
	ti.Placeholder = "Enter value..."
	ti.CharLimit = 256
	ti.Width = 40

	return &Modal{
		styles:    s,
		textInput: ti,
	}
}

// ShowTextInput shows a text input modal
func (m *Modal) ShowTextInput(title, message, defaultValue string, onConfirm func(string), onCancel func()) {
	m.modalType = ModalTextInput
	m.title = title
	m.message = message
	m.textInput.SetValue(defaultValue)
	m.textInput.Focus()
	m.onConfirm = onConfirm
	m.onCancel = onCancel
	m.visible = true
}

// ShowProgress shows a progress modal
func (m *Modal) ShowProgress(title, message string) {
	m.modalType = ModalProgress
	m.title = title
	m.progressMsg = message
	m.progress = 0
	m.visible = true
}

// UpdateProgress updates the progress bar
func (m *Modal) UpdateProgress(progress float64, message string) {
	m.progress = progress
	m.progressMsg = message
}

// Hide hides the modal
func (m *Modal) Hide() {
	m.visible = false
	m.modalType = ModalNone
	m.textInput.Blur()
}

// IsVisible returns whether modal is visible
func (m *Modal) IsVisible() bool {
	return m.visible
}

// GetType returns the modal type
func (m *Modal) GetType() ModalType {
	return m.modalType
}

// SetSize sets the modal container size
func (m *Modal) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.textInput.Width = min(40, width-20)
}

// Update handles input
func (m *Modal) Update(msg tea.Msg) (*Modal, tea.Cmd) {
	if !m.visible {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.modalType {
		case ModalTextInput:
			switch msg.String() {
			case "enter":
				if m.onConfirm != nil {
					m.onConfirm(m.textInput.Value())
				}
				m.Hide()
				return m, nil
			case "esc":
				if m.onCancel != nil {
					m.onCancel()
				}
				m.Hide()
				return m, nil
			}
			var cmd tea.Cmd
			m.textInput, cmd = m.textInput.Update(msg)
			return m, cmd

		case ModalProgress:
			if msg.String() == "esc" {
				if m.onCancel != nil {
					m.onCancel()
				}
				m.Hide()
				return m, nil
			}
		}
	}

	return m, nil
}

// View renders the modal
func (m *Modal) View() string {
	if !m.visible {
		return ""
	}

	var content strings.Builder

	// Title
	content.WriteString(m.styles.Title.Render(m.title))
	content.WriteString("\n\n")

	switch m.modalType {
	case ModalTextInput:
		if m.message != "" {
			content.WriteString(m.message)
			content.WriteString("\n\n")
		}
		content.WriteString(m.textInput.View())
		content.WriteString("\n\n")
		content.WriteString(m.styles.Muted.Render("Enter to confirm • Esc to cancel"))

	case ModalProgress:
		content.WriteString(m.progressMsg)
		content.WriteString("\n\n")
		content.WriteString(m.renderProgressBar())
		content.WriteString("\n\n")
		content.WriteString(m.styles.Muted.Render("Esc to cancel"))
	}

	// Modal box style with background
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.Header.GetBackground()).
		Background(lipgloss.Color("248")).
		Padding(1, 3).
		Width(50)

	return boxStyle.Render(content.String())
}

func (m *Modal) renderProgressBar() string {
	width := 40
	filled := int(m.progress * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	percent := int(m.progress * 100)

	return m.styles.HelpKey.Render(bar) + m.styles.Muted.Render(fmt.Sprintf(" %d%%", percent))
}
