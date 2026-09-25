// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/jaytaylor/html2text"
)

// FlexTime wraps time.Time with flexible JSON parsing for the v3 API's
// Salesforce date format ("2006-01-02T15:04:05.000+0000").
type FlexTime struct {
	time.Time
}

var flexTimeFormats = []string{
	"2006-01-02T15:04:05.000+0000",
	"2006-01-02T15:04:05.000-0000",
	time.RFC3339,
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05.000Z",
}

func (ft *FlexTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		ft.Time = time.Time{}
		return nil
	}
	for _, layout := range flexTimeFormats {
		if t, err := time.Parse(layout, s); err == nil {
			ft.Time = t
			return nil
		}
	}
	return fmt.Errorf("cannot parse time %q", s)
}

// Case represents a Red Hat support case (v3 API)
type Case struct {
	CaseNumber    string       `json:"caseNumber"`
	Summary       string       `json:"summary"`
	Description   string       `json:"description"`
	Status        string       `json:"status"`
	Severity      string       `json:"severity"`
	Product       string       `json:"product"`
	Version       string       `json:"version"`
	Type          string       `json:"caseType"`
	AccountNumber string       `json:"accountNumberRef"`
	ContactName   string       `json:"contactName"`
	ContactEmail  string       `json:"emailAddress"`
	Owner         string       `json:"ownerId"`
	CreatedBy     string       `json:"createdById"`
	CreatedDate   FlexTime     `json:"createdDate"`
	LastModified  FlexTime     `json:"lastModifiedDate"`
	ClosedDate    *FlexTime    `json:"lastClosedAt,omitempty"`
	IsClosed      bool         `json:"isClosed,omitempty"`
	GroupNumber   string       `json:"groupNumber,omitempty"`
	GroupName     string       `json:"groupName,omitempty"`
	Comments      []Comment    `json:"comments,omitempty"`
	Attachments   []Attachment `json:"attachments,omitempty"`
	URI           string       `json:"uri,omitempty"`
}

// Comment represents a comment on a support case (v3 API)
type Comment struct {
	ID            string   `json:"id"`
	CaseNumber    string   `json:"caseNumber,omitempty"`
	CommentBody   string   `json:"commentBody"`
	Author        string   `json:"createdBy"`
	CreatedByType string   `json:"createdByType,omitempty"`
	CreatedDate   FlexTime `json:"createdDate"`
	LastModified  FlexTime `json:"lastModifiedDate,omitempty"`
	Draft         bool     `json:"isDraft"`
	ContentType   string   `json:"contentType,omitempty"`
}

// IsPublicComment returns true if the comment is from a Customer or Associate (not Internal)
func (c *Comment) IsPublicComment() bool {
	return c.CreatedByType != "Internal"
}

// GetText returns the comment body as plain text, converting from HTML if needed.
func (c *Comment) GetText() string {
	if !strings.Contains(c.CommentBody, "<") {
		return c.CommentBody
	}
	text, err := html2text.FromString(c.CommentBody, html2text.Options{OmitLinks: true})
	if err != nil {
		return c.CommentBody
	}
	return text
}

// Attachment represents a file attached to a case (v3 API)
type Attachment struct {
	UUID         string   `json:"uuid"`
	Filename     string   `json:"fileName"`
	Description  string   `json:"description,omitempty"`
	Size         int64    `json:"size"`
	CreatedBy    string   `json:"createdBy"`
	CreatedDate  FlexTime `json:"createdDate"`
	LastModified FlexTime `json:"lastModifiedDate,omitempty"`
	Link         string   `json:"link,omitempty"`
}

// Solution represents a knowledge base solution
type Solution struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Abstract     string    `json:"abstract,omitempty"`
	Body         string    `json:"body,omitempty"`
	Issue        string    `json:"issue,omitempty"`
	Environment  string    `json:"environment,omitempty"`
	Resolution   string    `json:"resolution,omitempty"`
	RootCause    string    `json:"rootCause,omitempty"`
	CreatedDate  time.Time `json:"createdDate"`
	LastModified time.Time `json:"lastModifiedDate"`
	Published    bool      `json:"published"`
	URI          string    `json:"uri,omitempty"`
}

// Article represents a knowledge base article
type Article struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Abstract     string    `json:"abstract,omitempty"`
	Body         string    `json:"body,omitempty"`
	CreatedDate  time.Time `json:"createdDate"`
	LastModified time.Time `json:"lastModifiedDate"`
	Published    bool      `json:"published"`
	URI          string    `json:"uri,omitempty"`
}

// CaseFilter contains filter options for listing cases
type CaseFilter struct {
	Status        []string   `json:"status,omitempty"`
	Severity      []string   `json:"severity,omitempty"`
	Products      []string   `json:"products,omitempty"`
	Keyword       string     `json:"keyword,omitempty"`
	StartDate     *time.Time `json:"startDate,omitempty"`
	EndDate       *time.Time `json:"endDate,omitempty"`
	Count         int        `json:"count,omitempty"`
	StartIndex    int        `json:"startIndex,omitempty"`
	IncludeClosed bool       `json:"includeClosed,omitempty"`
	Accounts      []string   `json:"accounts,omitempty"`     // Filter by account number(s)
	GroupNumber   string     `json:"groupNumber,omitempty"`  // Filter by case group
	OwnerSSOName  string     `json:"ownerSSOName,omitempty"` // Filter by owner
	Cursor        string     `json:"cursor,omitempty"`       // GraphQL cursor for pagination
}

// SearchResult represents a search result item
type SearchResult struct {
	Type     string  `json:"type"` // "case", "solution", "article"
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Abstract string  `json:"abstract,omitempty"`
	URI      string  `json:"uri,omitempty"`
	Score    float64 `json:"score,omitempty"`
}

// ListResponse is a generic paginated response
type ListResponse[T any] struct {
	Items      []T    `json:"items"`
	TotalCount int    `json:"totalCount"`
	StartIndex int    `json:"startIndex"`
	Count      int    `json:"count"`
	NextCursor string `json:"nextCursor,omitempty"`
}
