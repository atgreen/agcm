// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package api

import "time"

// Case represents a Red Hat support case
type Case struct {
	CaseNumber    string       `json:"caseNumber"`
	Summary       string       `json:"summary"`
	Description   string       `json:"description"`
	Status        string       `json:"status"`
	Severity      string       `json:"severity"`
	Product       string       `json:"product"`
	Version       string       `json:"version"`
	Type          string       `json:"type"`
	AccountNumber string       `json:"accountNumber"`
	AccountName   string       `json:"accountName"`
	ContactName   string       `json:"contactName"`
	ContactEmail  string       `json:"contactEmail"`
	Owner         string       `json:"owner"`
	CreatedBy     string       `json:"createdBy"`
	CreatedDate   time.Time    `json:"createdDate"`
	LastModified  time.Time    `json:"lastModifiedDate"`
	ClosedDate    *time.Time   `json:"closedDate,omitempty"`
	Comments      []Comment    `json:"comments,omitempty"`
	Attachments   []Attachment `json:"attachments,omitempty"`
	URI           string       `json:"uri,omitempty"`
}

// Comment represents a comment on a support case
type Comment struct {
	ID           string    `json:"id"`
	CaseNumber   string    `json:"caseNumber,omitempty"`
	Text         string    `json:"text"`
	CommentBody  string    `json:"commentBody"`  // Alternative field name
	Author       string    `json:"createdBy"`
	AuthorEmail  string    `json:"createdByEmail,omitempty"`
	CreatedDate  time.Time `json:"createdDate"`
	LastModified time.Time `json:"lastModifiedDate,omitempty"`
	Public       bool      `json:"public"`
	IsPublic     bool      `json:"isPublic"`     // Alternative field name
	CasePublic   bool      `json:"casePublic"`   // Another alternative
	Draft        bool      `json:"draft"`
	URI          string    `json:"uri,omitempty"`
}

// IsPublicComment returns true if the comment is public (checks multiple field names)
func (c *Comment) IsPublicComment() bool {
	return c.Public || c.IsPublic || c.CasePublic
}

// GetText returns the comment text (checks multiple field names)
func (c *Comment) GetText() string {
	if c.CommentBody != "" {
		return c.CommentBody
	}
	return c.Text
}

// Attachment represents a file attached to a case
type Attachment struct {
	UUID         string    `json:"uuid"`
	Filename     string    `json:"fileName"`
	Description  string    `json:"description,omitempty"`
	Length       int64     `json:"length"`
	Size         int64     `json:"size"`
	FileSize     int64     `json:"fileSize"`
	ContentLength int64    `json:"contentLength"`
	MimeType     string    `json:"mimeType,omitempty"`
	CreatedBy    string    `json:"createdBy"`
	CreatedDate  time.Time `json:"createdDate"`
	LastModified time.Time `json:"lastModifiedDate,omitempty"`
	URI          string    `json:"uri,omitempty"`
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

// Product represents a Red Hat product
type Product struct {
	Name     string   `json:"name"`
	Code     string   `json:"code,omitempty"`
	Versions []string `json:"versions,omitempty"`
}

// Entitlement represents a support entitlement
type Entitlement struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	SLA         string    `json:"sla,omitempty"`
	StartDate   time.Time `json:"startDate"`
	EndDate     time.Time `json:"endDate"`
	ServiceType string    `json:"serviceType,omitempty"`
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
	Accounts      []string   `json:"accounts,omitempty"`    // Filter by account number(s)
	GroupNumber   string     `json:"groupNumber,omitempty"` // Filter by case group
	OwnerSSOName  string     `json:"ownerSSOName,omitempty"` // Filter by owner
}

// SearchResult represents a search result item
type SearchResult struct {
	Type        string `json:"type"` // "case", "solution", "article"
	ID          string `json:"id"`
	Title       string `json:"title"`
	Abstract    string `json:"abstract,omitempty"`
	URI         string `json:"uri,omitempty"`
	Score       float64 `json:"score,omitempty"`
}

// ListResponse is a generic paginated response
type ListResponse[T any] struct {
	Items      []T `json:"items"`
	TotalCount int `json:"totalCount"`
	StartIndex int `json:"startIndex"`
	Count      int `json:"count"`
}

// CaseValues contains reference values for cases
type CaseValues struct {
	Types      []string `json:"types"`
	Severities []string `json:"severities"`
	Statuses   []string `json:"statuses"`
}
