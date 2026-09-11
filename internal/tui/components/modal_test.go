// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/green/agcm/internal/tui/styles"
)

// Esc on the progress modal must invoke the cancel callback, and a progress
// modal must not inherit callbacks from an earlier text-input modal.
func TestProgressModalEscCancels(t *testing.T) {
	m := NewModal(styles.DarkStyles())

	stale := false
	m.ShowTextInput("t", "m", "", func(string) { stale = true }, func() { stale = true })
	m.Hide()

	cancelled := false
	m.ShowProgress("Exporting", "working...", func() { cancelled = true })

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if !cancelled {
		t.Error("Esc on progress modal did not invoke onCancel")
	}
	if stale {
		t.Error("progress modal invoked a stale text-input callback")
	}
	if m.IsVisible() {
		t.Error("modal still visible after Esc")
	}
}

func TestProgressModalIgnoresOtherKeys(t *testing.T) {
	m := NewModal(styles.DarkStyles())
	cancelled := false
	m.ShowProgress("Exporting", "working...", func() { cancelled = true })

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if cancelled {
		t.Error("non-Esc key cancelled the export")
	}
	if !m.IsVisible() {
		t.Error("non-Esc key dismissed the progress modal")
	}
}
