// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package api

import (
	"context"
	"fmt"
)

// GetSolution retrieves a single solution by ID
func (c *Client) GetSolution(ctx context.Context, solutionID string) (*Solution, error) {
	var result Solution
	if err := c.get(ctx, fmt.Sprintf("/rs/solutions/%s", solutionID), nil, &result); err != nil {
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
			ID               string   `json:"id"`
			AllTitle         string   `json:"allTitle"`
			Abstract         string   `json:"abstract,omitempty"`
			DocumentKind     string   `json:"documentKind"`
			URI              string   `json:"uri,omitempty"`
			View_URI         string   `json:"view_uri,omitempty"`
			PublishedTitle   string   `json:"publishedTitle,omitempty"`
			PortalTags       []string `json:"portal_tags,omitempty"`
			LastModifiedDate string   `json:"lastModifiedDate,omitempty"`
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

// SearchCases searches for cases by keyword via the v3 filter API
func (c *Client) SearchCases(ctx context.Context, keyword string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	filter := &CaseFilter{
		Keyword:       keyword,
		Count:         limit,
		IncludeClosed: true,
	}
	resp, err := c.ListCases(ctx, filter)
	if err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(resp.Items))
	for _, cs := range resp.Items {
		results = append(results, SearchResult{
			Type:  "case",
			ID:    cs.CaseNumber,
			Title: cs.Summary,
		})
	}

	return results, nil
}
