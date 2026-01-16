// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package components

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/green/agcm/internal/tui/styles"
)


// formatFilePickerOutput post-processes filepicker output to add full-width backgrounds
func formatFilePickerOutput(output string, width int) string {
	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		return output
	}

	cursorBg := lipgloss.NewStyle().Background(lipgloss.Color("238"))

	var result []string
	for _, line := range lines {
		if line == "" {
			result = append(result, line)
			continue
		}

		// Check if this is the cursor line (starts with > after stripping ANSI)
		plainLine := stripAnsi(line)
		isCursor := strings.HasPrefix(plainLine, ">")

		// Pad the line to full width
		lineLen := lipgloss.Width(line)
		padding := ""
		if lineLen < width {
			padding = strings.Repeat(" ", width-lineLen)
		}
		paddedLine := line + padding

		if isCursor {
			result = append(result, cursorBg.Render(paddedLine))
		} else {
			result = append(result, paddedLine)
		}
	}

	return strings.Join(result, "\n")
}

// FilePickerMode represents the mode of the file picker dialog
type FilePickerMode int

const (
	FilePickerModeFile FilePickerMode = iota // Select/create a file
	FilePickerModeDir                        // Select a directory
)

// FilePickerDialog is a modal file picker dialog
type FilePickerDialog struct {
	styles       *styles.Styles
	filepicker   filepicker.Model
	textInput    textinput.Model
	title        string
	message      string
	mode         FilePickerMode
	width        int
	height       int
	visible      bool
	showInput    bool // Toggle between filepicker and text input
	selectedPath string
	onConfirm    func(string)
	onCancel     func()
}

// NewFilePickerDialog creates a new file picker dialog
func NewFilePickerDialog(s *styles.Styles) *FilePickerDialog {
	fp := filepicker.New()
	fp.CurrentDirectory, _ = filepath.Abs(".")
	fp.ShowHidden = false
	fp.ShowPermissions = false
	fp.ShowSize = true
	fp.SetHeight(15)

	// Style the filepicker for dark background
	fp.Styles.Cursor = lipgloss.NewStyle().Foreground(lipgloss.Color("212")) // Pink cursor
	fp.Styles.Symlink = lipgloss.NewStyle().Foreground(lipgloss.Color("36")) // Cyan
	fp.Styles.Directory = lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true) // Blue, bold
	fp.Styles.File = lipgloss.NewStyle().Foreground(lipgloss.Color("252")) // Light gray
	fp.Styles.Permission = lipgloss.NewStyle().Foreground(lipgloss.Color("244")) // Gray
	fp.Styles.Selected = lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Bold(true) // Pink, bold
	fp.Styles.FileSize = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Width(7).Align(lipgloss.Right) // Dim gray, right-aligned
	fp.Styles.EmptyDirectory = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true)

	ti := textinput.New()
	ti.Placeholder = "Enter path..."
	ti.CharLimit = 256
	ti.Width = 50
	ti.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))       // Blue prompt
	ti.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))        // White text
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Dim placeholder
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))     // Pink cursor

	return &FilePickerDialog{
		styles:     s,
		filepicker: fp,
		textInput:  ti,
	}
}

// Show displays the file picker dialog
func (f *FilePickerDialog) Show(title, message string, mode FilePickerMode, defaultPath string, onConfirm func(string), onCancel func()) tea.Cmd {
	f.title = title
	f.message = message
	f.mode = mode
	f.visible = true
	f.onConfirm = onConfirm
	f.onCancel = onCancel

	// Configure filepicker based on mode
	if mode == FilePickerModeDir {
		f.filepicker.DirAllowed = true
		f.filepicker.FileAllowed = false
		f.showInput = false // Start in directory browser mode
	} else {
		f.filepicker.DirAllowed = true // Allow entering directories to navigate
		f.filepicker.FileAllowed = true
		f.filepicker.AllowedTypes = []string{".md"}
		f.showInput = true // Start in text input mode for new file creation
		f.textInput.Focus()
	}

	// Set starting directory
	if defaultPath != "" {
		dir := filepath.Dir(defaultPath)
		if dir == "." {
			dir, _ = filepath.Abs(".")
		}
		if absDir, err := filepath.Abs(dir); err == nil {
			f.filepicker.CurrentDirectory = absDir
		}
		f.textInput.SetValue(defaultPath)
		f.selectedPath = defaultPath
	} else {
		cwd, _ := filepath.Abs(".")
		f.filepicker.CurrentDirectory = cwd
		f.textInput.SetValue("")
		f.selectedPath = ""
	}

	return f.filepicker.Init()
}

// Hide hides the dialog
func (f *FilePickerDialog) Hide() {
	f.visible = false
	f.textInput.Blur()
}

