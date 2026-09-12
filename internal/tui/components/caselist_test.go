// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package components

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/green/agcm/internal/api"
	"github.com/green/agcm/internal/tui/styles"
)

func testCaseList() *CaseList {
	return NewCaseList(styles.DarkStyles(), styles.DefaultKeyMap())
}

// Truncation must never split UTF-8 sequences or overflow the column budget,
// including for CJK and emoji summaries.
func TestRenderRowWideCharacters(t *testing.T) {
	c := testCaseList()
	c.SetSize(80, 10)

	summaries := []string{
		strings.Repeat("サーバーが応答しません", 10),
		strings.Repeat("é", 200),
		strings.Repeat("🔥", 100),
		"plain ascii but very long " + strings.Repeat("x", 200),
	}
	for _, summary := range summaries {
		cs := &api.Case{
			CaseNumber:   "00000001",
			Summary:      summary,
			Status:       "Waiting on Red Hat",
			Severity:     "2 (High)",
			LastModified: time.Now(),
		}
		for _, selected := range []bool{true, false} {
			row := c.renderRow(cs, 75, selected)
			if !utf8.ValidString(row) {
				t.Errorf("renderRow produced invalid UTF-8 for %.20q (selected=%v)", summary, selected)
			}
			if w := lipgloss.Width(row); w > 76 {
				t.Errorf("renderRow width = %d, want <= 76 for %.20q (selected=%v)", w, summary, selected)
			}
		}
	}
}

// A long status must be truncated to its column, not bleed into SUMMARY.
func TestRenderRowStatusColumn(t *testing.T) {
	c := testCaseList()
	cs := &api.Case{
		CaseNumber:   "00000001",
		Summary:      "s",
		Status:       strings.Repeat("状態", 30),
		Severity:     "3 (Normal)",
		LastModified: time.Now(),
	}
	row := c.renderRow(cs, 75, true)
	if !utf8.ValidString(row) {
		t.Error("renderRow produced invalid UTF-8 for wide status")
	}
}

// Header hit-testing must agree with the rendered column layout at both
// wide and narrow widths.
func TestHeaderColumnAt(t *testing.T) {
	c := testCaseList()

	c.SetSize(85, 10) // contentWidth 80, STATUS at full 20
	wide := map[int]HeaderColumn{
		0:  HeaderColCase,
		9:  HeaderColCase,
		10: HeaderColNone, // separator space
		11: HeaderColDate,
		22: HeaderColDate,
		24: HeaderColSev,
		29: HeaderColStatus,
		48: HeaderColStatus,
		50: HeaderColSummary,
		79: HeaderColSummary,
		80: HeaderColNone,
	}
	for x, want := range wide {
		if got := c.HeaderColumnAt(x); got != want {
			t.Errorf("wide: HeaderColumnAt(%d) = %v, want %v", x, got, want)
		}
	}

	c.SetSize(60, 10) // contentWidth 55, STATUS shrinks to 12
	narrow := map[int]HeaderColumn{
		29: HeaderColStatus,
		40: HeaderColStatus,
		42: HeaderColSummary,
		54: HeaderColSummary,
		55: HeaderColNone,
	}
	for x, want := range narrow {
		if got := c.HeaderColumnAt(x); got != want {
			t.Errorf("narrow: HeaderColumnAt(%d) = %v, want %v", x, got, want)
		}
	}
}

// On narrow terminals the STATUS column shrinks so SUMMARY keeps room.
func TestNarrowWidthKeepsSummaryRoom(t *testing.T) {
	c := testCaseList()
	c.SetSize(60, 10)
	cs := &api.Case{
		CaseNumber:   "00000001",
		Summary:      "kernel panic on boot after upgrade",
		Status:       "Waiting on Red Hat",
		Severity:     "2 (High)",
		LastModified: time.Now(),
	}
	row := c.renderRow(cs, 55, false)
	if w := lipgloss.Width(row); w > 55 {
		t.Errorf("narrow row width = %d, want <= 55", w)
	}
	if !strings.Contains(row, "kernel panic") {
		t.Errorf("summary crushed out of narrow row: %q", row)
	}
}

// The date column must follow the sort field so the header stays honest.
func TestRenderRowDateFollowsSortField(t *testing.T) {
	c := testCaseList()
	created := time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC)
	modified := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	cs := &api.Case{
		CaseNumber:   "00000001",
		Summary:      "s",
		Status:       "Open",
		Severity:     "3 (Normal)",
		CreatedDate:  created,
		LastModified: modified,
	}

	c.SetSort(SortByLastModified, true)
	if row := c.renderRow(cs, 75, true); !strings.Contains(row, "Jun 30 2026") {
		t.Error("sorting by modified: row should show the last-modified date")
	}
	c.SetSort(SortByCreated, true)
	if row := c.renderRow(cs, 75, true); !strings.Contains(row, "Jan 15 2020") {
		t.Error("sorting by created: row should show the created date")
	}
}

// keyMsg builds a tea.KeyMsg for a named key like "ctrl+u" or "pgdown".
func keyMsg(name string) tea.KeyMsg {
	switch name {
	case "ctrl+u":
		return tea.KeyMsg{Type: tea.KeyCtrlU}
	case "ctrl+d":
		return tea.KeyMsg{Type: tea.KeyCtrlD}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)}
}

// The paging bindings advertised in the KeyMap (ctrl+u/ctrl+d and
// pgup/pgdown) must actually move the cursor.
func TestPageKeysMoveCursor(t *testing.T) {
	c := testCaseList()
	c.SetSize(80, 10)

	cases := make([]api.Case, 50)
	for i := range cases {
		cases[i] = api.Case{CaseNumber: string(rune('a' + i%26))}
	}
	c.SetCases(cases)

	for _, keys := range [][]string{{"ctrl+d", "ctrl+u"}, {"pgdown", "pgup"}} {
		c.SetCursor(0)
		c.Update(keyMsg(keys[0]))
		if c.cursor == 0 {
			t.Errorf("%s did not move cursor down", keys[0])
		}
		c.Update(keyMsg(keys[1]))
		if c.cursor != 0 {
			t.Errorf("%s did not move cursor back to top, got %d", keys[1], c.cursor)
		}
	}
}
