// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// v1StatusToV3 maps legacy v1 API status names to their v3 equivalents.
var v1StatusToV3 = map[string]string{
	"open":                "In Progress",
	"waiting on red hat":  "In Progress",
	"waiting on customer": "Waiting on Customer Action Required",
	"closed":              "Closed",
}

// normalizeStatuses translates any v1-era status names to their v3 equivalents.
func normalizeStatuses(statuses []string) []string {
	out := make([]string, 0, len(statuses))
	for _, s := range statuses {
		if mapped, ok := v1StatusToV3[strings.ToLower(s)]; ok {
			out = append(out, mapped)
		} else {
			out = append(out, s)
		}
	}
	// Deduplicate (e.g. "Open" and "Waiting on Red Hat" both map to "In Progress")
	seen := make(map[string]bool, len(out))
	deduped := out[:0]
	for _, s := range out {
		if !seen[s] {
			seen[s] = true
			deduped = append(deduped, s)
		}
	}
	return deduped
}

// ListCases retrieves cases via the Red Hat GraphQL API, requesting only
// the fields needed for the case list view. Individual case detail,
// comments, and attachments are still fetched via REST v3.
func (c *Client) ListCases(ctx context.Context, filter *CaseFilter) (*ListResponse[Case], error) {
	wanted := 25
	if filter != nil && filter.Count > 0 {
		wanted = filter.Count
	}
	if wanted > 200 {
		wanted = 200
	}

	where := buildCaseWhere(filter)

	vars := map[string]interface{}{
		"first": wanted,
	}
	if filter != nil && filter.Cursor != "" {
		vars["after"] = filter.Cursor
	}
	if where != nil {
		vars["where"] = where
	}

	resp, err := c.postGraphQL(ctx, &graphQLRequest{
		Query:     listCasesQuery,
		Variables: vars,
	})
	if err != nil {
		return nil, err
	}

	conn := resp.Data.RedhatSupportUiapi.Query.RedHatSupportCase
	cases := make([]Case, 0, len(conn.Edges))
	for _, edge := range conn.Edges {
		cases = append(cases, edge.Node.toCase())
	}

	// Client-side product filtering: the product name includes the
	// version, so we prefix-match against the user's filter.
	items := cases
	if filter != nil && len(filter.Products) > 0 {
		filtered := make([]Case, 0, len(items))
		for _, cs := range items {
			for _, p := range filter.Products {
				if strings.EqualFold(cs.Product, p) || strings.HasPrefix(strings.ToLower(cs.Product), strings.ToLower(p)+" ") {
					filtered = append(filtered, cs)
					break
				}
			}
		}
		items = filtered
	}

	nextCursor := ""
	if conn.PageInfo.HasNextPage {
		nextCursor = conn.PageInfo.EndCursor
	}

	startIndex := 0
	if filter != nil {
		startIndex = filter.StartIndex
	}

	return &ListResponse[Case]{
		Items:      items,
		TotalCount: conn.TotalCount,
		StartIndex: startIndex,
		Count:      len(items),
		NextCursor: nextCursor,
	}, nil
}

// GetCase retrieves a single case by case number via the v3 API
func (c *Client) GetCase(ctx context.Context, caseNumber string) (*Case, error) {
	var result Case
	if err := c.get(ctx, fmt.Sprintf("/support/v3/cases/%s", caseNumber), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetCaseComments retrieves all comments for a case via the v3 API
func (c *Client) GetCaseComments(ctx context.Context, caseNumber string) ([]Comment, error) {
	query := url.Values{}
	query.Set("sortField", "createdDate")
	query.Set("sortOrder", "asc")

	body, err := c.getRaw(ctx, fmt.Sprintf("/support/v3/cases/%s/comments", caseNumber), query)
	if err != nil {
		return nil, err
	}

	if c.debugFile != nil && len(body) > 0 {
		var raw []map[string]interface{}
		if json.Unmarshal(body, &raw) == nil && len(raw) > 0 {
			if sample, err := json.MarshalIndent(raw[0], "", "  "); err == nil {
				_, _ = fmt.Fprintf(c.debugFile, "  Sample comment JSON:\n%s\n", string(sample))
			}
		}
	}

	var comments []Comment
	if err := json.Unmarshal(body, &comments); err != nil {
		return nil, fmt.Errorf("failed to decode comments: %w", err)
	}
	return comments, nil
}

// GetCaseAttachments retrieves all attachments for a case via the v3 API
func (c *Client) GetCaseAttachments(ctx context.Context, caseNumber string) ([]Attachment, error) {
	body, err := c.getRaw(ctx, fmt.Sprintf("/support/v3/cases/%s/attachments", caseNumber), nil)
	if err != nil {
		return nil, err
	}

	var attachments []Attachment
	if err := json.Unmarshal(body, &attachments); err != nil {
		return nil, fmt.Errorf("failed to decode attachments: %w", err)
	}
	return attachments, nil
}

// ListCaseProducts retrieves distinct product names from the user's cases.
func (c *Client) ListCaseProducts(ctx context.Context) ([]string, error) {
	filter := &CaseFilter{
		Count:         200,
		IncludeClosed: true,
	}
	result, err := c.ListCases(ctx, filter)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	for _, cs := range result.Items {
		if cs.Product != "" {
			seen[cs.Product] = true
		}
	}

	products := make([]string, 0, len(seen))
	for p := range seen {
		products = append(products, p)
	}
	sort.Strings(products)
	return products, nil
}
