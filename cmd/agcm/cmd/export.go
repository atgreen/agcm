// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/green/agcm/internal/api"
	"github.com/green/agcm/internal/export"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export cases to markdown",
	Long:  `Export support cases and their conversations to markdown format.`,
}

var exportCaseCmd = &cobra.Command{
	Use:   "case [case-numbers...]",
	Short: "Export specific cases",
	Long: `Export one or more specific cases by their case numbers.

Examples:
  agcm export case 01234567
  agcm export case 01234567 01234568 01234569 --output-dir ./exports/
  agcm export case 01234567 --output ./case.md`,
	Args: cobra.MinimumNArgs(1),
	RunE: runExportCase,
}

var exportCasesCmd = &cobra.Command{
	Use:   "cases",
	Short: "Export cases with filters",
	Long: `Export multiple cases matching the specified filters.

Examples:
  agcm export cases --status open
  agcm export cases --status open,waiting --severity 1,2
  agcm export cases --product "Red Hat Enterprise Linux" --since 2024-01-01`,
	RunE: runExportCases,
}

var (
	// Export flags
	exportOutput          string
	exportOutputDir       string
	exportFormat          string
	exportCombined        bool
	exportIncludeAttach   bool
	exportAttachmentsDir  string
	exportTemplate        string
	exportConcurrency     int

	// Filter flags
	exportStatus    string
	exportSeverity  string
	exportProduct   string
	exportSince     string
	exportUntil     string
)

func init() {
	rootCmd.AddCommand(exportCmd)
	exportCmd.AddCommand(exportCaseCmd)
	exportCmd.AddCommand(exportCasesCmd)

	// Common export flags
	exportCaseCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "output file (for single case)")
	exportCaseCmd.Flags().StringVarP(&exportOutputDir, "output-dir", "d", "./exports", "output directory")
	exportCaseCmd.Flags().StringVar(&exportFormat, "format", "markdown", "output format (markdown, json)")
	exportCaseCmd.Flags().BoolVar(&exportCombined, "combined", false, "combine all cases into single file")
	exportCaseCmd.Flags().BoolVar(&exportIncludeAttach, "include-attachments", false, "download attachments")
	exportCaseCmd.Flags().StringVar(&exportAttachmentsDir, "attachments-dir", "attachments", "attachments directory name")
	exportCaseCmd.Flags().StringVar(&exportTemplate, "template", "", "custom Go template file")
	exportCaseCmd.Flags().IntVar(&exportConcurrency, "concurrency", 4, "parallel downloads")

	exportCasesCmd.Flags().StringVarP(&exportOutputDir, "output-dir", "d", "./exports", "output directory")
	exportCasesCmd.Flags().StringVar(&exportFormat, "format", "markdown", "output format (markdown, json)")
	exportCasesCmd.Flags().BoolVar(&exportCombined, "combined", false, "combine all cases into single file")
	exportCasesCmd.Flags().BoolVar(&exportIncludeAttach, "include-attachments", false, "download attachments")
	exportCasesCmd.Flags().StringVar(&exportAttachmentsDir, "attachments-dir", "attachments", "attachments directory name")
	exportCasesCmd.Flags().StringVar(&exportTemplate, "template", "", "custom Go template file")
	exportCasesCmd.Flags().IntVar(&exportConcurrency, "concurrency", 4, "parallel downloads")

	// Filter flags for cases command
	exportCasesCmd.Flags().StringVar(&exportStatus, "status", "", "filter by status: open, closed, or exact values (comma-separated)")
	exportCasesCmd.Flags().StringVar(&exportSeverity, "severity", "", "filter by severity (comma-separated)")
	exportCasesCmd.Flags().StringVar(&exportProduct, "product", "", "filter by product")
	exportCasesCmd.Flags().StringVar(&exportSince, "since", "", "filter by start date (YYYY-MM-DD)")
	exportCasesCmd.Flags().StringVar(&exportUntil, "until", "", "filter by end date (YYYY-MM-DD)")
	exportCasesCmd.Flags().StringVarP(&exportAccount, "account", "a", "", "filter by account number")
	exportCasesCmd.Flags().StringVarP(&exportGroup, "group", "g", "", "filter by case group number")
}

var (
	exportAccount string
	exportGroup   string
)

