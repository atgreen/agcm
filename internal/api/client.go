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
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	DefaultBaseURL = "https://api.access.redhat.com"
	DefaultTimeout = 30 * time.Second
)

// Client is the Red Hat Customer Portal API client
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
	tokenMu    sync.RWMutex
	debug      bool
	debugFile  *os.File

	// TokenRefresher is called when a new access token is needed
	TokenRefresher func(ctx context.Context) (string, error)
}

// ClientOption configures the Client
type ClientOption func(*Client)

// WithBaseURL sets a custom base URL
func WithBaseURL(url string) ClientOption {
	return func(c *Client) {
		c.baseURL = strings.TrimSuffix(url, "/")
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// WithToken sets the initial access token
func WithToken(token string) ClientOption {
	return func(c *Client) {
		c.token = token
	}
}

// WithTokenRefresher sets the token refresh function
func WithTokenRefresher(fn func(ctx context.Context) (string, error)) ClientOption {
	return func(c *Client) {
		c.TokenRefresher = fn
	}
}

// WithDebug enables debug output to /tmp/agcm-debug.log
func WithDebug(debug bool) ClientOption {
	return func(c *Client) {
		c.debug = debug
		if debug {
			f, err := os.OpenFile("/tmp/agcm-debug.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
			if err == nil {
				c.debugFile = f
			}
		}
	}
}

// NewClient creates a new API client
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		baseURL: DefaultBaseURL,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// SetToken updates the access token
func (c *Client) SetToken(token string) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	c.token = token
}

// getToken returns the current token, refreshing if needed
func (c *Client) getToken(ctx context.Context) (string, error) {
	c.tokenMu.RLock()
	token := c.token
	c.tokenMu.RUnlock()

	if token == "" && c.TokenRefresher != nil {
		newToken, err := c.TokenRefresher(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to refresh token: %w", err)
		}
		c.SetToken(newToken)
		return newToken, nil
	}
	return token, nil
}

// do performs an HTTP request with authentication
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body io.Reader) (*http.Response, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return nil, err
	}

	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	// Read body for debug if needed
	var bodyBytes []byte
	if body != nil && c.debug {
		bodyBytes, _ = io.ReadAll(body)
		body = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.debug && c.debugFile != nil {
		fmt.Fprintf(c.debugFile, "[%s] %s %s\n", time.Now().Format("15:04:05"), method, u)
		if len(bodyBytes) > 0 {
			fmt.Fprintf(c.debugFile, "  Request: %s\n", string(bodyBytes))
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Handle token expiration
	if resp.StatusCode == http.StatusUnauthorized && c.TokenRefresher != nil {
		resp.Body.Close()
		newToken, err := c.TokenRefresher(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to refresh token: %w", err)
		}
		c.SetToken(newToken)

		// Retry with new token
		req, err = http.NewRequestWithContext(ctx, method, u, body)
		if err != nil {
			return nil, fmt.Errorf("failed to create retry request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+newToken)
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err = c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("retry request failed: %w", err)
		}
	}

	return resp, nil
}

// get performs a GET request and decodes the response
func (c *Client) get(ctx context.Context, path string, query url.Values, result interface{}) error {
	body, err := c.getRaw(ctx, path, query)
	if err != nil {
		return err
	}

	if result != nil {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}
	return nil
}

// getRaw performs a GET request and returns the raw response body
func (c *Client) getRaw(ctx context.Context, path string, query url.Values) ([]byte, error) {
	resp, err := c.do(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		if c.debug && c.debugFile != nil {
			fmt.Fprintf(c.debugFile, "  Response: %d %s\n", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	if c.debug && c.debugFile != nil {
		preview := string(body)
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		fmt.Fprintf(c.debugFile, "  Response: %d (%d bytes): %s\n", resp.StatusCode, len(body), preview)
	}

	return body, nil
}

// post performs a POST request and decodes the response
func (c *Client) post(ctx context.Context, path string, requestBody interface{}, result interface{}) error {
	var body io.Reader
	if requestBody != nil {
		jsonBytes, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("failed to encode request body: %w", err)
		}
		body = bytes.NewReader(jsonBytes)
	}

	resp, err := c.do(ctx, http.MethodPost, path, nil, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if c.debug && c.debugFile != nil {
			fmt.Fprintf(c.debugFile, "  Response: %d %s\n", resp.StatusCode, string(respBody))
		}
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	if c.debug && c.debugFile != nil {
		preview := string(respBody)
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		fmt.Fprintf(c.debugFile, "  Response: %d (%d bytes): %s\n", resp.StatusCode, len(respBody), preview)
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}
	return nil
}

// postHydra performs a POST request to the Hydra API (different base URL)
func (c *Client) postHydra(ctx context.Context, path string, body io.Reader, result interface{}) error {
	// Hydra API uses access.redhat.com instead of api.access.redhat.com
	hydraURL := "https://access.redhat.com" + path

	var bodyBytes []byte
	if body != nil && c.debug {
		bodyBytes, _ = io.ReadAll(body)
		body = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hydraURL, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Get token
	c.tokenMu.RLock()
	token := c.token
	c.tokenMu.RUnlock()

	if token == "" && c.TokenRefresher != nil {
		var err error
		token, err = c.TokenRefresher(ctx)
		if err != nil {
			return fmt.Errorf("failed to refresh token: %w", err)
		}
		c.tokenMu.Lock()
		c.token = token
		c.tokenMu.Unlock()
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	if c.debug && c.debugFile != nil {
		fmt.Fprintf(c.debugFile, "[%s] POST %s\n", time.Now().Format("15:04:05"), hydraURL)
		if len(bodyBytes) > 0 {
			fmt.Fprintf(c.debugFile, "  Request: %s\n", string(bodyBytes))
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if c.debug && c.debugFile != nil {
		preview := string(respBody)
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		fmt.Fprintf(c.debugFile, "  Response: %d (%d bytes): %s\n", resp.StatusCode, len(respBody), preview)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Hydra API error %d: %s", resp.StatusCode, string(respBody))
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}
	return nil
}

// DownloadAttachment downloads an attachment and returns the content
func (c *Client) DownloadAttachment(ctx context.Context, caseNumber, uuid string) (io.ReadCloser, string, error) {
	path := fmt.Sprintf("/support/v1/cases/%s/attachments/%s", caseNumber, uuid)
	resp, err := c.do(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, "", err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, "", fmt.Errorf("failed to download attachment: status %d", resp.StatusCode)
	}

	filename := ""
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if i := strings.Index(cd, "filename="); i != -1 {
			filename = strings.Trim(cd[i+9:], `"`)
		}
	}

	return resp.Body, filename, nil
}
