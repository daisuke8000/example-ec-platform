package http

import (
	"embed"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/daisuke8000/example-ec-platform/services/user/internal/adapter/hydra"
	"github.com/daisuke8000/example-ec-platform/services/user/internal/domain"
	"github.com/daisuke8000/example-ec-platform/services/user/internal/usecase"
)

//go:embed templates/*.html
var templateFS embed.FS

type Handler struct {
	hydra              *hydra.Client
	userUC             usecase.UserUseCase
	rateLimit          RateLimiter
	templates          *template.Template
	logger             *slog.Logger
	loginRememberFor   int
	consentRememberFor int
}

type RateLimiter interface {
	Allow(key string) bool
	Reset(key string) error
}

type NoOpRateLimiter struct{}

func (n *NoOpRateLimiter) Allow(key string) bool  { return true }
func (n *NoOpRateLimiter) Reset(key string) error { return nil }

type HandlerConfig struct {
	LoginRememberFor   int
	ConsentRememberFor int
}

func NewHandler(hydraClient *hydra.Client, userUC usecase.UserUseCase, rateLimit RateLimiter, logger *slog.Logger, cfg HandlerConfig) (*Handler, error) {
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	if rateLimit == nil {
		rateLimit = &NoOpRateLimiter{}
	}

	return &Handler{
		hydra:              hydraClient,
		userUC:             userUC,
		rateLimit:          rateLimit,
		templates:          tmpl,
		logger:             logger,
		loginRememberFor:   cfg.LoginRememberFor,
		consentRememberFor: cfg.ConsentRememberFor,
	}, nil
}

// Router returns an http.Handler with all OAuth2 routes configured.
func (h *Handler) Router() http.Handler {
	mux := http.NewServeMux()

	// Login flow
	mux.HandleFunc("GET /oauth2/login", h.handleLoginGet)
	mux.HandleFunc("POST /oauth2/login", h.handleLoginPost)

	// Consent flow
	mux.HandleFunc("GET /oauth2/consent", h.handleConsentGet)
	mux.HandleFunc("POST /oauth2/consent", h.handleConsentPost)

	// Logout flow
	mux.HandleFunc("GET /oauth2/logout", h.handleLogoutGet)
	mux.HandleFunc("POST /oauth2/logout", h.handleLogoutPost)

	// Error page
	mux.HandleFunc("GET /oauth2/error", h.handleError)

	// Health check
	mux.HandleFunc("GET /health", h.handleHealth)

	return mux
}

// LoginData holds data for the login template.
type LoginData struct {
	Challenge  string
	ClientName string
	Email      string
	Error      string
}

// handleLoginGet renders the login form.
func (h *Handler) handleLoginGet(w http.ResponseWriter, r *http.Request) {
	challenge := r.URL.Query().Get("login_challenge")
	if challenge == "" {
		h.redirectToError(w, r, "invalid_request", "Missing login challenge")
		return
	}

	loginReq, err := h.hydra.GetLoginRequest(r.Context(), challenge)
	if err != nil {
		h.logger.Error("failed to get login request", slog.String("error", err.Error()))
		h.redirectToError(w, r, "server_error", "Failed to process login request")
		return
	}

	// If skip is true, accept login with existing subject
	if loginReq.Skip {
		resp, err := h.hydra.AcceptLogin(r.Context(), challenge, hydra.AcceptLoginRequest{
			Subject: loginReq.Subject,
		})
		if err != nil {
			h.logger.Error("failed to accept login (skip)", slog.String("error", err.Error()))
			h.redirectToError(w, r, "server_error", "Failed to process login")
			return
		}
		http.Redirect(w, r, resp.RedirectTo, http.StatusFound)
		return
	}

	// Render login form
	clientName := loginReq.Client.ClientName
	if clientName == "" {
		clientName = loginReq.Client.ClientID
	}

	data := LoginData{
		Challenge:  challenge,
		ClientName: clientName,
	}

	if err := h.templates.ExecuteTemplate(w, "login.html", data); err != nil {
		h.logger.Error("failed to render login template", slog.String("error", err.Error()))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleLoginPost processes login form submission.
func (h *Handler) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.redirectToError(w, r, "invalid_request", "Failed to parse form")
		return
	}

	challenge := r.FormValue("login_challenge")
	email := r.FormValue("email")
	password := r.FormValue("password")
	remember := r.FormValue("remember") == "true"

	if challenge == "" {
		h.redirectToError(w, r, "invalid_request", "Missing login challenge")
		return
	}

	// Check rate limiting
	if !h.rateLimit.Allow(email) {
		data := LoginData{
			Challenge:  challenge,
			ClientName: "Application",
			Email:      email,
			Error:      "Too many login attempts. Please try again later.",
		}
		w.WriteHeader(http.StatusTooManyRequests)
		h.templates.ExecuteTemplate(w, "login.html", data)
		return
	}

	// Verify credentials
	user, err := h.userUC.VerifyPassword(r.Context(), email, password)
	if err != nil {
		h.logger.Debug("login failed",
			slog.String("error", err.Error()),
		)

		// Re-render login form with error
		loginReq, _ := h.hydra.GetLoginRequest(r.Context(), challenge)
		clientName := "Application"
		if loginReq != nil && loginReq.Client.ClientName != "" {
			clientName = loginReq.Client.ClientName
		}

		data := LoginData{
			Challenge:  challenge,
			ClientName: clientName,
			Email:      email,
			Error:      "Invalid email or password",
		}

		if err == domain.ErrInvalidCredentials {
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			data.Error = "An error occurred. Please try again."
		}

		h.templates.ExecuteTemplate(w, "login.html", data)
		return
	}

	// Reset rate limit on successful login
	h.rateLimit.Reset(email)

	// Accept login
	acceptReq := hydra.AcceptLoginRequest{
		Subject: user.ID.String(),
	}

	if remember {
		acceptReq.Remember = true
		acceptReq.RememberFor = h.loginRememberFor
	}

	resp, err := h.hydra.AcceptLogin(r.Context(), challenge, acceptReq)
	if err != nil {
		h.logger.Error("failed to accept login", slog.String("error", err.Error()))
		h.redirectToError(w, r, "server_error", "Failed to complete login")
		return
	}

	h.logger.Info("user logged in",
		slog.String("user_id", user.ID.String()),
		slog.Bool("remember", remember),
	)

	http.Redirect(w, r, resp.RedirectTo, http.StatusFound)
}

