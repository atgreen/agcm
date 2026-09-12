// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package cmd

import (
	"fmt"

	"github.com/green/agcm/internal/api"
)

// applyPresetArg loads the preset named by args (a single slot digit 0-9)
// into filter. It returns done=true when the command should exit early
// (no preset saved in the slot and no other filters given); noun names the
// command in that message ("list", "export").
func applyPresetArg(args []string, hasCliFilters bool, filter *api.CaseFilter, noun string) (done bool, err error) {
	if len(args) != 1 {
		return false, nil
	}
	slot := args[0]
	if len(slot) != 1 || slot[0] < '0' || slot[0] > '9' {
		return false, fmt.Errorf("invalid preset: %s (must be 0-9)", slot)
	}
	preset := configMgr.GetPreset(slot)
	if preset == nil {
		if !hasCliFilters {
			fmt.Printf("No preset saved in slot %s. Nothing to %s.\n", slot, noun)
			return true, nil
		}
		return false, nil
	}

	// Load preset filters as defaults; CLI flags override them afterwards
	fmt.Printf("Using preset %s: %s\n", slot, preset.Name)
	if len(preset.Status) > 0 {
		filter.Status = preset.Status
	}
	if len(preset.Severity) > 0 {
		filter.Severity = preset.Severity
	}
	if len(preset.Products) > 0 {
		filter.Products = preset.Products
	}
	if len(preset.Accounts) > 0 {
		filter.Accounts = preset.Accounts
	}
	return false, nil
}
