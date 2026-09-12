// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package styles

import "github.com/charmbracelet/lipgloss"

// Theme is a named color scheme
type Theme struct {
	Name   string
	Colors ColorScheme
}

// Themes returns the available themes in cycle order. Names are what
// gets stored in the config's ui.theme field.
func Themes() []Theme {
	return []Theme{
		{Name: "dark", Colors: DarkColors()},
		{Name: "light", Colors: LightColors()},
		{Name: "dracula", Colors: DraculaColors()},
		{Name: "solarized", Colors: SolarizedColors()},
		{Name: "mono", Colors: MonoColors()},
	}
}

// ThemeIndex returns the index of the named theme, or -1 if unknown
// (including "auto" and "").
func ThemeIndex(name string) int {
	for i, t := range Themes() {
		if t.Name == name {
			return i
		}
	}
	return -1
}

// Apply rebuilds this Styles value in place from a color scheme, so every
// component holding a pointer to it re-skins on the next render.
func (s *Styles) Apply(c ColorScheme) {
	*s = *NewStyles(c)
}

// DraculaColors returns the Dracula theme color scheme
func DraculaColors() ColorScheme {
	return ColorScheme{
		Primary:    lipgloss.Color("#BD93F9"), // Purple
		Secondary:  lipgloss.Color("#8BE9FD"), // Cyan
		Accent:     lipgloss.Color("#FFB86C"), // Orange
		Success:    lipgloss.Color("#50FA7B"), // Green
		Warning:    lipgloss.Color("#F1FA8C"), // Yellow
		Error:      lipgloss.Color("#FF5555"), // Red
		Muted:      lipgloss.Color("#6272A4"), // Comment blue-grey
		Foreground: lipgloss.Color("#F8F8F2"),
		Border:     lipgloss.Color("#44475A"),
		Highlight:  lipgloss.Color("#6272A4"),
		Severity1:  lipgloss.Color("#FF5555"),
		Severity2:  lipgloss.Color("#FFB86C"),
		Severity3:  lipgloss.Color("#8BE9FD"),
		Severity4:  lipgloss.Color("#6272A4"),
	}
}

// SolarizedColors returns the Solarized Dark theme color scheme
func SolarizedColors() ColorScheme {
	return ColorScheme{
		Primary:    lipgloss.Color("#268BD2"), // Blue
		Secondary:  lipgloss.Color("#2AA198"), // Cyan
		Accent:     lipgloss.Color("#B58900"), // Yellow
		Success:    lipgloss.Color("#859900"), // Green
		Warning:    lipgloss.Color("#B58900"), // Yellow
		Error:      lipgloss.Color("#DC322F"), // Red
		Muted:      lipgloss.Color("#586E75"), // Base01
		Foreground: lipgloss.Color("#93A1A1"), // Base1
		Border:     lipgloss.Color("#586E75"),
		Highlight:  lipgloss.Color("#268BD2"),
		Severity1:  lipgloss.Color("#DC322F"),
		Severity2:  lipgloss.Color("#CB4B16"), // Orange
		Severity3:  lipgloss.Color("#268BD2"),
		Severity4:  lipgloss.Color("#586E75"),
	}
}

// MonoColors returns a monochrome scheme built from ANSI greys, for
// limited terminals and readers who prefer meaning without color.
// Severity and status stay legible through their text, not their hue.
func MonoColors() ColorScheme {
	white := lipgloss.Color("15")
	grey := lipgloss.Color("8")
	return ColorScheme{
		Primary:    grey,
		Secondary:  white,
		Accent:     white,
		Success:    white,
		Warning:    white,
		Error:      white,
		Muted:      grey,
		Foreground: white,
		Border:     grey,
		Highlight:  grey,
		Severity1:  white,
		Severity2:  white,
		Severity3:  white,
		Severity4:  grey,
	}
}
