// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package styles

import "testing"

func TestThemeRegistry(t *testing.T) {
	themes := Themes()
	if len(themes) < 2 {
		t.Fatalf("expected at least dark and light themes, got %d", len(themes))
	}

	seen := map[string]bool{}
	for i, th := range themes {
		if th.Name == "" {
			t.Errorf("theme %d has no name", i)
		}
		if seen[th.Name] {
			t.Errorf("duplicate theme name %q", th.Name)
		}
		seen[th.Name] = true
		if ThemeIndex(th.Name) != i {
			t.Errorf("ThemeIndex(%q) = %d, want %d", th.Name, ThemeIndex(th.Name), i)
		}
	}

	for _, unknown := range []string{"auto", "", "no-such-theme"} {
		if idx := ThemeIndex(unknown); idx != -1 {
			t.Errorf("ThemeIndex(%q) = %d, want -1", unknown, idx)
		}
	}
}

// Apply must rebuild the Styles in place so components sharing the pointer
// see the new palette.
func TestApplyMutatesInPlace(t *testing.T) {
	s := NewStyles(DarkColors())
	shared := s // simulates a component holding the same pointer

	before := shared.Border.GetBorderTopForeground()
	s.Apply(DraculaColors())
	after := shared.Border.GetBorderTopForeground()

	if before == after {
		t.Error("Apply did not change the shared Styles value")
	}
}
