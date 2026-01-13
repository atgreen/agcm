// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// SearchSolutions searches for solutions by keyword
func (c *Client) SearchSolutions(ctx context.Context, keyword string, limit int) (*ListResponse[Solution], error) {
	query := url.Values{}
	query.Set("keyword", keyword)
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}

	var result ListResponse[Solution]
	if err := c.get(ctx, "/rs/solutions", query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetSolution retrieves a single solution by ID
func (c *Client) GetSolution(ctx context.Context, solutionID string) (*Solution, error) {
	var result Solution
	if err := c.get(ctx, fmt.Sprintf("/rs/solutions/%s", solutionID), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SearchArticles searches for articles by keyword
func (c *Client) SearchArticles(ctx context.Context, keyword string, limit int) (*ListResponse[Article], error) {
	query := url.Values{}
	query.Set("keyword", keyword)
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}

	var result ListResponse[Article]
	if err := c.get(ctx, "/rs/articles", query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetArticle retrieves a single article by ID
func (c *Client) GetArticle(ctx context.Context, articleID string) (*Article, error) {
	var result Article
	if err := c.get(ctx, fmt.Sprintf("/rs/articles/%s", articleID), nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// KCSSearchRequest is the request body for the KCS search API
type KCSSearchRequest struct {
	Query string `json:"q"`
	Rows  int    `json:"rows,omitempty"`
	Start int    `json:"start,omitempty"`
}

// KCSSearchResponse is the response from the KCS search API
type KCSSearchResponse struct {
	Response struct {
		NumFound int `json:"numFound"`
		Start    int `json:"start"`
		Docs     []struct {
			ID              string   `json:"id"`
			AllTitle        string   `json:"allTitle"`
			Abstract        string   `json:"abstract,omitempty"`
			DocumentKind    string   `json:"documentKind"`
			URI             string   `json:"uri,omitempty"`
			View_URI        string   `json:"view_uri,omitempty"`
			PublishedTitle  string   `json:"publishedTitle,omitempty"`
			PortalTags      []string `json:"portal_tags,omitempty"`
			LastModifiedDate string  `json:"lastModifiedDate,omitempty"`
		} `json:"docs"`
	} `json:"response"`
}

// Search performs a global search across KCS (solutions and articles)
func (c *Client) Search(ctx context.Context, keyword string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	reqBody := KCSSearchRequest{
		Query: keyword,
		Rows:  limit,
	}

	var result KCSSearchResponse
	if err := c.post(ctx, "/support/search/v2/kcs", reqBody, &result); err != nil {
		return nil, err
	}

	// Convert KCS response to SearchResult format
	searchResults := make([]SearchResult, 0, len(result.Response.Docs))
	for _, doc := range result.Response.Docs {
		sr := SearchResult{
			ID:       doc.ID,
			Title:    doc.AllTitle,
			Abstract: doc.Abstract,
			URI:      doc.View_URI,
		}
		if sr.URI == "" {
			sr.URI = doc.URI
		}
		if sr.Title == "" {
			sr.Title = doc.PublishedTitle
		}
		// Determine type from documentKind
		switch doc.DocumentKind {
		case "Solution":
			sr.Type = "solution"
		case "Article":
			sr.Type = "article"
		default:
			sr.Type = "article"
		}
		searchResults = append(searchResults, sr)
	}

	return searchResults, nil
}

// SearchCases searches for cases by keyword
func (c *Client) SearchCases(ctx context.Context, keyword string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	// Build the expression for case search
	fieldList := "case_number,case_summary,case_status,case_severity"
	expression := "sort=case_lastModifiedDate desc&fl=" + url.QueryEscape(fieldList)

	req := HydraSearchRequest{
		Query:         keyword,
		Start:         0,
		Rows:          limit,
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

	// Convert to SearchResult format
	results := make([]SearchResult, 0, len(resp.Response.Docs))
	for _, doc := range resp.Response.Docs {
		results = append(results, SearchResult{
			Type:  "case",
			ID:    doc.CaseNumber,
			Title: doc.CaseSummary,
		})
	}

	return results, nil
}
