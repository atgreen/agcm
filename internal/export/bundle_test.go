// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package export

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/green/agcm/internal/api"
)

// Two cases too big to share a 4MB bundle must land in separate files, and a
// case that fails to fetch is skipped without aborting the export.
func TestExportBundles(t *testing.T) {
	big := strings.Repeat("x", 2_500_000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/comments"):
			fmt.Fprint(w, `[]`)
		case strings.Contains(r.URL.Path, "/cases/FAIL"):
			http.Error(w, "boom", http.StatusInternalServerError)
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"caseNumber":  path.Base(r.URL.Path),
				"summary":     "summary of " + path.Base(r.URL.Path),
				"description": big,
			})
		}
	}))
	defer srv.Close()

	exporter, err := NewExporter(api.NewClient(api.WithBaseURL(srv.URL)), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	exported, bundles, err := exporter.ExportBundles(context.Background(), []string{"AAA", "FAIL", "BBB"}, dir, nil)
	if err != nil {
		t.Fatalf("ExportBundles: %v", err)
	}
	if exported != 2 {
		t.Errorf("exported = %d, want 2 (FAIL should be skipped)", exported)
	}
	if bundles != 2 {
		t.Errorf("bundles = %d, want 2 (cases exceed MaxBundleSize together)", bundles)
	}

	first, err := os.ReadFile(filepath.Join(dir, "export-bundle-1.md"))
	if err != nil {
		t.Fatalf("bundle 1 missing: %v", err)
	}
	if !strings.Contains(string(first), "AAA") || strings.Contains(string(first), "BBB") {
		t.Error("bundle 1 should contain case AAA only")
	}
	second, err := os.ReadFile(filepath.Join(dir, "export-bundle-2.md"))
	if err != nil {
		t.Fatalf("bundle 2 missing: %v", err)
	}
	if !strings.Contains(string(second), "BBB") {
		t.Error("bundle 2 should contain case BBB")
	}
}

// Small cases share one bundle, separated by a rule.
func TestExportBundlesSingleFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/comments") {
			fmt.Fprint(w, `[]`)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"caseNumber": path.Base(r.URL.Path),
			"summary":    "s",
		})
	}))
	defer srv.Close()

	exporter, err := NewExporter(api.NewClient(api.WithBaseURL(srv.URL)), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	exported, bundles, err := exporter.ExportBundles(context.Background(), []string{"A1", "A2", "A3"}, dir, nil)
	if err != nil {
		t.Fatalf("ExportBundles: %v", err)
	}
	if exported != 3 || bundles != 1 {
		t.Errorf("exported=%d bundles=%d, want 3 and 1", exported, bundles)
	}
	content, err := os.ReadFile(filepath.Join(dir, "export-bundle-1.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(content), bundleSeparator); got != 2 {
		t.Errorf("separator count = %d, want 2", got)
	}
}
