// Package hydra provides an adapter for communicating with Ory Hydra Admin API.
package hydra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client handles communication with the Hydra Admin API.
type Client struct {
	adminURL   string
	httpClient *http.Client
}

// NewClient creates a new Hydra Admin API client.
func NewClient(adminURL string) *Client {
	return &Client{
		adminURL: adminURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// LoginRequest represents the login request details from Hydra.
type LoginRequest struct {
	Challenge       string         `json:"challenge"`
	RequestedScope  []string       `json:"requested_scope"`
	Skip            bool           `json:"skip"`
	Subject         string         `json:"subject"`
	Client          OAuth2Client   `json:"client"`
	RequestURL      string         `json:"request_url"`
	SessionID       string         `json:"session_id,omitempty"`
	OIDCContext     *OIDCContext   `json:"oidc_context,omitempty"`
}

// OAuth2Client represents information about the OAuth2 client making the request.
type OAuth2Client struct {
	ClientID   string                 `json:"client_id"`
	ClientName string                 `json:"client_name,omitempty"`
	LogoURI    string                 `json:"logo_uri,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// OIDCContext contains OpenID Connect specific context.
type OIDCContext struct {
	ACRValues         []string          `json:"acr_values,omitempty"`
	Display           string            `json:"display,omitempty"`
	IDTokenHintClaims map[string]string `json:"id_token_hint_claims,omitempty"`
	LoginHint         string            `json:"login_hint,omitempty"`
	UILocales         []string          `json:"ui_locales,omitempty"`
}

// AcceptLoginRequest contains the data to accept a login request.
type AcceptLoginRequest struct {
	Subject     string `json:"subject"`
	Remember    bool   `json:"remember,omitempty"`
	RememberFor int    `json:"remember_for,omitempty"` // Seconds
	ACR         string `json:"acr,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
}

// RejectRequest contains the data to reject a login or consent request.
type RejectRequest struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
	ErrorHint        string `json:"error_hint,omitempty"`
	StatusCode       int    `json:"status_code,omitempty"`
}

// RedirectResponse represents the redirect URL returned by Hydra.
type RedirectResponse struct {
	RedirectTo string `json:"redirect_to"`
}

// ConsentRequest represents the consent request details from Hydra.
type ConsentRequest struct {
	Challenge                    string       `json:"challenge"`
	RequestedScope               []string     `json:"requested_scope"`
	RequestedAccessTokenAudience []string     `json:"requested_access_token_audience"`
	Skip                         bool         `json:"skip"`
	Subject                      string       `json:"subject"`
	Client                       OAuth2Client `json:"client"`
	RequestURL                   string       `json:"request_url"`
	LoginChallenge               string       `json:"login_challenge,omitempty"`
	LoginSessionID               string       `json:"login_session_id,omitempty"`
	ACR                          string       `json:"acr,omitempty"`
	Context                      map[string]interface{} `json:"context,omitempty"`
}

// AcceptConsentRequest contains the data to accept a consent request.
type AcceptConsentRequest struct {
	GrantScope               []string       `json:"grant_scope"`
	GrantAccessTokenAudience []string       `json:"grant_access_token_audience,omitempty"`
	Session                  *ConsentSession `json:"session,omitempty"`
	Remember                 bool           `json:"remember,omitempty"`
	RememberFor              int            `json:"remember_for,omitempty"` // Seconds
}

// ConsentSession contains session data for the consent.
type ConsentSession struct {
	AccessToken map[string]interface{} `json:"access_token,omitempty"`
	IDToken     map[string]interface{} `json:"id_token,omitempty"`
}

// LogoutRequest represents the logout request details from Hydra.
type LogoutRequest struct {
	Challenge       string `json:"challenge"`
	Subject         string `json:"subject"`
	SessionID       string `json:"sid,omitempty"`
	RequestURL      string `json:"request_url,omitempty"`
	RPInitiated     bool   `json:"rp_initiated"`
}

// GetLoginRequest fetches login request details from Hydra.
func (c *Client) GetLoginRequest(ctx context.Context, challenge string) (*LoginRequest, error) {
	endpoint := fmt.Sprintf("%s/admin/oauth2/auth/requests/login?login_challenge=%s",
		c.adminURL, url.QueryEscape(challenge))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	var loginReq LoginRequest
	if err := json.NewDecoder(resp.Body).Decode(&loginReq); err != nil {
		return nil, fmt.Errorf("failed to decode login request: %w", err)
	}

	return &loginReq, nil
}

// AcceptLogin accepts a login request.
func (c *Client) AcceptLogin(ctx context.Context, challenge string, accept AcceptLoginRequest) (*RedirectResponse, error) {
	endpoint := fmt.Sprintf("%s/admin/oauth2/auth/requests/login/accept?login_challenge=%s",
		c.adminURL, url.QueryEscape(challenge))

	body, err := json.Marshal(accept)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal accept request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to accept login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	var redirectResp RedirectResponse
	if err := json.NewDecoder(resp.Body).Decode(&redirectResp); err != nil {
		return nil, fmt.Errorf("failed to decode redirect response: %w", err)
	}

	return &redirectResp, nil
}

// RejectLogin rejects a login request.
func (c *Client) RejectLogin(ctx context.Context, challenge string, reject RejectRequest) (*RedirectResponse, error) {
	endpoint := fmt.Sprintf("%s/admin/oauth2/auth/requests/login/reject?login_challenge=%s",
		c.adminURL, url.QueryEscape(challenge))

	body, err := json.Marshal(reject)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal reject request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reject login: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	var redirectResp RedirectResponse
	if err := json.NewDecoder(resp.Body).Decode(&redirectResp); err != nil {
		return nil, fmt.Errorf("failed to decode redirect response: %w", err)
	}

	return &redirectResp, nil
}

// GetConsentRequest fetches consent request details from Hydra.
func (c *Client) GetConsentRequest(ctx context.Context, challenge string) (*ConsentRequest, error) {
	endpoint := fmt.Sprintf("%s/admin/oauth2/auth/requests/consent?consent_challenge=%s",
		c.adminURL, url.QueryEscape(challenge))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch consent request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	var consentReq ConsentRequest
	if err := json.NewDecoder(resp.Body).Decode(&consentReq); err != nil {
		return nil, fmt.Errorf("failed to decode consent request: %w", err)
	}

	return &consentReq, nil
}

// AcceptConsent accepts a consent request.
func (c *Client) AcceptConsent(ctx context.Context, challenge string, accept AcceptConsentRequest) (*RedirectResponse, error) {
	endpoint := fmt.Sprintf("%s/admin/oauth2/auth/requests/consent/accept?consent_challenge=%s",
		c.adminURL, url.QueryEscape(challenge))

	body, err := json.Marshal(accept)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal accept request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to accept consent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	var redirectResp RedirectResponse
	if err := json.NewDecoder(resp.Body).Decode(&redirectResp); err != nil {
		return nil, fmt.Errorf("failed to decode redirect response: %w", err)
	}

	return &redirectResp, nil
}

// RejectConsent rejects a consent request.
func (c *Client) RejectConsent(ctx context.Context, challenge string, reject RejectRequest) (*RedirectResponse, error) {
	endpoint := fmt.Sprintf("%s/admin/oauth2/auth/requests/consent/reject?consent_challenge=%s",
		c.adminURL, url.QueryEscape(challenge))

	body, err := json.Marshal(reject)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal reject request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reject consent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	var redirectResp RedirectResponse
	if err := json.NewDecoder(resp.Body).Decode(&redirectResp); err != nil {
		return nil, fmt.Errorf("failed to decode redirect response: %w", err)
	}

	return &redirectResp, nil
}

// GetLogoutRequest fetches logout request details from Hydra.
func (c *Client) GetLogoutRequest(ctx context.Context, challenge string) (*LogoutRequest, error) {
	endpoint := fmt.Sprintf("%s/admin/oauth2/auth/requests/logout?logout_challenge=%s",
		c.adminURL, url.QueryEscape(challenge))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch logout request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	var logoutReq LogoutRequest
	if err := json.NewDecoder(resp.Body).Decode(&logoutReq); err != nil {
		return nil, fmt.Errorf("failed to decode logout request: %w", err)
	}

	return &logoutReq, nil
}

// AcceptLogout accepts a logout request.
func (c *Client) AcceptLogout(ctx context.Context, challenge string) (*RedirectResponse, error) {
	endpoint := fmt.Sprintf("%s/admin/oauth2/auth/requests/logout/accept?logout_challenge=%s",
		c.adminURL, url.QueryEscape(challenge))

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to accept logout: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	var redirectResp RedirectResponse
	if err := json.NewDecoder(resp.Body).Decode(&redirectResp); err != nil {
		return nil, fmt.Errorf("failed to decode redirect response: %w", err)
	}

	return &redirectResp, nil
}

// RejectLogout rejects a logout request.
func (c *Client) RejectLogout(ctx context.Context, challenge string) error {
	endpoint := fmt.Sprintf("%s/admin/oauth2/auth/requests/logout/reject?logout_challenge=%s",
		c.adminURL, url.QueryEscape(challenge))

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reject logout: %w", err)
	}
	defer resp.Body.Close()

	// 204 No Content is the expected response for logout reject
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return c.handleErrorResponse(resp)
	}

	return nil
}

// HydraError represents an error returned by Hydra API.
type HydraError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
	StatusCode       int    `json:"status_code,omitempty"`
}

func (e *HydraError) Err() error {
	return fmt.Errorf("hydra error: %s - %s (status: %d)", e.Error, e.ErrorDescription, e.StatusCode)
}

func (c *Client) handleErrorResponse(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("hydra API error (status %d): failed to read response body", resp.StatusCode)
	}

	var hydraErr HydraError
	if err := json.Unmarshal(body, &hydraErr); err != nil {
		return fmt.Errorf("hydra API error (status %d): %s", resp.StatusCode, string(body))
	}

	hydraErr.StatusCode = resp.StatusCode
	return hydraErr.Err()
}
