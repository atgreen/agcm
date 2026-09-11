// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package styles

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all keyboard shortcuts
type KeyMap struct {
	Up           key.Binding
	Down         key.Binding
	Left         key.Binding
	Right        key.Binding
	Top          key.Binding
	Bottom       key.Binding
	PageUp       key.Binding
	PageDown     key.Binding
	Select       key.Binding
	Back         key.Binding
	Tab          key.Binding
	ShiftTab     key.Binding
	Search       key.Binding
	Filter       key.Binding
	ClearFilter  key.Binding
	PresetLoad   key.Binding
	PresetSave   key.Binding
	Sort         key.Binding
	SortOrder    key.Binding
	Refresh      key.Binding
	Theme        key.Binding
	About        key.Binding
	Help         key.Binding
	Quit         key.Binding
	Open         key.Binding
	Export       key.Binding
	BulkExport   key.Binding
	BundleExport key.Binding
	TextSearch   key.Binding
	NextComment  key.Binding
	PrevComment  key.Binding
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() *KeyMap {
	return &KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "prev detail tab"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "next detail tab"),
		),
		Top: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("gg", "go to top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "go to bottom"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("ctrl+u", "pgup"),
			key.WithHelp("ctrl+u", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("ctrl+d", "pgdown"),
			key.WithHelp("ctrl+d", "page down"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "focus detail"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc", "backspace"),
			key.WithHelp("esc", "back to list"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "switch pane"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("shift+tab", "prev pane"),
		),
		Search: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "search"),
		),
		Filter: key.NewBinding(
			key.WithKeys("f"),
			key.WithHelp("f", "filter"),
		),
		ClearFilter: key.NewBinding(
			key.WithKeys("F"),
			key.WithHelp("F", "clear filter"),
		),
		PresetLoad: key.NewBinding(
			key.WithKeys("1", "2", "3", "4", "5", "6", "7", "8", "9", "0"),
			key.WithHelp("1-9, 0", "load filter preset"),
		),
		PresetSave: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s + #", "save filter preset"),
		),
		Sort: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "cycle sort field"),
		),
		SortOrder: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "toggle sort order"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh"),
		),
		Theme: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "cycle theme"),
		),
		About: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "about"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Open: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open in browser"),
		),
		Export: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "export"),
		),
		BulkExport: key.NewBinding(
			key.WithKeys("E"),
			key.WithHelp("E", "export all cases"),
		),
		BundleExport: key.NewBinding(
			key.WithKeys("B"),
			key.WithHelp("B", "bundle export (4MB files)"),
		),
		TextSearch: key.NewBinding(
			key.WithKeys("ctrl+f"),
			key.WithHelp("ctrl+f", "find in case"),
		),
		NextComment: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "next comment"),
		),
		PrevComment: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "prev comment"),
		),
	}
}

// ShortHelp returns the bindings shown in the status bar
func (k *KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Search, k.Filter, k.Sort, k.Help, k.Quit}
}

// FullHelp returns the bindings shown on the help screen, in display order
func (k *KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Top, k.Bottom, k.PageUp, k.PageDown},
		{k.Tab, k.ShiftTab, k.Select, k.Back},
		{k.Open, k.Search, k.Filter, k.ClearFilter},
		{k.TextSearch, k.PresetLoad, k.PresetSave},
		{k.NextComment, k.PrevComment, k.Sort, k.SortOrder},
		{k.Refresh, k.Export, k.BulkExport, k.BundleExport},
		{k.Theme, k.About, k.Help, k.Quit},
	}
}
