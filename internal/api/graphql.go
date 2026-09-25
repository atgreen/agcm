// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const graphQLBaseURL = "https://graphql.redhat.com"

const listCasesQuery = `query ListCases($first: Int, $after: String, $where: RedHatSupportCase_Filter) {
  redhat_support_uiapi {
    query {
      RedHatSupportCase(
        first: $first
        after: $after
        where: $where
        orderBy: { LastModifiedDate: { order: DESC } }
      ) {
        totalCount
        edges {
          node {
            CaseNumber__c { value }
            Subject { value }
            Status { value }
            Priority { value }
            CreatedDate { value }
            LastModifiedDate { value }
            Product {
              Name { value }
            }
          }
          cursor
        }
        pageInfo {
          hasNextPage
          endCursor
        }
      }
    }
  }
}`

type graphQLRequest struct {
	Query     string      `json:"query"`
	Variables interface{} `json:"variables,omitempty"`
}

type graphQLResponse struct {
	Data struct {
		RedhatSupportUiapi struct {
			Query struct {
				RedHatSupportCase gqlCaseConnection `json:"RedHatSupportCase"`
			} `json:"query"`
		} `json:"redhat_support_uiapi"`
	} `json:"data"`
	Errors []graphQLError `json:"errors,omitempty"`
}

type graphQLError struct {
	Message string `json:"message"`
}

type gqlCaseConnection struct {
	TotalCount int           `json:"totalCount"`
	Edges      []gqlCaseEdge `json:"edges"`
	PageInfo   gqlPageInfo   `json:"pageInfo"`
}

type gqlCaseEdge struct {
	Node   gqlCaseNode `json:"node"`
	Cursor string      `json:"cursor"`
}

type gqlPageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type gqlValue struct {
	Value string `json:"value"`
}

type gqlCaseNode struct {
	CaseNumber   gqlValue    `json:"CaseNumber__c"`
	Subject      gqlValue    `json:"Subject"`
	Status       gqlValue    `json:"Status"`
	Priority     gqlValue    `json:"Priority"`
	CreatedDate  gqlValue    `json:"CreatedDate"`
	LastModified gqlValue    `json:"LastModifiedDate"`
	Product      *gqlProduct `json:"Product"`
}

type gqlProduct struct {
	Name gqlValue `json:"Name"`
}

func (n *gqlCaseNode) toCase() Case {
	c := Case{
		CaseNumber:   n.CaseNumber.Value,
		Summary:      n.Subject.Value,
		Status:       n.Status.Value,
		Severity:     n.Priority.Value,
		CreatedDate:  parseFlexTime(n.CreatedDate.Value),
		LastModified: parseFlexTime(n.LastModified.Value),
	}
	if n.Product != nil {
		c.Product = n.Product.Name.Value
	}
	return c
}

func parseFlexTime(s string) FlexTime {
	if s == "" {
		return FlexTime{}
	}
	for _, layout := range flexTimeFormats {
		if t, err := time.Parse(layout, s); err == nil {
			return FlexTime{t}
		}
	}
	return FlexTime{}
}

// postGraphQL sends a GraphQL request to the Red Hat GraphQL endpoint.
func (c *Client) postGraphQL(ctx context.Context, gqlReq *graphQLRequest) (*graphQLResponse, error) {
	jsonBytes, err := json.Marshal(gqlReq)
	if err != nil {
		return nil, fmt.Errorf("failed to encode GraphQL request: %w", err)
	}

	token, err := c.getToken(ctx)
	if err != nil {
		return nil, err
	}

	makeReq := func(tok string) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, graphQLBaseURL, bytes.NewReader(jsonBytes))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("apollographql-client-name", "agcm")
		req.Header.Set("apollographql-client-version", "1.0")
		return req, nil
	}

	c.debugf("[%s] POST %s\n", time.Now().Format("15:04:05"), graphQLBaseURL)
	c.debugf("  Request: %s\n", string(jsonBytes))

	req, err := makeReq(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GraphQL request failed: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized && c.TokenRefresher != nil {
		_ = resp.Body.Close()
		newToken, err := c.TokenRefresher(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to refresh token: %w", err)
		}
		c.SetToken(newToken)
		req, err = makeReq(newToken)
		if err != nil {
			return nil, fmt.Errorf("failed to create retry request: %w", err)
		}
		resp, err = c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("GraphQL retry failed: %w", err)
		}
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read GraphQL response: %w", err)
	}

	c.debugResponse(resp.StatusCode, body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GraphQL error %d: %s", resp.StatusCode, string(body))
	}

	var result graphQLResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode GraphQL response: %w", err)
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL error: %s", result.Errors[0].Message)
	}

	return &result, nil
}

