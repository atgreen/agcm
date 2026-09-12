// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package components

import (
	"testing"

	"github.com/green/agcm/internal/tui/styles"
)

// Dropdown placement and mouse hit-testing must follow wherever the app
// says the dialog was drawn (agcm-81e: the position used to be stuck at 0,0).
func TestFilterDialogPositionTracking(t *testing.T) {
	f := NewFilterDialog(styles.DarkStyles())
	f.ShowWithFilter(nil)

	x0, y0 := f.GetDropdownPosition()
	f.SetPosition(20, 5)
	x1, y1 := f.GetDropdownPosition()

	if x1 != x0+20 || y1 != y0+5 {
		t.Errorf("dropdown position (%d,%d) -> (%d,%d); want offset by (20,5)", x0, y0, x1, y1)
	}
}
