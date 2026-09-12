// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package export

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Manifest records metadata about an export operation
type Manifest struct {
	ExportedAt     time.Time        `json:"exported_at"`
	TotalCases     int              `json:"total_cases"`
	FiltersApplied *ManifestFilters `json:"filters_applied,omitempty"`
	Cases          []ManifestCase   `json:"cases"`
}

// ManifestFilters records what filters were used
type ManifestFilters struct {
	Status   []string `json:"status,omitempty"`
	Severity []string `json:"severity,omitempty"`
	Products []string `json:"products,omitempty"`
	Since    string   `json:"since,omitempty"`
	Until    string   `json:"until,omitempty"`
}

// ManifestCase records info about a single exported case
type ManifestCase struct {
	CaseNumber            string `json:"case_number"`
	Summary               string `json:"summary"`
	File                  string `json:"file"`
	AttachmentsDownloaded int    `json:"attachments_downloaded"`
}

// NewManifest creates a new empty manifest
func NewManifest() *Manifest {
	return &Manifest{
		Cases: make([]ManifestCase, 0),
	}
}

// AddCase adds a case to the manifest
func (m *Manifest) AddCase(caseNumber, summary, file string, attachments int) {
	m.Cases = append(m.Cases, ManifestCase{
		CaseNumber:            caseNumber,
		Summary:               summary,
		File:                  file,
		AttachmentsDownloaded: attachments,
	})
}

// SetFilters records the filters that were applied
func (m *Manifest) SetFilters(status, severity, products []string, since, until string) {
	m.FiltersApplied = &ManifestFilters{
		Status:   status,
		Severity: severity,
		Products: products,
		Since:    since,
		Until:    until,
	}
}

// Save writes the manifest to a JSON file
func (m *Manifest) Save(path string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	return nil
}
