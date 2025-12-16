package jwt

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// ValidatorConfig holds JWT validation configuration.
type ValidatorConfig struct {
	Issuer    string
	Audience  string
	ClockSkew time.Duration
}

// ValidatedClaims contains extracted claims from a validated JWT.
type ValidatedClaims struct {
	Subject   string
	Scopes    []string
	ExpiresAt time.Time
	IssuedAt  time.Time
}

// Validator validates JWT tokens.
type Validator struct {
	config      ValidatorConfig
	jwksManager *JWKSManager
}

// Error types
type TokenExpiredError struct {
	ExpiredAt time.Time
}

func (e *TokenExpiredError) Error() string {
	return fmt.Sprintf("token expired at %v", e.ExpiredAt)
}

type InvalidIssuerError struct {
	Expected string
	Actual   string
}

func (e *InvalidIssuerError) Error() string {
	return fmt.Sprintf("invalid issuer: expected %s, got %s", e.Expected, e.Actual)
}

type InvalidAudienceError struct {
	Expected string
	Actual   []string
}

func (e *InvalidAudienceError) Error() string {
	return fmt.Sprintf("invalid audience: expected %s, got %v", e.Expected, e.Actual)
}

type InvalidSignatureError struct {
	Reason string
}

func (e *InvalidSignatureError) Error() string {
	return fmt.Sprintf("invalid signature: %s", e.Reason)
}

type InvalidAlgorithmError struct {
	Algorithm string
}

func (e *InvalidAlgorithmError) Error() string {
	return fmt.Sprintf("invalid algorithm: %s (only RS256 is allowed)", e.Algorithm)
}

// Error type checkers
func IsTokenExpiredError(err error) bool {
	var e *TokenExpiredError
	return errors.As(err, &e)
}

func IsInvalidIssuerError(err error) bool {
	var e *InvalidIssuerError
	return errors.As(err, &e)
}

func IsInvalidAudienceError(err error) bool {
	var e *InvalidAudienceError
	return errors.As(err, &e)
}

func IsInvalidSignatureError(err error) bool {
	var e *InvalidSignatureError
	return errors.As(err, &e)
}

func IsInvalidAlgorithmError(err error) bool {
	var e *InvalidAlgorithmError
	return errors.As(err, &e)
}

// NewValidator creates a new JWT validator.
func NewValidator(config ValidatorConfig, jwksManager *JWKSManager) *Validator {
	return &Validator{
		config:      config,
		jwksManager: jwksManager,
	}
}

// Validate validates a JWT token and returns extracted claims.
func (v *Validator) Validate(ctx context.Context, tokenString string) (*ValidatedClaims, error) {
	// Parse token without verification first to get the kid
	unverified, err := jwt.ParseInsecure([]byte(tokenString))
	if err != nil {
		return nil, &InvalidSignatureError{Reason: "failed to parse token"}
	}

	// Get kid from token header
	kid, err := extractKidFromJWT(tokenString)
	if err != nil || kid == "" {
		return nil, &InvalidSignatureError{Reason: "missing kid in token header"}
	}

	// Get public key from JWKS
	key, err := v.jwksManager.GetKey(ctx, kid)
	if err != nil {
		if IsKeyNotFoundError(err) {
			return nil, &InvalidSignatureError{Reason: fmt.Sprintf("key not found: %s", kid)}
		}
		return nil, fmt.Errorf("failed to get key: %w", err)
	}

	// Check algorithm from key
	alg := key.Algorithm()
	if alg != nil {
		algSig, ok := alg.(jwa.SignatureAlgorithm)
		if ok && algSig != jwa.RS256 {
			return nil, &InvalidAlgorithmError{Algorithm: algSig.String()}
		}
	}

	// Verify and parse token with validation options
	now := time.Now()
	clockSkew := v.config.ClockSkew

	token, err := jwt.Parse([]byte(tokenString),
		jwt.WithKey(jwa.RS256, key),
		jwt.WithValidate(true),
		jwt.WithIssuer(v.config.Issuer),
		jwt.WithAudience(v.config.Audience),
		jwt.WithAcceptableSkew(clockSkew),
	)
	if err != nil {
		return nil, v.mapValidationError(err, unverified, now)
	}

	// Extract claims
	claims := &ValidatedClaims{
		Subject:   token.Subject(),
		Scopes:    extractScopes(token),
		ExpiresAt: token.Expiration(),
		IssuedAt:  token.IssuedAt(),
	}

	return claims, nil
}

func (v *Validator) mapValidationError(err error, token jwt.Token, now time.Time) error {
	if errors.Is(err, jwt.ErrTokenExpired()) {
		return &TokenExpiredError{ExpiredAt: token.Expiration()}
	}

	if errors.Is(err, jwt.ErrInvalidIssuer()) {
		return &InvalidIssuerError{
			Expected: v.config.Issuer,
			Actual:   token.Issuer(),
		}
	}

	if errors.Is(err, jwt.ErrInvalidAudience()) {
		return &InvalidAudienceError{
			Expected: v.config.Audience,
			Actual:   token.Audience(),
		}
	}

	if errors.Is(err, jwt.ErrTokenNotYetValid()) {
		return fmt.Errorf("token not yet valid: nbf claim validation failed")
	}

	if jwt.IsValidationError(err) {
		return &InvalidSignatureError{Reason: err.Error()}
	}

	return fmt.Errorf("token validation failed: %w", err)
}

func extractKidFromJWT(tokenString string) (string, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid JWT format")
	}

	// Decode base64url header
	decoded, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", err
	}

	var header struct {
		Kid string `json:"kid"`
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(decoded, &header); err != nil {
		return "", err
	}

	return header.Kid, nil
}

func extractScopes(token jwt.Token) []string {
	scopeClaim, ok := token.Get("scope")
	if !ok {
		return []string{}
	}

	scopeStr, ok := scopeClaim.(string)
	if !ok {
		return []string{}
	}

	if scopeStr == "" {
		return []string{}
	}

	return strings.Split(scopeStr, " ")
}