// buildCaseWhere constructs a GraphQL where filter from a CaseFilter.
func buildCaseWhere(filter *CaseFilter) map[string]interface{} {
	if filter == nil {
		return nil
	}

	var conditions []map[string]interface{}

	if len(filter.Status) > 0 {
		statuses := normalizeStatuses(filter.Status)
		if len(statuses) == 1 {
			conditions = append(conditions, map[string]interface{}{
				"Status": map[string]interface{}{"eq": statuses[0]},
			})
		} else {
			conditions = append(conditions, map[string]interface{}{
				"Status": map[string]interface{}{"in": statuses},
			})
		}
	} else if !filter.IncludeClosed {
		conditions = append(conditions, map[string]interface{}{
			"Status": map[string]interface{}{"ne": "Closed"},
		})
	}

	if len(filter.Severity) > 0 {
		conditions = append(conditions, map[string]interface{}{
			"Priority": map[string]interface{}{"in": filter.Severity},
		})
	}

	if filter.Keyword != "" {
		conditions = append(conditions, map[string]interface{}{
			"Subject": map[string]interface{}{"like": "%" + filter.Keyword + "%"},
		})
	}

	if filter.StartDate != nil {
		conditions = append(conditions, map[string]interface{}{
			"CreatedDate": map[string]interface{}{
				"gt": map[string]interface{}{"value": filter.StartDate.Format(time.RFC3339)},
			},
		})
	}
	if filter.EndDate != nil {
		conditions = append(conditions, map[string]interface{}{
			"CreatedDate": map[string]interface{}{
				"lt": map[string]interface{}{"value": filter.EndDate.Format(time.RFC3339)},
			},
		})
	}

	if len(filter.Accounts) > 0 {
		if len(filter.Accounts) == 1 {
			conditions = append(conditions, map[string]interface{}{
				"RedHatSupportAccount": map[string]interface{}{
					"AccountNumber": map[string]interface{}{"eq": filter.Accounts[0]},
				},
			})
		} else {
			var orConds []map[string]interface{}
			for _, acct := range filter.Accounts {
				orConds = append(orConds, map[string]interface{}{
					"RedHatSupportAccount": map[string]interface{}{
						"AccountNumber": map[string]interface{}{"eq": acct},
					},
				})
			}
			conditions = append(conditions, map[string]interface{}{"or": orConds})
		}
	}

	// Default 2-year lookback when no scoping filters are applied.
	// GroupNumber and OwnerSSOName are not supported by the GraphQL
	// endpoint, so they don't count as scope here.
	hasScope := len(filter.Accounts) > 0 || filter.Keyword != ""
	if !hasScope && filter.StartDate == nil && filter.EndDate == nil {
		twoYearsAgo := time.Now().AddDate(-2, 0, 0)
		conditions = append(conditions, map[string]interface{}{
			"LastModifiedDate": map[string]interface{}{
				"gt": map[string]interface{}{"value": twoYearsAgo.Format(time.RFC3339)},
			},
		})
	}

	if len(conditions) == 0 {
		return nil
	}
	if len(conditions) == 1 {
		return conditions[0]
	}
	return map[string]interface{}{"and": conditions}
}
