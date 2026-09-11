// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package styles

import (
	"reflect"
	"testing"

	"github.com/charmbracelet/bubbles/key"
)

// Every binding defined on the KeyMap must appear on the help screen, so a
// newly added binding can't silently become undiscoverable.
func TestFullHelpCoversAllBindings(t *testing.T) {
	k := DefaultKeyMap()

	covered := map[string]bool{}
	for _, group := range k.FullHelp() {
		for _, b := range group {
			covered[b.Help().Key] = true
		}
	}

	v := reflect.ValueOf(*k)
	for i := 0; i < v.NumField(); i++ {
		b, ok := v.Field(i).Interface().(key.Binding)
		if !ok {
			continue
		}
		name := v.Type().Field(i).Name
		if b.Help().Key == "" || b.Help().Desc == "" {
			t.Errorf("binding %s has no help text", name)
		}
		if !covered[b.Help().Key] {
			t.Errorf("binding %s (%q) is missing from FullHelp", name, b.Help().Key)
		}
	}
}
