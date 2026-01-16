// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	// Red Hat SSO OAuth endpoints
	TokenURL = "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token"
	ClientID = "rhsm-api"

	// Token validity buffer - refresh before expiry
	TokenExpiryBuffer = 60 * time.Second
)

// TokenResponse represents the OAuth token response
type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	Scope            string `json:"scope"`
}

// TokenManager handles OAuth token lifecycle
type TokenManager struct {
	offlineToken string
	accessToken  string
	expiresAt    time.Time
	httpClient   *http.Client
	mu           sync.RWMutex
}

// NewTokenManager creates a new TokenManager
func NewTokenManager(offlineToken string) *TokenManager {
	return &TokenManager{
		offlineToken: offlineToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetOfflineToken updates the offline token
func (tm *TokenManager) SetOfflineToken(token string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.offlineToken = token
	tm.accessToken = ""
	tm.expiresAt = time.Time{}
}

// GetAccessToken returns a valid access token, refreshing if necessary
func (tm *TokenManager) GetAccessToken(ctx context.Context) (string, error) {
	tm.mu.RLock()
	if tm.accessToken != "" && time.Now().Add(TokenExpiryBuffer).Before(tm.expiresAt) {
		token := tm.accessToken
		tm.mu.RUnlock()
		return token, nil
	}
	tm.mu.RUnlock()

	return tm.refreshToken(ctx)
}

// refreshToken exchanges the offline token for a new access token
func (tm *TokenManager) refreshToken(ctx context.Context) (string, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Double-check after acquiring write lock
	if tm.accessToken != "" && time.Now().Add(TokenExpiryBuffer).Before(tm.expiresAt) {
		return tm.accessToken, nil
	}

	if tm.offlineToken == "" {
		return "", fmt.Errorf("no offline token configured")
	}

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", ClientID)
	data.Set("refresh_token", tm.offlineToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := tm.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	tm.accessToken = tokenResp.AccessToken
	tm.expiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return tm.accessToken, nil
}

// ValidateOfflineToken checks if an offline token is valid
func ValidateOfflineToken(ctx context.Context, offlineToken string) error {
	tm := NewTokenManager(offlineToken)
	_, err := tm.GetAccessToken(ctx)
	return err
}

// IsTokenExpired checks if the current access token is expired
func (tm *TokenManager) IsTokenExpired() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.accessToken == "" || time.Now().Add(TokenExpiryBuffer).After(tm.expiresAt)
}

// Clear removes all stored tokens
func (tm *TokenManager) Clear() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.offlineToken = ""
	tm.accessToken = ""
	tm.expiresAt = time.Time{}
}
