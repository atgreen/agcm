// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/green/agcm/internal/api"
	"github.com/green/agcm/internal/tui/styles"
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

		m.showHelp = false
		m.showAbout = true
		assertFrame(t, m.View(), size.w, size.h, label+" about")
	}
}

// The About box must show attribution and the issue tracker, and any key
// must dismiss it.
func TestAboutBox(t *testing.T) {
	m := frameModel(80, 24)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = updated.(*Model)
	if !m.showAbout {
		t.Fatal("pressing a did not open the About box")
	}
	view := m.View()
	for _, want := range []string{"GPL-3.0-or-later", "github.com/atgreen/agcm/issues", "Anthony Green"} {
		if !strings.Contains(view, want) {
			t.Errorf("About view missing %q", want)
		}
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = updated.(*Model)
	if m.showAbout {
		t.Error("About box not dismissed by a keypress")
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

func loadPage(t *testing.T, m *Model, msg casesLoadedMsg) *Model {
	t.Helper()
	updated, _ := m.Update(msg)
	return updated.(*Model)
}

func pageCase(n string, mod time.Time) api.Case {
	return api.Case{CaseNumber: n, Summary: "s", Status: "Open", Severity: "3 (Normal)", LastModified: mod}
}

// The server pages by row offset over a live list sorted by last-modified,
// so a page can re-deliver a case already loaded (a modification between
// fetches shifts every row down). The append must dedupe, keeping the
// fresher copy.
func TestAppendPageDeduplicates(t *testing.T) {
	m := testModel()
	m.width, m.height = 80, 24
	m.ready = true
	m.updateLayout()
	now := time.Now()

	m = loadPage(t, m, casesLoadedMsg{
		cases: []api.Case{
			pageCase("00000001", now),
			pageCase("00000002", now.Add(-time.Hour)),
			pageCase("00000003", now.Add(-2*time.Hour)),
		},
		totalCount: 5,
	})

	// Page 2 re-delivers 00000003 with a fresher timestamp
	fresh := now.Add(time.Minute)
	m = loadPage(t, m, casesLoadedMsg{
		cases: []api.Case{
			pageCase("00000003", fresh),
			pageCase("00000004", now.Add(-3*time.Hour)),
			pageCase("00000005", now.Add(-4*time.Hour)),
		},
		totalCount: 5,
		startIndex: 3,
		append:     true,
	})

	if len(m.cases) != 5 {
		t.Fatalf("got %d cases, want 5 (duplicate not merged)", len(m.cases))
	}
	seen := map[string]int{}
	for _, c := range m.cases {
		seen[c.CaseNumber]++
		if c.CaseNumber == "00000003" && !c.LastModified.Equal(fresh) {
			t.Error("merge kept the stale copy of the re-delivered case")
		}
	}
	for n, count := range seen {
		if count > 1 {
			t.Errorf("case %s appears %d times", n, count)
		}
	}
}

// An append whose start index no longer matches the list length is from
// before a refresh or filter change and must be dropped, not appended.
func TestStaleAppendDropped(t *testing.T) {
	m := testModel()
	m.width, m.height = 80, 24
	m.ready = true
	m.updateLayout()
	now := time.Now()

	m = loadPage(t, m, casesLoadedMsg{
		cases:      []api.Case{pageCase("00000001", now), pageCase("00000002", now)},
		totalCount: 2,
	})

	m = loadPage(t, m, casesLoadedMsg{
		cases:      []api.Case{pageCase("00000009", now)},
		totalCount: 200,
		startIndex: 100,
		append:     true,
	})

	if len(m.cases) != 2 {
		t.Fatalf("stale append changed the list: got %d cases, want 2", len(m.cases))
	}
	for _, c := range m.cases {
		if c.CaseNumber == "00000009" {
			t.Error("stale page's case was appended")
		}
	}
}

// Pressing t must advance through every theme and wrap back to the start,
// re-skinning the shared styles as it goes.
func TestCycleTheme(t *testing.T) {
	m := frameModel(80, 24)
	themes := styles.Themes()
	start := m.themeIndex

	for i := 1; i <= len(themes); i++ {
		before := m.styles.Border.GetBorderTopForeground()
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
		m = updated.(*Model)

		want := (start + i) % len(themes)
		if m.themeIndex != want {
			t.Fatalf("after %d presses themeIndex = %d, want %d", i, m.themeIndex, want)
		}
		after := m.styles.Border.GetBorderTopForeground()
		if before == after && themes[want].Colors.Border != themes[(want+len(themes)-1)%len(themes)].Colors.Border {
			t.Errorf("press %d: styles did not change (border %v)", i, after)
		}
	}
	if m.themeIndex != start {
		t.Errorf("full cycle should return to start theme, got %d want %d", m.themeIndex, start)
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
