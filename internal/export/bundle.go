// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package export

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MaxBundleSize caps each bundle file written by ExportBundles.
const MaxBundleSize = 4 * 1024 * 1024 // 4MB

// bundleSeparator joins cases within a bundle file.
const bundleSeparator = "\n\n---\n\n"

// ExportBundles exports the given cases (with comments, without attachments)
// into bundled markdown files named export-bundle-N.md in outputDir, each at
// most MaxBundleSize bytes. Cases that fail to fetch or format are skipped.
// If progressCh is non-nil it receives an update before each case is fetched.
// Returns the number of cases exported and bundle files written.
func (e *Exporter) ExportBundles(ctx context.Context, caseNumbers []string, outputDir string, progressCh chan<- Progress) (exported, bundles int, err error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return 0, 0, fmt.Errorf("failed to create output directory: %w", err)
	}

	total := len(caseNumbers)
	bundleNum := 1
	var bundle strings.Builder
	casesInBundle := 0

	writeBundle := func() error {
		path := filepath.Join(outputDir, fmt.Sprintf("export-bundle-%d.md", bundleNum))
		if err := os.WriteFile(path, []byte(bundle.String()), 0644); err != nil {
			return fmt.Errorf("failed to write bundle %d: %w", bundleNum, err)
		}
		return nil
	}

	for i, caseNumber := range caseNumbers {
		select {
		case <-ctx.Done():
			return exported, bundleNum - 1, ctx.Err()
		default:
		}

		if progressCh != nil {
			progressCh <- Progress{
				TotalCases:     total,
				CompletedCases: i,
				CurrentCase:    caseNumber,
				CurrentStep:    fmt.Sprintf("Fetching case %d/%d", i+1, total),
			}
		}

		caseDetail, err := e.client.GetCase(ctx, caseNumber)
		if err != nil {
			e.debugf("ExportBundles: skipping case %s: %v", caseNumber, err)
			continue
		}
		comments, _ := e.client.GetCaseComments(ctx, caseNumber)

		caseMarkdown, err := e.formatter.FormatCase(&CaseExport{
			Case:       caseDetail,
			Comments:   comments,
			ExportedAt: time.Now(),
		})
		if err != nil {
			e.debugf("ExportBundles: skipping case %s: %v", caseNumber, err)
			continue
		}

		// Flush the current bundle when this case would push it over the cap
		if bundle.Len() > 0 && bundle.Len()+len(caseMarkdown)+len(bundleSeparator) > MaxBundleSize {
			if err := writeBundle(); err != nil {
				return exported, bundleNum - 1, err
			}
			bundleNum++
			bundle.Reset()
			casesInBundle = 0
		}

		if casesInBundle > 0 {
			bundle.WriteString(bundleSeparator)
		}
		bundle.WriteString(caseMarkdown)
		casesInBundle++
		exported++
	}

	if bundle.Len() == 0 {
		return exported, bundleNum - 1, nil
	}
	if err := writeBundle(); err != nil {
		return exported, bundleNum - 1, err
	}
	return exported, bundleNum, nil
}
