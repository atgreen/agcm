// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package components

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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
