// SPDX-License-Identifier: GPL-3.0-or-later
package api

import (
	"context"
	"fmt"
	"net/url"
)

// ListProducts retrieves all available products
func (c *Client) ListProducts(ctx context.Context) ([]Product, error) {
	var result struct {
		Products []Product `json:"product"`
	}
	if err := c.get(ctx, "/rs/products", nil, &result); err != nil {
		return nil, err
	}
	return result.Products, nil
}

// GetProductVersions retrieves versions for a specific product
func (c *Client) GetProductVersions(ctx context.Context, productName string) ([]string, error) {
	var result struct {
		Versions []string `json:"version"`
	}
	path := fmt.Sprintf("/rs/products/%s/versions", url.PathEscape(productName))
	if err := c.get(ctx, path, nil, &result); err != nil {
		return nil, err
	}
	return result.Versions, nil
}

// ListEntitlements retrieves all entitlements for the account
func (c *Client) ListEntitlements(ctx context.Context) ([]Entitlement, error) {
	var result struct {
		Entitlements []Entitlement `json:"entitlement"`
	}
	if err := c.get(ctx, "/rs/entitlements", nil, &result); err != nil {
		return nil, err
	}
	return result.Entitlements, nil
}

// ListGroups retrieves case groups
func (c *Client) ListGroups(ctx context.Context) ([]Group, error) {
	var result struct {
		Groups []Group `json:"group"`
	}
	if err := c.get(ctx, "/rs/groups", nil, &result); err != nil {
		return nil, err
	}
	return result.Groups, nil
}

// Group represents a case group
type Group struct {
	Number string `json:"number"`
	Name   string `json:"name"`
	URI    string `json:"uri,omitempty"`
}

// GetUser retrieves user information by SSO username
func (c *Client) GetUser(ctx context.Context, ssoUserName string) (*User, error) {
	query := url.Values{}
	query.Set("ssoUserName", ssoUserName)

	var result User
	if err := c.get(ctx, "/rs/users", query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// User represents a Red Hat user
type User struct {
	SSOUsername string `json:"ssoUserName"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	Email       string `json:"email"`
	OrgAdmin    bool   `json:"orgAdmin"`
}