func runExportCase(cmd *cobra.Command, args []string) error {
	client := GetAPIClient()

	opts := &export.Options{
		OutputDir:          exportOutputDir,
		OutputFile:         exportOutput,
		Format:             exportFormat,
		IncludeAttachments: exportIncludeAttach,
		AttachmentsDir:     exportAttachmentsDir,
		Combined:           exportCombined,
		Concurrency:        exportConcurrency,
		TemplatePath:       exportTemplate,
		CaseNumbers:        args,
		Debug:              IsDebugMode(),
	}

	exporter, err := export.NewExporter(client, opts)
	if err != nil {
		return fmt.Errorf("failed to create exporter: %w", err)
	}

	// Single case with direct output
	if len(args) == 1 && exportOutput != "" {
		fmt.Printf("Exporting case %s to %s...\n", args[0], exportOutput)
		if err := exporter.ExportCaseToFile(context.Background(), args[0], exportOutput); err != nil {
			return fmt.Errorf("export failed: %w", err)
		}
		fmt.Println("Export complete!")
		return nil
	}

	// Multiple cases
	progressCh := make(chan export.Progress, 10)
	go func() {
		for p := range progressCh {
			fmt.Printf("\r[%d/%d] %s: %s          ",
				p.CompletedCases, p.TotalCases, p.CurrentCase, p.CurrentStep)
		}
	}()

	fmt.Printf("Exporting %d case(s) to %s...\n", len(args), exportOutputDir)
	manifest, err := exporter.ExportCases(context.Background(), args, progressCh)
	close(progressCh)
	fmt.Println()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	if manifest != nil {
		fmt.Printf("Exported %d cases\n", len(manifest.Cases))
		fmt.Printf("Manifest: %s/export-manifest.json\n", exportOutputDir)
	}

	return nil
}

func runExportCases(cmd *cobra.Command, args []string) error {
	client := GetAPIClient()

	// Build filter
	filter := &api.CaseFilter{}

	if exportStatus != "" {
		// Handle status aliases
		statusInput := strings.ToLower(strings.TrimSpace(exportStatus))
		switch statusInput {
		case "open":
			// "open" is an alias for all active (non-closed) statuses
			filter.Status = []string{"Waiting on Red Hat", "Waiting on Customer"}
		case "closed":
			filter.Status = []string{"Closed"}
		default:
			filter.Status = strings.Split(exportStatus, ",")
		}
	}
	if exportSeverity != "" {
		filter.Severity = strings.Split(exportSeverity, ",")
	}
	if exportProduct != "" {
		products := strings.Split(exportProduct, ",")
		for i := range products {
			products[i] = strings.TrimSpace(products[i])
		}
		filter.Products = products
	}
	if exportSince != "" {
		t, err := time.Parse("2006-01-02", exportSince)
		if err != nil {
			return fmt.Errorf("invalid since date: %w", err)
		}
		filter.StartDate = &t
	}
	if exportUntil != "" {
		t, err := time.Parse("2006-01-02", exportUntil)
		if err != nil {
			return fmt.Errorf("invalid until date: %w", err)
		}
		filter.EndDate = &t
	}
	if exportAccount != "" {
		accounts := strings.Split(exportAccount, ",")
		for i := range accounts {
			accounts[i] = strings.TrimSpace(accounts[i])
		}
		filter.Accounts = accounts
	}
	if exportGroup != "" {
		filter.GroupNumber = exportGroup
	}

	opts := &export.Options{
		OutputDir:          exportOutputDir,
		Format:             exportFormat,
		IncludeAttachments: exportIncludeAttach,
		AttachmentsDir:     exportAttachmentsDir,
		Combined:           exportCombined,
		Concurrency:        exportConcurrency,
		TemplatePath:       exportTemplate,
		Debug:              IsDebugMode(),
	}

	exporter, err := export.NewExporter(client, opts)
	if err != nil {
		return fmt.Errorf("failed to create exporter: %w", err)
	}

	progressCh := make(chan export.Progress, 10)
	go func() {
		for p := range progressCh {
			fmt.Printf("\r[%d/%d] %s: %s          ",
				p.CompletedCases, p.TotalCases, p.CurrentCase, p.CurrentStep)
		}
	}()

	fmt.Println("Fetching cases matching filters...")
	manifest, err := exporter.ExportWithFilter(context.Background(), filter, progressCh)
	close(progressCh)
	fmt.Println()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	if manifest != nil {
		fmt.Printf("Exported %d cases to %s\n", len(manifest.Cases), exportOutputDir)
		fmt.Printf("Manifest: %s/export-manifest.json\n", exportOutputDir)

		// Record filters in manifest and re-save
		if filter.Status != nil || filter.Severity != nil || len(filter.Products) > 0 {
			manifest.SetFilters(filter.Status, filter.Severity, filter.Products, exportSince, exportUntil)
			// Re-save manifest with filter metadata
			manifestPath := exportOutputDir + "/export-manifest.json"
			if err := manifest.Save(manifestPath); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to save manifest with filters: %v\n", err)
			}
		}
	}

	return nil
}
