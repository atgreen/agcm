// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/green/agcm/internal/api"
)

func TestNormalizeCaseNumber(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"3789456", "03789456"},
		{"03789456", "03789456"},
		{"  42  ", "00000042"},
		{"123456789", "123456789"},
		{"abc123", "abc123"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := normalizeCaseNumber(tt.in); got != tt.want {
			t.Errorf("normalizeCaseNumber(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func testModel() *Model {
	return NewModel(nil, Options{}, nil)
}

func TestAnsiCutPreservesEscapes(t *testing.T) {
	link := "\x1b]8;;https://example.com\x1b\\text\x1b]8;;\x1b\\"
	csi := "\x1b[31mred\x1b[0m"

	// A cut that keeps the region must keep the OSC-8 open/close pair
	got := ansiCut(link+"tail", 0, 4)
	if !strings.Contains(got, "\x1b]8;;https://example.com\x1b\\") {
		t.Errorf("OSC-8 open sequence lost: %q", got)
	}
	if !strings.Contains(got, "text") {
		t.Errorf("visible text lost: %q", got)
	}

	// CSI sequences inside the window survive; visible width is respected
	got = ansiCut(csi, 0, 3)
	if !strings.Contains(got, "\x1b[31m") || !strings.Contains(got, "red") {
		t.Errorf("CSI cut broken: %q", got)
	}

	// A double-width rune straddling the boundary is dropped, not split
	got = ansiCut("ab漢cd", 0, 3)
	if strings.ContainsRune(got, '漢') {
		t.Errorf("straddling wide rune should be dropped: %q", got)
	}
	if got != "ab" {
		t.Errorf("ansiCut(ab漢cd, 0, 3) = %q, want %q", got, "ab")
	}
}

func TestSortCasesPreservesSelection(t *testing.T) {
	m := testModel()
	now := time.Now()
	m.cases = []api.Case{
		{CaseNumber: "00000001", LastModified: now.Add(-3 * time.Hour)},
		{CaseNumber: "00000002", LastModified: now.Add(-2 * time.Hour)},
		{CaseNumber: "00000003", LastModified: now.Add(-1 * time.Hour)},
	}
	m.caseList.SetSize(80, 10)
	m.sortCases()
	m.caseList.SetCursor(1)
	selected := m.caseList.SelectedCase().CaseNumber

	m.toggleSortOrder()

	if got := m.caseList.SelectedCase().CaseNumber; got != selected {
		t.Errorf("selection after sort = %s, want %s", got, selected)
	}
}

func TestCycleSortFieldKeepsSelection(t *testing.T) {
	m := testModel()
	now := time.Now()
	m.cases = []api.Case{
		{CaseNumber: "00000009", LastModified: now, CreatedDate: now.Add(-9 * time.Hour), Severity: "3 (Normal)"},
		{CaseNumber: "00000001", LastModified: now.Add(-5 * time.Hour), CreatedDate: now, Severity: "1 (Urgent)"},
		{CaseNumber: "00000005", LastModified: now.Add(-1 * time.Hour), CreatedDate: now.Add(-1 * time.Hour), Severity: "2 (High)"},
	}
	m.caseList.SetSize(80, 10)
	m.sortCases()
	m.caseList.SetCursor(2)
	selected := m.caseList.SelectedCase().CaseNumber

	for i := 0; i < 4; i++ {
		m.cycleSortField()
		if got := m.caseList.SelectedCase().CaseNumber; got != selected {
			t.Errorf("after cycle %d: selection = %s, want %s", i, got, selected)
		}
	}
}
