// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package components

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/green/agcm/internal/tui/styles"
)

var fpDebugFile *os.File

func init() {
	fpDebugFile, _ = os.OpenFile("/tmp/agcm-filepicker-debug.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
}

func fpDebugLog(format string, args ...interface{}) {
	if fpDebugFile != nil {
		fmt.Fprintf(fpDebugFile, time.Now().Format("15:04:05.000")+" "+format+"\n", args...)
		_ = fpDebugFile.Sync()
	}
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

	ti := textinput.New()
	ti.Placeholder = "Enter path..."
	ti.CharLimit = 256
	ti.Width = 50

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
		fpDebugLog("KeyMsg: %q showInput=%v", msg.String(), f.showInput)
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
			fpDebugLog("Esc pressed, cancelling")
			if f.onCancel != nil {
				f.onCancel()
			}
			f.Hide()
			return f, nil

		case "enter":
			fpDebugLog("Enter pressed, showInput=%v", f.showInput)
			if f.showInput {
				// Use the text input value
				path := f.textInput.Value()
				fpDebugLog("Text input value: %q", path)
				if path != "" {
					fpDebugLog("Calling onConfirm with path: %s", path)
					if f.onConfirm != nil {
						f.onConfirm(path)
					}
					f.Hide()
					fpDebugLog("Hidden, visible=%v", f.visible)
				}
				return f, nil
			}
			// For filepicker mode, enter is handled by filepicker itself

		case " ": // Space to select current directory in dir mode
			if f.mode == FilePickerModeDir && !f.showInput {
				path := f.filepicker.CurrentDirectory
				fpDebugLog("Space pressed, selecting dir: %s", path)
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
		fpDebugLog("File selected via filepicker: %s", path)
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

	// Title
	content.WriteString(f.styles.Title.Render(f.title))
	content.WriteString("\n")

	// Message
	if f.message != "" {
		content.WriteString(f.styles.Muted.Render(f.message))
		content.WriteString("\n")
	}

	// Current directory
	dirStyle := f.styles.Label.Bold(true)
	content.WriteString(dirStyle.Render("📁 " + f.filepicker.CurrentDirectory))
	content.WriteString("\n")
	content.WriteString(strings.Repeat("─", 50))
	content.WriteString("\n")

	if f.showInput {
		// Text input mode
		content.WriteString("\n")
		content.WriteString(f.styles.Label.Render("Path: "))
		content.WriteString(f.textInput.View())
		content.WriteString("\n\n")
		content.WriteString(f.styles.Muted.Render("Enter to confirm • Tab to browse • Esc to cancel"))
	} else {
		// File picker mode
		content.WriteString(f.filepicker.View())
		content.WriteString("\n")

		// Help text based on mode
		var helpText string
		if f.mode == FilePickerModeDir {
			helpText = "Space to select this directory • Enter to open • Tab to type • Esc to cancel"
		} else {
			helpText = "Enter to select file • Tab to type path • Esc to cancel"
		}
		content.WriteString(f.styles.Muted.Render(helpText))
	}

	// Modal box style with background
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(f.styles.Header.GetBackground()).
		Background(lipgloss.Color("248")).
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
