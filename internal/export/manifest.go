// SPDX-License-Identifier: GPL-3.0-or-later
package export

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Manifest records metadata about an export operation
type Manifest struct {
	ExportedAt     time.Time         `json:"exported_at"`
	TotalCases     int               `json:"total_cases"`
	FiltersApplied *ManifestFilters  `json:"filters_applied,omitempty"`
	Cases          []ManifestCase    `json:"cases"`
}

// ManifestFilters records what filters were used
type ManifestFilters struct {
	Status   []string `json:"status,omitempty"`
	Severity []string `json:"severity,omitempty"`
	Product  string   `json:"product,omitempty"`
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
func (m *Manifest) SetFilters(status, severity []string, product, since, until string) {
	m.FiltersApplied = &ManifestFilters{
		Status:   status,
		Severity: severity,
		Product:  product,
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

// LoadManifest reads a manifest from a JSON file
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return &m, nil
}

// GetCaseNumbers returns all case numbers in the manifest
func (m *Manifest) GetCaseNumbers() []string {
	numbers := make([]string, len(m.Cases))
	for i, c := range m.Cases {
		numbers[i] = c.CaseNumber
	}
	return numbers
}

// FindCase finds a case in the manifest by number
func (m *Manifest) FindCase(caseNumber string) *ManifestCase {
	for i := range m.Cases {
		if m.Cases[i].CaseNumber == caseNumber {
			return &m.Cases[i]
		}
	}
	return nil
}
