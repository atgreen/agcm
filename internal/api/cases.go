// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

// HydraSearchRequest is the request body for the Hydra search API
type HydraSearchRequest struct {
	Query         string `json:"q"`
	Start         int    `json:"start"`
	Rows          int    `json:"rows"`
	PartnerSearch bool   `json:"partnerSearch"`
	Expression    string `json:"expression"`
}

// HydraSearchResponse is the response from the Hydra search API
type HydraSearchResponse struct {
	Response struct {
		NumFound int        `json:"numFound"`
		Start    int        `json:"start"`
		Docs     []HydraDoc `json:"docs"`
	} `json:"response"`
}

// HydraDoc represents a case document from Hydra search
type HydraDoc struct {
	CaseNumber       string   `json:"case_number"`
	CaseSummary      string   `json:"case_summary"`
	CaseStatus       string   `json:"case_status"`
	CaseProduct      []string `json:"case_product"`
	CaseVersion      string   `json:"case_version"`
	CaseSeverity     string   `json:"case_severity"`
	CaseOwner        string   `json:"case_owner"`
	CaseAccountNum   string   `json:"case_accountNumber"`
	CaseContactName  string   `json:"case_contactName"`
	CaseCreatedDate  string   `json:"case_createdDate"`
	CaseCreatedBy    string   `json:"case_createdByName"`
	CaseModifiedDate string   `json:"case_lastModifiedDate"`
	CaseModifiedBy   string   `json:"case_lastModifiedByName"`
	URI              string   `json:"uri"`
}

// CaseFilterRequest is the request body for the POST /support/v1/cases/filter endpoint (legacy)
type CaseFilterRequest struct {
	MaxResults    int      `json:"maxResults"`
	Offset        int      `json:"offset,omitempty"`
	Status        string   `json:"status,omitempty"`
	Statuses      []string `json:"statuses,omitempty"`
	Severity      string   `json:"severity,omitempty"`
	Severities    []string `json:"severities,omitempty"`
	Product       string   `json:"product,omitempty"`
	Products      []string `json:"products,omitempty"`
	IncludeClosed bool     `json:"includeClosed,omitempty"`
	AccountNumber string   `json:"accountNumber,omitempty"`
	GroupNumber   string   `json:"groupNumber,omitempty"`
	OwnerSSOName  string   `json:"ownerSSOName,omitempty"`
	Keyword       string   `json:"keyword,omitempty"`
	StartDate     string   `json:"startDate,omitempty"`
	EndDate       string   `json:"endDate,omitempty"`
	SortField     string   `json:"sortField,omitempty"`
	SortOrder     string   `json:"sortOrder,omitempty"`
}

// CaseFilterResponse is the response from /support/v1/cases/filter (legacy)
type CaseFilterResponse struct {
	Cases      []Case `json:"cases"`
	TotalCount int    `json:"totalCount"`
	Offset     int    `json:"offset"`
	MaxResults int    `json:"maxResults"`
}

