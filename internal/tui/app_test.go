// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
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

// frameModel builds a ready model with a few cases at the given size
func frameModel(w, h int) *Model {
	m := testModel()
	m.width, m.height = w, h
	m.ready = true
	m.initialLoadDone = true
	now := time.Now()
	m.cases = []api.Case{
		{CaseNumber: "00000001", Summary: "kernel panic on boot", Status: "Open", Severity: "1 (Urgent)", LastModified: now},
		{CaseNumber: "00000002", Summary: "サーバーが応答しません — wide chars", Status: "Waiting on Red Hat", Severity: "2 (High)", LastModified: now.Add(-time.Hour)},
		{CaseNumber: "00000003", Summary: "slow NFS mounts", Status: "Closed", Severity: "3 (Normal)", LastModified: now.Add(-2 * time.Hour)},
	}
	m.updateLayout()
	m.sortCases()
	return m
}

// assertFrame checks the renderer contract: exactly h lines, none wider than w
func assertFrame(t *testing.T, view string, w, h int, label string) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) != h {
		t.Errorf("%s: frame has %d lines, want %d", label, len(lines), h)
	}
	for i, line := range lines {
		if lw := lipgloss.Width(line); lw > w {
			t.Errorf("%s: line %d width = %d, want <= %d", label, i, lw, w)
		}
	}
}

// The frame must be exactly terminal-sized at every supported size,
// including the 60-column tmux split and the declared minimum.
func TestViewFrameSizes(t *testing.T) {
	sizes := []struct{ w, h int }{
		{80, 24},
		{60, 20},
		{120, 40},
		{minTermWidth, minTermHeight},
	}
	for _, size := range sizes {
		label := fmt.Sprintf("%dx%d", size.w, size.h)
		m := frameModel(size.w, size.h)
		assertFrame(t, m.View(), size.w, size.h, label)

		m.showHelp = true
		assertFrame(t, m.View(), size.w, size.h, label+" help")
	}
}

// Below the minimum the app must say so rather than degrade silently.
func TestViewTooSmall(t *testing.T) {
	m := frameModel(30, 8)
	view := m.View()
	if !strings.Contains(view, "Terminal too small") {
		t.Errorf("30x8 frame missing too-small message: %q", view)
	}
	assertFrame(t, view, 30, 8, "30x8")
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