// ScopeInfo holds information about an OAuth2 scope for display.
type ScopeInfo struct {
	ID          string
	Name        string
	Description string
}

// ConsentData holds data for the consent template.
type ConsentData struct {
	Challenge  string
	ClientName string
	Scopes     []ScopeInfo
}

var scopeDescriptions = map[string]ScopeInfo{
	"openid": {
		ID:          "openid",
		Name:        "OpenID",
		Description: "Access your basic identity information (user ID)",
	},
	"profile": {
		ID:          "profile",
		Name:        "Profile",
		Description: "Access your profile information (name)",
	},
	"email": {
		ID:          "email",
		Name:        "Email",
		Description: "Access your email address",
	},
	"offline_access": {
		ID:          "offline_access",
		Name:        "Offline Access",
		Description: "Stay signed in and access your data when you're not using the app",
	},
}

// handleConsentGet renders the consent form.
func (h *Handler) handleConsentGet(w http.ResponseWriter, r *http.Request) {
	challenge := r.URL.Query().Get("consent_challenge")
	if challenge == "" {
		h.redirectToError(w, r, "invalid_request", "Missing consent challenge")
		return
	}

	consentReq, err := h.hydra.GetConsentRequest(r.Context(), challenge)
	if err != nil {
		h.logger.Error("failed to get consent request", slog.String("error", err.Error()))
		h.redirectToError(w, r, "server_error", "Failed to process consent request")
		return
	}

	// If skip is true, accept consent with previously granted scopes
	if consentReq.Skip {
		resp, err := h.hydra.AcceptConsent(r.Context(), challenge, hydra.AcceptConsentRequest{
			GrantScope: consentReq.RequestedScope,
		})
		if err != nil {
			h.logger.Error("failed to accept consent (skip)", slog.String("error", err.Error()))
			h.redirectToError(w, r, "server_error", "Failed to process consent")
			return
		}
		http.Redirect(w, r, resp.RedirectTo, http.StatusFound)
		return
	}

	// Build scope information for display
	var scopes []ScopeInfo
	for _, scopeID := range consentReq.RequestedScope {
		if info, ok := scopeDescriptions[scopeID]; ok {
			scopes = append(scopes, info)
		} else {
			scopes = append(scopes, ScopeInfo{
				ID:          scopeID,
				Name:        scopeID,
				Description: "Access to " + scopeID,
			})
		}
	}

	clientName := consentReq.Client.ClientName
	if clientName == "" {
		clientName = consentReq.Client.ClientID
	}

	data := ConsentData{
		Challenge:  challenge,
		ClientName: clientName,
		Scopes:     scopes,
	}

	if err := h.templates.ExecuteTemplate(w, "consent.html", data); err != nil {
		h.logger.Error("failed to render consent template", slog.String("error", err.Error()))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleConsentPost processes consent form submission.
func (h *Handler) handleConsentPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.redirectToError(w, r, "invalid_request", "Failed to parse form")
		return
	}

	challenge := r.FormValue("consent_challenge")
	action := r.FormValue("action")

	if challenge == "" {
		h.redirectToError(w, r, "invalid_request", "Missing consent challenge")
		return
	}

	// Handle denial
	if action == "deny" {
		resp, err := h.hydra.RejectConsent(r.Context(), challenge, hydra.RejectRequest{
			Error:            "access_denied",
			ErrorDescription: "The user denied the request",
		})
		if err != nil {
			h.logger.Error("failed to reject consent", slog.String("error", err.Error()))
			h.redirectToError(w, r, "server_error", "Failed to process consent")
			return
		}
		http.Redirect(w, r, resp.RedirectTo, http.StatusFound)
		return
	}

	// Get the consent request to retrieve user info
	consentReq, err := h.hydra.GetConsentRequest(r.Context(), challenge)
	if err != nil {
		h.logger.Error("failed to get consent request", slog.String("error", err.Error()))
		h.redirectToError(w, r, "server_error", "Failed to process consent")
		return
	}

	// Get granted scopes from form
	grantedScopes := r.Form["grant_scope"]
	if len(grantedScopes) == 0 {
		grantedScopes = consentReq.RequestedScope
	}

	remember := r.FormValue("remember") == "true"

	// Build session with user claims
	session := &hydra.ConsentSession{
		IDToken: map[string]interface{}{
			"sub": consentReq.Subject,
		},
	}

	// Add email claim if email scope is granted
	for _, scope := range grantedScopes {
		if scope == "email" {
			// In a real implementation, fetch user email from database
			session.IDToken["email"] = consentReq.Subject + "@example.com"
			session.IDToken["email_verified"] = true
		}
		if scope == "profile" {
			session.IDToken["name"] = "User"
		}
	}

	acceptReq := hydra.AcceptConsentRequest{
		GrantScope: grantedScopes,
		Session:    session,
	}

	if remember {
		acceptReq.Remember = true
		acceptReq.RememberFor = h.consentRememberFor
	}

	resp, err := h.hydra.AcceptConsent(r.Context(), challenge, acceptReq)
	if err != nil {
		h.logger.Error("failed to accept consent", slog.String("error", err.Error()))
		h.redirectToError(w, r, "server_error", "Failed to complete consent")
		return
	}

	h.logger.Info("consent granted",
		slog.String("subject", consentReq.Subject),
		slog.Any("scopes", grantedScopes),
		slog.Bool("remember", remember),
	)

	http.Redirect(w, r, resp.RedirectTo, http.StatusFound)
}

