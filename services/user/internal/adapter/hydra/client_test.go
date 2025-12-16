package hydra

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetLoginRequest(t *testing.T) {
	tests := []struct {
		name       string
		challenge  string
		response   LoginRequest
		statusCode int
		wantErr    bool
	}{
		{
			name:      "returns login request successfully",
			challenge: "test-challenge",
			response: LoginRequest{
				Challenge:      "test-challenge",
				RequestedScope: []string{"openid", "profile"},
				Skip:           false,
				Subject:        "",
				Client: OAuth2Client{
					ClientID:   "test-client",
					ClientName: "Test Client",
				},
			},
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:      "returns login request with skip true",
			challenge: "skip-challenge",
			response: LoginRequest{
				Challenge: "skip-challenge",
				Skip:      true,
				Subject:   "user-123",
				Client: OAuth2Client{
					ClientID: "test-client",
				},
			},
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "returns error for invalid challenge",
			challenge:  "invalid-challenge",
			statusCode: http.StatusNotFound,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("expected GET, got %s", r.Method)
				}
				if r.URL.Query().Get("login_challenge") != tt.challenge {
					t.Errorf("expected challenge %s, got %s", tt.challenge, r.URL.Query().Get("login_challenge"))
				}

				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					json.NewEncoder(w).Encode(tt.response)
				} else {
					json.NewEncoder(w).Encode(HydraError{Error: "not_found"})
				}
			}))
			defer server.Close()

			client := NewClient(server.URL)
			req, err := client.GetLoginRequest(context.Background(), tt.challenge)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if req.Challenge != tt.response.Challenge {
				t.Errorf("challenge = %s, want %s", req.Challenge, tt.response.Challenge)
			}
			if req.Skip != tt.response.Skip {
				t.Errorf("skip = %v, want %v", req.Skip, tt.response.Skip)
			}
			if req.Subject != tt.response.Subject {
				t.Errorf("subject = %s, want %s", req.Subject, tt.response.Subject)
			}
		})
	}
}

func TestAcceptLogin(t *testing.T) {
	tests := []struct {
		name       string
		challenge  string
		accept     AcceptLoginRequest
		response   RedirectResponse
		statusCode int
		wantErr    bool
	}{
		{
			name:      "accepts login successfully",
			challenge: "test-challenge",
			accept: AcceptLoginRequest{
				Subject:     "user-123",
				Remember:    true,
				RememberFor: 604800, // 7 days
			},
			response: RedirectResponse{
				RedirectTo: "https://hydra/callback?code=abc",
			},
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:      "accepts login without remember",
			challenge: "test-challenge",
			accept: AcceptLoginRequest{
				Subject: "user-456",
			},
			response: RedirectResponse{
				RedirectTo: "https://hydra/callback?code=def",
			},
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "returns error for expired challenge",
			challenge:  "expired-challenge",
			accept:     AcceptLoginRequest{Subject: "user-123"},
			statusCode: http.StatusBadRequest,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("expected PUT, got %s", r.Method)
				}

				var accept AcceptLoginRequest
				json.NewDecoder(r.Body).Decode(&accept)

				if accept.Subject != tt.accept.Subject {
					t.Errorf("subject = %s, want %s", accept.Subject, tt.accept.Subject)
				}

				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					json.NewEncoder(w).Encode(tt.response)
				} else {
					json.NewEncoder(w).Encode(HydraError{Error: "invalid_request"})
				}
			}))
			defer server.Close()

			client := NewClient(server.URL)
			resp, err := client.AcceptLogin(context.Background(), tt.challenge, tt.accept)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if resp.RedirectTo != tt.response.RedirectTo {
				t.Errorf("redirect_to = %s, want %s", resp.RedirectTo, tt.response.RedirectTo)
			}
		})
	}
}

func TestRejectLogin(t *testing.T) {
	tests := []struct {
		name       string
		challenge  string
		reject     RejectRequest
		response   RedirectResponse
		statusCode int
		wantErr    bool
	}{
		{
			name:      "rejects login successfully",
			challenge: "test-challenge",
			reject: RejectRequest{
				Error:            "access_denied",
				ErrorDescription: "Invalid credentials",
			},
			response: RedirectResponse{
				RedirectTo: "https://client/callback?error=access_denied",
			},
			statusCode: http.StatusOK,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("expected PUT, got %s", r.Method)
				}

				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			client := NewClient(server.URL)
			resp, err := client.RejectLogin(context.Background(), tt.challenge, tt.reject)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if resp.RedirectTo != tt.response.RedirectTo {
				t.Errorf("redirect_to = %s, want %s", resp.RedirectTo, tt.response.RedirectTo)
			}
		})
	}
}

