// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package export

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/green/agcm/internal/api"
)

// Options configures the export operation
type Options struct {
	OutputDir          string
	OutputFile         string   // For single-file combined export
	Format             string   // "markdown" or "json"
	IncludeAttachments bool
	AttachmentsDir     string
	Combined           bool     // Combine all cases into single file
	Concurrency        int
	TemplatePath       string   // Custom template file
	CaseNumbers        []string // Specific cases to export
}

// DefaultOptions returns sensible defaults
func DefaultOptions() *Options {
	return &Options{
		OutputDir:      "./exports",
		Format:         "markdown",
		AttachmentsDir: "attachments",
		Concurrency:    4,
	}
}

// Progress reports export progress
type Progress struct {
	TotalCases      int
	CompletedCases  int
	CurrentCase     string
	CurrentStep     string
	Error           error
}

// Exporter handles bulk case exports
type Exporter struct {
	client    *api.Client
	formatter *Formatter
	opts      *Options
}

// NewExporter creates a new exporter
func NewExporter(client *api.Client, opts *Options) (*Exporter, error) {
	if opts == nil {
		opts = DefaultOptions()
	}

	var formatter *Formatter
	var err error

	if opts.TemplatePath != "" {
		tmplData, err := os.ReadFile(opts.TemplatePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read template: %w", err)
		}
		formatter, err = NewFormatterWithTemplate(string(tmplData))
		if err != nil {
			return nil, err
		}
	} else {
		formatter, err = NewFormatter()
		if err != nil {
			return nil, err
		}
	}

	return &Exporter{
		client:    client,
		formatter: formatter,
		opts:      opts,
	}, nil
}

// ExportCase exports a single case to markdown
func (e *Exporter) ExportCase(ctx context.Context, caseNumber string) (*CaseExport, error) {
	// Get case details
	c, err := e.client.GetCase(ctx, caseNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get case %s: %w", caseNumber, err)
	}

	// Get comments
	comments, err := e.client.GetCaseComments(ctx, caseNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments for case %s: %w", caseNumber, err)
	}

	// Get attachments
	attachments, err := e.client.GetCaseAttachments(ctx, caseNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get attachments for case %s: %w", caseNumber, err)
	}

	return &CaseExport{
		Case:        c,
		Comments:    comments,
		Attachments: attachments,
		ExportedAt:  time.Now(),
	}, nil
}

