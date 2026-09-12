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

// solrClause builds a Solr field query matching any of the given values,
// e.g. case_status:("Open" OR "Closed")
func solrClause(field string, values []string) string {
	if len(values) == 1 {
		return fmt.Sprintf("%s:%q", field, values[0])
	}
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = fmt.Sprintf("%q", v)
	}
	return fmt.Sprintf("%s:(%s)", field, strings.Join(quoted, " OR "))
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
		if len(filter.Status) > 0 {
			fqParts = append(fqParts, solrClause("case_status", filter.Status))
		}
		if len(filter.Severity) > 0 {
			fqParts = append(fqParts, solrClause("case_severity", filter.Severity))
		}
		if len(filter.Products) > 0 {
			fqParts = append(fqParts, solrClause("case_product", filter.Products))
		}
		if len(filter.Accounts) > 0 {
			fqParts = append(fqParts, solrClause("case_accountNumber", filter.Accounts))
		}
		if filter.GroupNumber != "" {
			fqParts = append(fqParts, fmt.Sprintf("case_groupNumber:%q", filter.GroupNumber))
		}
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

		cs := Case{
			CaseNumber:    doc.CaseNumber,
			Summary:       doc.CaseSummary,
			Status:        doc.CaseStatus,
			Severity:      doc.CaseSeverity,
			Product:       product,
			Version:       doc.CaseVersion,
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

// GetCaseComments retrieves all comments for a case
func (c *Client) GetCaseComments(ctx context.Context, caseNumber string) ([]Comment, error) {
	body, err := c.getRaw(ctx, fmt.Sprintf("/support/v1/cases/%s/comments", caseNumber), nil)
	if err != nil {
		return nil, err
	}

	// Debug: write first comment's raw JSON to see field names
	if c.debugFile != nil && len(body) > 0 {
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