// LogoutData holds data for the logout template.
type LogoutData struct {
	Challenge string
}

// handleLogoutGet renders the logout confirmation page.
func (h *Handler) handleLogoutGet(w http.ResponseWriter, r *http.Request) {
	challenge := r.URL.Query().Get("logout_challenge")
	if challenge == "" {
		h.redirectToError(w, r, "invalid_request", "Missing logout challenge")
		return
	}

	_, err := h.hydra.GetLogoutRequest(r.Context(), challenge)
	if err != nil {
		h.logger.Error("failed to get logout request", slog.String("error", err.Error()))
		h.redirectToError(w, r, "server_error", "Failed to process logout request")
		return
	}

	data := LogoutData{
		Challenge: challenge,
	}

	if err := h.templates.ExecuteTemplate(w, "logout.html", data); err != nil {
		h.logger.Error("failed to render logout template", slog.String("error", err.Error()))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleLogoutPost processes logout form submission.
func (h *Handler) handleLogoutPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.redirectToError(w, r, "invalid_request", "Failed to parse form")
		return
	}

	challenge := r.FormValue("logout_challenge")
	action := r.FormValue("action")

	if challenge == "" {
		h.redirectToError(w, r, "invalid_request", "Missing logout challenge")
		return
	}

	// Handle cancellation
	if action == "cancel" {
		if err := h.hydra.RejectLogout(r.Context(), challenge); err != nil {
			h.logger.Error("failed to reject logout", slog.String("error", err.Error()))
			h.redirectToError(w, r, "server_error", "Failed to cancel logout")
			return
		}
		// Redirect to a default page since logout was cancelled
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Accept logout
	resp, err := h.hydra.AcceptLogout(r.Context(), challenge)
	if err != nil {
		h.logger.Error("failed to accept logout", slog.String("error", err.Error()))
		h.redirectToError(w, r, "server_error", "Failed to complete logout")
		return
	}

	h.logger.Info("user logged out")

	http.Redirect(w, r, resp.RedirectTo, http.StatusFound)
}

// ErrorData holds data for the error template.
type ErrorData struct {
	ErrorCode        string
	ErrorDescription string
	ErrorHint        string
}

// handleError renders the error page.
func (h *Handler) handleError(w http.ResponseWriter, r *http.Request) {
	data := ErrorData{
		ErrorCode:        r.URL.Query().Get("error"),
		ErrorDescription: r.URL.Query().Get("error_description"),
		ErrorHint:        r.URL.Query().Get("error_hint"),
	}

	w.WriteHeader(http.StatusBadRequest)
	if err := h.templates.ExecuteTemplate(w, "error.html", data); err != nil {
		h.logger.Error("failed to render error template", slog.String("error", err.Error()))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleHealth returns OK if the service is healthy.
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// redirectToError redirects to the error page with the given error details.
func (h *Handler) redirectToError(w http.ResponseWriter, r *http.Request, errorCode, description string) {
	redirectURL := "/oauth2/error?error=" + url.QueryEscape(errorCode) + "&error_description=" + url.QueryEscape(description)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}