func TestGetConsentRequest(t *testing.T) {
	tests := []struct {
		name       string
		challenge  string
		response   ConsentRequest
		statusCode int
		wantErr    bool
	}{
		{
			name:      "returns consent request successfully",
			challenge: "consent-challenge",
			response: ConsentRequest{
				Challenge:      "consent-challenge",
				RequestedScope: []string{"openid", "profile", "email"},
				Skip:           false,
				Subject:        "user-123",
				Client: OAuth2Client{
					ClientID:   "test-client",
					ClientName: "Test Application",
				},
			},
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:      "returns consent request with skip true",
			challenge: "skip-consent-challenge",
			response: ConsentRequest{
				Challenge:      "skip-consent-challenge",
				RequestedScope: []string{"openid"},
				Skip:           true,
				Subject:        "user-123",
			},
			statusCode: http.StatusOK,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("expected GET, got %s", r.Method)
				}
				if r.URL.Query().Get("consent_challenge") != tt.challenge {
					t.Errorf("expected challenge %s", tt.challenge)
				}

				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			client := NewClient(server.URL)
			req, err := client.GetConsentRequest(context.Background(), tt.challenge)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if req.Challenge != tt.response.Challenge {
				t.Errorf("challenge = %s, want %s", req.Challenge, tt.response.Challenge)
			}
			if req.Skip != tt.response.Skip {
				t.Errorf("skip = %v, want %v", req.Skip, tt.response.Skip)
			}
		})
	}
}

func TestAcceptConsent(t *testing.T) {
	tests := []struct {
		name       string
		challenge  string
		accept     AcceptConsentRequest
		response   RedirectResponse
		statusCode int
		wantErr    bool
	}{
		{
			name:      "accepts consent with all scopes",
			challenge: "consent-challenge",
			accept: AcceptConsentRequest{
				GrantScope:  []string{"openid", "profile", "email"},
				Remember:    true,
				RememberFor: 2592000, // 30 days
				Session: &ConsentSession{
					IDToken: map[string]interface{}{
						"email": "user@example.com",
						"name":  "Test User",
					},
				},
			},
			response: RedirectResponse{
				RedirectTo: "https://client/callback?code=xyz",
			},
			statusCode: http.StatusOK,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("expected PUT, got %s", r.Method)
				}

				var accept AcceptConsentRequest
				json.NewDecoder(r.Body).Decode(&accept)

				if len(accept.GrantScope) != len(tt.accept.GrantScope) {
					t.Errorf("grant_scope length = %d, want %d", len(accept.GrantScope), len(tt.accept.GrantScope))
				}

				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			client := NewClient(server.URL)
			resp, err := client.AcceptConsent(context.Background(), tt.challenge, tt.accept)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if resp.RedirectTo != tt.response.RedirectTo {
				t.Errorf("redirect_to = %s, want %s", resp.RedirectTo, tt.response.RedirectTo)
			}
		})
	}
}

func TestGetLogoutRequest(t *testing.T) {
	tests := []struct {
		name       string
		challenge  string
		response   LogoutRequest
		statusCode int
		wantErr    bool
	}{
		{
			name:      "returns logout request successfully",
			challenge: "logout-challenge",
			response: LogoutRequest{
				Challenge:   "logout-challenge",
				Subject:     "user-123",
				SessionID:   "session-abc",
				RPInitiated: true,
			},
			statusCode: http.StatusOK,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("expected GET, got %s", r.Method)
				}

				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			client := NewClient(server.URL)
			req, err := client.GetLogoutRequest(context.Background(), tt.challenge)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if req.Challenge != tt.response.Challenge {
				t.Errorf("challenge = %s, want %s", req.Challenge, tt.response.Challenge)
			}
			if req.Subject != tt.response.Subject {
				t.Errorf("subject = %s, want %s", req.Subject, tt.response.Subject)
			}
		})
	}
}

func TestAcceptLogout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(RedirectResponse{
			RedirectTo: "https://example.com/logged-out",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	resp, err := client.AcceptLogout(context.Background(), "logout-challenge")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if resp.RedirectTo != "https://example.com/logged-out" {
		t.Errorf("redirect_to = %s, want %s", resp.RedirectTo, "https://example.com/logged-out")
	}
}

func TestRejectLogout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	err := client.RejectLogout(context.Background(), "logout-challenge")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
