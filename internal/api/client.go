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
	"path/filepath"
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

// WithTimeout sets the HTTP client timeout
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// WithTokenRefresher sets the token refresh function
func WithTokenRefresher(fn func(ctx context.Context) (string, error)) ClientOption {
	return func(c *Client) {
		c.TokenRefresher = fn
	}
}

// WithDebugLog enables debug output to the specified file path.
// Parent directories are created automatically.
func WithDebugLog(path string) ClientOption {
	return func(c *Client) {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err == nil {
			c.debugFile = f
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

// Close releases resources held by the client
func (c *Client) Close() error {
	if c.debugFile != nil {
		err := c.debugFile.Close()
		c.debugFile = nil
		return err
	}
	return nil
}

// DebugFile returns the debug log file, or nil if debug logging is not enabled.
func (c *Client) DebugFile() *os.File {
	return c.debugFile
}

// SetToken updates the access token
func (c *Client) SetToken(token string) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	c.token = token
}

// debugf writes to the debug log when enabled
func (c *Client) debugf(format string, args ...interface{}) {
	if c.debugFile != nil {
		_, _ = fmt.Fprintf(c.debugFile, format, args...)
	}
}

// debugResponse logs a truncated preview of a response body
func (c *Client) debugResponse(status int, body []byte) {
	preview := string(body)
	if len(preview) > 500 {
		preview = preview[:500] + "..."
	}
	c.debugf("  Response: %d (%d bytes): %s\n", status, len(body), preview)
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

	// Read body into bytes so we can retry if needed (body reader is consumed on first request)
	var bodyBytes []byte
	if body != nil {
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

	c.debugf("[%s] %s %s\n", time.Now().Format("15:04:05"), method, u)
	if len(bodyBytes) > 0 {
		c.debugf("  Request: %s\n", string(bodyBytes))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	// Handle token expiration
	if resp.StatusCode == http.StatusUnauthorized && c.TokenRefresher != nil {
		_ = resp.Body.Close()
		newToken, err := c.TokenRefresher(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to refresh token: %w", err)
		}
		c.SetToken(newToken)

		// Retry with new token - recreate body reader since it was consumed
		var retryBody io.Reader
		if len(bodyBytes) > 0 {
			retryBody = bytes.NewReader(bodyBytes)
		}
		req, err = http.NewRequestWithContext(ctx, method, u, retryBody)
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
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		c.debugf("  Response: %d %s\n", resp.StatusCode, string(body))
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	c.debugResponse(resp.StatusCode, body)
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
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.debugf("  Response: %d %s\n", resp.StatusCode, string(respBody))
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	c.debugResponse(resp.StatusCode, respBody)

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}
	return nil
}

// downloadURLResponse is the v3 attachment download response
type downloadURLResponse struct {
	DownloadURL      string `json:"downloadUrl"`
	FileName         string `json:"fileName"`
	ExpiresInSeconds string `json:"expiresInSeconds"`
}

// DownloadAttachment downloads an attachment via the v3 API.
// The v3 endpoint returns a presigned URL which is then fetched.
func (c *Client) DownloadAttachment(ctx context.Context, caseNumber, uuid string) (io.ReadCloser, string, error) {
	var dlResp downloadURLResponse
	path := fmt.Sprintf("/support/v3/cases/attachments/downloadFile/%s/%s", caseNumber, uuid)
	if err := c.get(ctx, path, nil, &dlResp); err != nil {
		return nil, "", fmt.Errorf("failed to get download URL: %w", err)
	}

	if dlResp.DownloadURL == "" {
		return nil, "", fmt.Errorf("empty download URL for attachment %s", uuid)
	}

	c.debugf("[%s] GET %s (presigned)\n", time.Now().Format("15:04:05"), dlResp.DownloadURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dlResp.DownloadURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create download request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download attachment: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, "", fmt.Errorf("failed to download attachment: status %d", resp.StatusCode)
	}

	return resp.Body, dlResp.FileName, nil
}