// IsVisible returns whether dialog is visible
func (f *FilePickerDialog) IsVisible() bool {
	return f.visible
}

// SetSize sets the dialog container size
func (f *FilePickerDialog) SetSize(width, height int) {
	f.width = width
	f.height = height
	f.filepicker.SetHeight(min(15, height-12))
	f.textInput.Width = min(50, width-20)
}

// Update handles input
func (f *FilePickerDialog) Update(msg tea.Msg) (*FilePickerDialog, tea.Cmd) {
	if !f.visible {
		return f, nil
	}

	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			// Toggle between filepicker and text input mode
			f.showInput = !f.showInput
			if f.showInput {
				f.textInput.Focus()
				// Pre-fill with current directory
				currentVal := f.textInput.Value()
				if currentVal == "" || filepath.Dir(currentVal) == "." {
					f.textInput.SetValue(f.filepicker.CurrentDirectory + "/")
				}
			} else {
				f.textInput.Blur()
			}
			return f, nil

		case "esc":
			if f.onCancel != nil {
				f.onCancel()
			}
			f.Hide()
			return f, nil

		case "enter":
			if f.showInput {
				// Use the text input value
				path := f.textInput.Value()
				if path != "" {
					if f.onConfirm != nil {
						f.onConfirm(path)
					}
					f.Hide()
				}
				return f, nil
			}
			// For filepicker mode, enter is handled by filepicker itself

		case " ": // Space to select current directory in dir mode
			if f.mode == FilePickerModeDir && !f.showInput {
				path := f.filepicker.CurrentDirectory
				if f.onConfirm != nil {
					f.onConfirm(path)
				}
				f.Hide()
				return f, nil
			}
		}

		if f.showInput {
			// Update text input
			var cmd tea.Cmd
			f.textInput, cmd = f.textInput.Update(msg)
			return f, cmd
		}
	}

	// Update filepicker
	var cmd tea.Cmd
	f.filepicker, cmd = f.filepicker.Update(msg)
	cmds = append(cmds, cmd)

	// Check if a file/dir was selected
	if didSelect, path := f.filepicker.DidSelectFile(msg); didSelect {
		f.selectedPath = path
		if f.onConfirm != nil {
			f.onConfirm(path)
		}
		f.Hide()
		return f, nil
	}

	return f, tea.Batch(cmds...)
}

// View renders the dialog
func (f *FilePickerDialog) View() string {
	if !f.visible {
		return ""
	}

	var content strings.Builder

	// Title - bright white for visibility
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	content.WriteString(titleStyle.Render(f.title))
	content.WriteString("\n")

	// Message - light gray
	if f.message != "" {
		msgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
		content.WriteString(msgStyle.Render(f.message))
		content.WriteString("\n")
	}

	// Current directory - use explicit light styling for dark background
	dirStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true) // Blue
	content.WriteString(dirStyle.Render("📁 " + f.filepicker.CurrentDirectory))
	content.WriteString("\n")
	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // Dim gray
	content.WriteString(separatorStyle.Render(strings.Repeat("─", 50)))
	content.WriteString("\n")

	// Style definitions for dark background
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Bold(true)
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true)

	if f.showInput {
		// Text input mode
		content.WriteString("\n")
		content.WriteString(labelStyle.Render("Path: "))
		content.WriteString(f.textInput.View())
		content.WriteString("\n\n")
		content.WriteString(helpStyle.Render("Enter to confirm • Tab to browse • Esc to cancel"))
	} else {
		// File picker mode - format output for alignment and full-width backgrounds
		fpOutput := formatFilePickerOutput(f.filepicker.View(), min(56, f.width-8))
		content.WriteString(fpOutput)
		content.WriteString("\n")

		// Help text based on mode
		var helpText string
		if f.mode == FilePickerModeDir {
			helpText = "Space to select this directory • Enter to open • Tab to type • Esc to cancel"
		} else {
			helpText = "Enter to select file • Tab to type path • Esc to cancel"
		}
		content.WriteString(helpStyle.Render(helpText))
	}

	// Modal box style with dark background for better contrast
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("33")). // Blue border
		Background(lipgloss.Color("235")).      // Dark gray background
		Foreground(lipgloss.Color("252")).      // Light gray text
		Padding(1, 2).
		Width(min(60, f.width-4))

	return boxStyle.Render(content.String())
}

// GetBox returns just the dialog box for overlaying
func (f *FilePickerDialog) GetBox() string {
	return f.View()
}

// GetDimensions returns the dimensions needed for centering
func (f *FilePickerDialog) GetDimensions() (width, height int) {
	return f.width, f.height
}
