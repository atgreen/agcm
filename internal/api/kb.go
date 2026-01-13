package api

import (
	"context"
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

// Search performs a global search across solutions and articles
func (c *Client) Search(ctx context.Context, keyword string, limit int) ([]SearchResult, error) {
	query := url.Values{}
	query.Set("keyword", keyword)
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}

	var result struct {
		SearchResults []SearchResult `json:"searchResult"`
	}
	if err := c.get(ctx, "/rs/search", query, &result); err != nil {
		return nil, err
	}
	return result.SearchResults, nil
}