// ListCases retrieves a list of cases with optional filtering
// Uses the Hydra search API for reliable filtering
func (c *Client) ListCases(ctx context.Context, filter *CaseFilter) (*ListResponse[Case], error) {
	rows := 100
	start := 0

	if filter != nil {
		if filter.Count > 0 {
			rows = filter.Count
		}
		if filter.StartIndex > 0 {
			start = filter.StartIndex
		}
	}

	// Build Solr filter query (fq) parts
	var fqParts []string

	if filter != nil {
		// Status filter
		if len(filter.Status) > 0 {
			if len(filter.Status) == 1 {
				fqParts = append(fqParts, fmt.Sprintf("case_status:%q", filter.Status[0]))
			} else {
				// Multiple statuses: case_status:("Open" OR "Closed")
				quoted := make([]string, len(filter.Status))
				for i, s := range filter.Status {
					quoted[i] = fmt.Sprintf("%q", s)
				}
				fqParts = append(fqParts, fmt.Sprintf("case_status:(%s)", strings.Join(quoted, " OR ")))
			}
		}

		// Severity filter - use full severity strings like status filter
		if len(filter.Severity) > 0 {
			if len(filter.Severity) == 1 {
				fqParts = append(fqParts, fmt.Sprintf("case_severity:%q", filter.Severity[0]))
			} else {
				quoted := make([]string, len(filter.Severity))
				for i, s := range filter.Severity {
					quoted[i] = fmt.Sprintf("%q", s)
				}
				fqParts = append(fqParts, fmt.Sprintf("case_severity:(%s)", strings.Join(quoted, " OR ")))
			}
		}

		// Product filter (supports multiple products)
		if len(filter.Products) > 0 {
			if len(filter.Products) == 1 {
				fqParts = append(fqParts, fmt.Sprintf("case_product:%q", filter.Products[0]))
			} else {
				quoted := make([]string, len(filter.Products))
				for i, p := range filter.Products {
					quoted[i] = fmt.Sprintf("%q", p)
				}
				fqParts = append(fqParts, fmt.Sprintf("case_product:(%s)", strings.Join(quoted, " OR ")))
			}
		}

		// Account filter (supports multiple accounts)
		if len(filter.Accounts) > 0 {
			if len(filter.Accounts) == 1 {
				fqParts = append(fqParts, fmt.Sprintf("case_accountNumber:%q", filter.Accounts[0]))
			} else {
				quoted := make([]string, len(filter.Accounts))
				for i, a := range filter.Accounts {
					quoted[i] = fmt.Sprintf("%q", a)
				}
				fqParts = append(fqParts, fmt.Sprintf("case_accountNumber:(%s)", strings.Join(quoted, " OR ")))
			}
		}

		// Group filter
		if filter.GroupNumber != "" {
			fqParts = append(fqParts, fmt.Sprintf("case_groupNumber:%q", filter.GroupNumber))
		}

		// Owner filter
		if filter.OwnerSSOName != "" {
			fqParts = append(fqParts, fmt.Sprintf("case_owner:%q", filter.OwnerSSOName))
		}

		// Date range filters
		if filter.StartDate != nil {
			// Solr date format: 2006-01-02T15:04:05Z
			fqParts = append(fqParts, fmt.Sprintf("case_createdDate:[%s TO *]", filter.StartDate.Format("2006-01-02T15:04:05Z")))
		}
		if filter.EndDate != nil {
			fqParts = append(fqParts, fmt.Sprintf("case_createdDate:[* TO %s]", filter.EndDate.Format("2006-01-02T15:04:05Z")))
		}

		// Exclude closed by default unless IncludeClosed is true
		if !filter.IncludeClosed && len(filter.Status) == 0 {
			fqParts = append(fqParts, "-case_status:\"Closed\"")
		}
	}

	// Build the expression string
	// Field list for case data we need
	fieldList := "case_number,case_summary,case_status,case_product,case_version,case_severity,case_owner,case_accountNumber,case_contactName,case_createdDate,case_createdByName,case_lastModifiedDate,case_lastModifiedByName,uri"

	expression := "sort=case_lastModifiedDate desc&fl=" + url.QueryEscape(fieldList)

	// Add filter queries
	for _, fq := range fqParts {
		expression += "&fq=" + url.QueryEscape(fq)
	}

	// Build query - use keyword if provided, otherwise wildcard
	query := "*:*"
	if filter != nil && filter.Keyword != "" {
		query = filter.Keyword
	}

	req := HydraSearchRequest{
		Query:         query,
		Start:         start,
		Rows:          rows,
		PartnerSearch: false,
		Expression:    expression,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal search request: %w", err)
	}

	var resp HydraSearchResponse
	if err := c.postHydra(ctx, "/hydra/rest/search/v2/cases", bytes.NewReader(body), &resp); err != nil {
		return nil, err
	}

	// Convert Hydra docs to Case structs
	cases := make([]Case, 0, len(resp.Response.Docs))
	for _, doc := range resp.Response.Docs {
		// Get first element from array fields
		product := ""
		if len(doc.CaseProduct) > 0 {
			product = doc.CaseProduct[0]
		}
		version := doc.CaseVersion

		cs := Case{
			CaseNumber:    doc.CaseNumber,
			Summary:       doc.CaseSummary,
			Status:        doc.CaseStatus,
			Severity:      doc.CaseSeverity,
			Product:       product,
			Version:       version,
			AccountNumber: doc.CaseAccountNum,
			ContactName:   doc.CaseContactName,
		}

		// Parse dates
		if doc.CaseCreatedDate != "" {
			if t, err := time.Parse(time.RFC3339, doc.CaseCreatedDate); err == nil {
				cs.CreatedDate = t
			}
		}
		if doc.CaseModifiedDate != "" {
			if t, err := time.Parse(time.RFC3339, doc.CaseModifiedDate); err == nil {
				cs.LastModified = t
			}
		}

		cases = append(cases, cs)
	}

	return &ListResponse[Case]{
		Items:      cases,
		TotalCount: resp.Response.NumFound,
		StartIndex: resp.Response.Start,
		Count:      len(cases),
	}, nil
}