// ExportCaseToFile exports a single case to a file
func (e *Exporter) ExportCaseToFile(ctx context.Context, caseNumber, outputPath string) error {
	export, err := e.ExportCase(ctx, caseNumber)
	if err != nil {
		return err
	}

	md, err := e.formatter.FormatCase(export)
	if err != nil {
		return fmt.Errorf("failed to format case: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(outputPath, []byte(md), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ExportCases exports multiple cases with progress reporting
func (e *Exporter) ExportCases(ctx context.Context, caseNumbers []string, progressCh chan<- Progress) (*Manifest, error) {
	if err := os.MkdirAll(e.opts.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	manifest := NewManifest()
	manifest.ExportedAt = time.Now()
	manifest.TotalCases = len(caseNumbers)

	var exports []*CaseExport
	var exportsMu sync.Mutex

	// Use semaphore for concurrency control
	sem := make(chan struct{}, e.opts.Concurrency)
	var wg sync.WaitGroup
	errCh := make(chan error, len(caseNumbers))

	for i, caseNum := range caseNumbers {
		wg.Add(1)
		go func(idx int, cn string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			if progressCh != nil {
				progressCh <- Progress{
					TotalCases:     len(caseNumbers),
					CompletedCases: idx,
					CurrentCase:    cn,
					CurrentStep:    "Fetching case data",
				}
			}

			export, err := e.ExportCase(ctx, cn)
			if err != nil {
				errCh <- fmt.Errorf("case %s: %w", cn, err)
				return
			}

			exportsMu.Lock()
			exports = append(exports, export)
			exportsMu.Unlock()

			if !e.opts.Combined {
				// Write individual file
				var outputPath string
				if e.opts.OutputDir != "" {
					caseDir := filepath.Join(e.opts.OutputDir, cn)
					if err := os.MkdirAll(caseDir, 0755); err != nil {
						errCh <- fmt.Errorf("failed to create case directory: %w", err)
						return
					}
					outputPath = filepath.Join(caseDir, "case.md")
				} else {
					outputPath = fmt.Sprintf("case-%s.md", cn)
				}

				md, err := e.formatter.FormatCase(export)
				if err != nil {
					errCh <- fmt.Errorf("failed to format case %s: %w", cn, err)
					return
				}

				if err := os.WriteFile(outputPath, []byte(md), 0644); err != nil {
					errCh <- fmt.Errorf("failed to write case %s: %w", cn, err)
					return
				}

				// Download attachments if requested
				if e.opts.IncludeAttachments && len(export.Attachments) > 0 {
					attDir := filepath.Join(filepath.Dir(outputPath), e.opts.AttachmentsDir)
					if err := os.MkdirAll(attDir, 0755); err != nil {
						errCh <- fmt.Errorf("failed to create attachments directory: %w", err)
						return
					}

					for _, att := range export.Attachments {
						if progressCh != nil {
							progressCh <- Progress{
								TotalCases:     len(caseNumbers),
								CompletedCases: idx,
								CurrentCase:    cn,
								CurrentStep:    fmt.Sprintf("Downloading %s", att.Filename),
							}
						}

						if err := e.downloadAttachment(ctx, cn, att, attDir); err != nil {
							// Log but don't fail the whole export
							fmt.Fprintf(os.Stderr, "Warning: failed to download attachment %s: %v\n", att.Filename, err)
						}
					}
				}

				// Add to manifest
				manifest.AddCase(cn, export.Case.Summary, filepath.Base(filepath.Dir(outputPath))+"/case.md", len(export.Attachments))
			}

			if progressCh != nil {
				progressCh <- Progress{
					TotalCases:     len(caseNumbers),
					CompletedCases: idx + 1,
					CurrentCase:    cn,
					CurrentStep:    "Complete",
				}
			}
		}(i, caseNum)
	}

	wg.Wait()
	close(errCh)

	// Collect errors
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		// Return partial results with error
		return manifest, fmt.Errorf("export completed with %d errors: %v", len(errs), errs[0])
	}

	// Write combined file if requested
	if e.opts.Combined && len(exports) > 0 {
		md, err := e.formatter.FormatCases(exports)
		if err != nil {
			return manifest, fmt.Errorf("failed to format combined export: %w", err)
		}

		outputPath := e.opts.OutputFile
		if outputPath == "" {
			outputPath = filepath.Join(e.opts.OutputDir, "all-cases.md")
		}

		if err := os.WriteFile(outputPath, []byte(md), 0644); err != nil {
			return manifest, fmt.Errorf("failed to write combined export: %w", err)
		}
	}

	// Write manifest
	manifestPath := filepath.Join(e.opts.OutputDir, "export-manifest.json")
	if err := manifest.Save(manifestPath); err != nil {
		return manifest, fmt.Errorf("failed to write manifest: %w", err)
	}

	return manifest, nil
}

// downloadAttachment downloads a single attachment
func (e *Exporter) downloadAttachment(ctx context.Context, caseNumber string, att api.Attachment, destDir string) error {
	reader, filename, err := e.client.DownloadAttachment(ctx, caseNumber, att.UUID)
	if err != nil {
		return err
	}
	defer reader.Close()

	if filename == "" {
		filename = att.Filename
	}

	destPath := filepath.Join(destDir, filename)
	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, reader); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ExportWithFilter exports cases matching the given filter
func (e *Exporter) ExportWithFilter(ctx context.Context, filter *api.CaseFilter, progressCh chan<- Progress) (*Manifest, error) {
	// Fetch all matching cases
	var allCases []api.Case
	filter.Count = 100 // Fetch in batches
	filter.StartIndex = 0

	for {
		result, err := e.client.ListCases(ctx, filter)
		if err != nil {
			return nil, fmt.Errorf("failed to list cases: %w", err)
		}

		allCases = append(allCases, result.Items...)

		if len(result.Items) < filter.Count || len(allCases) >= result.TotalCount {
			break
		}
		filter.StartIndex += filter.Count
	}

	// Extract case numbers
	caseNumbers := make([]string, len(allCases))
	for i, c := range allCases {
		caseNumbers[i] = c.CaseNumber
	}

	return e.ExportCases(ctx, caseNumbers, progressCh)
}