// GetCase retrieves a single case by case number
func (c *Client) GetCase(ctx context.Context, caseNumber string) (*Case, error) {
	var result Case
	if err := c.get(ctx, fmt.Sprintf("/support/v1/cases/%s", caseNumber), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// FilterCases performs advanced case filtering using POST
// This is now the same as ListCases with the new API
func (c *Client) FilterCases(ctx context.Context, filter *CaseFilter) (*ListResponse[Case], error) {
	return c.ListCases(ctx, filter)
}

// GetCaseComments retrieves all comments for a case
func (c *Client) GetCaseComments(ctx context.Context, caseNumber string) ([]Comment, error) {
	body, err := c.getRaw(ctx, fmt.Sprintf("/support/v1/cases/%s/comments", caseNumber), nil)
	if err != nil {
		return nil, err
	}

	// Debug: write first comment's raw JSON to see field names
	if c.debug && c.debugFile != nil && len(body) > 0 {
		// Parse as generic JSON to see actual structure
		var raw []map[string]interface{}
		if json.Unmarshal(body, &raw) == nil && len(raw) > 0 {
			if sample, err := json.MarshalIndent(raw[0], "", "  "); err == nil {
				_, _ = fmt.Fprintf(c.debugFile, "  Sample comment JSON:\n%s\n", string(sample))
			}
		}
	}

	// Try unwrapped array format first (API returns raw array)
	var comments []Comment
	if err := json.Unmarshal(body, &comments); err == nil {
		return comments, nil
	}

	// Try wrapped format {"comments": [...]}
	var wrapped struct {
		Comments []Comment `json:"comments"`
	}
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return nil, fmt.Errorf("failed to decode comments: %w", err)
	}
	return wrapped.Comments, nil
}

// GetCaseAttachments retrieves all attachments for a case
func (c *Client) GetCaseAttachments(ctx context.Context, caseNumber string) ([]Attachment, error) {
	body, err := c.getRaw(ctx, fmt.Sprintf("/support/v1/cases/%s/attachments", caseNumber), nil)
	if err != nil {
		return nil, err
	}

	// Try unwrapped array format first (API returns raw array)
	var attachments []Attachment
	if err := json.Unmarshal(body, &attachments); err == nil {
		return attachments, nil
	}

	// Try wrapped format {"attachments": [...]}
	var wrapped struct {
		Attachments []Attachment `json:"attachments"`
	}
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return nil, fmt.Errorf("failed to decode attachments: %w", err)
	}
	return wrapped.Attachments, nil
}

// GetCaseValues retrieves reference values for cases (types, severities, statuses)
// Note: This may need adjustment based on the new API structure
func (c *Client) GetCaseValues(ctx context.Context) (*CaseValues, error) {
	// The new API may have different endpoints for these values
	// For now, return hardcoded common values
	return &CaseValues{
		Types:      []string{"Bug", "Feature Request", "Documentation", "Other"},
		Severities: []string{"1 (Urgent)", "2 (High)", "3 (Normal)", "4 (Low)"},
		Statuses:   []string{"Open", "Waiting on Red Hat", "Waiting on Customer", "Closed"},
	}, nil
}

// ListCaseProducts retrieves distinct product names from the Hydra case index.
func (c *Client) ListCaseProducts(ctx context.Context) ([]string, error) {
	req := HydraSearchRequest{
		Query:         "*:*",
		Start:         0,
		Rows:          0,
		PartnerSearch: false,
		Expression:    "facet=on&facet.field=case_product&facet.limit=-1&wt=json",
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal product facet request: %w", err)
	}

	var resp struct {
		FacetCounts struct {
			FacetFields map[string][]interface{} `json:"facet_fields"`
		} `json:"facet_counts"`
	}
	if err := c.postHydra(ctx, "/hydra/rest/search/v2/cases", bytes.NewReader(body), &resp); err != nil {
		return nil, err
	}

	values := resp.FacetCounts.FacetFields["case_product"]
	if len(values) == 0 {
		return nil, fmt.Errorf("no product facet data")
	}

	products := make([]string, 0, len(values)/2)
	for i := 0; i+1 < len(values); i += 2 {
		name, ok := values[i].(string)
		if !ok || name == "" {
			continue
		}
		products = append(products, name)
	}
	sort.Strings(products)
	return products, nil
}
