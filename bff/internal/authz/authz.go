package authz

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"

	pkgmw "github.com/daisuke8000/example-ec-platform/pkg/connect/middleware"
)

const (
	ScopeAdmin      = "admin"
	ScopeUserRead   = "user:read"
	ScopeUserWrite  = "user:write"
	ScopeUserDelete = "user:delete"
)

var (
	ErrUnauthenticated = connect.NewError(connect.CodeUnauthenticated, errors.New("authentication required"))
	ErrPermissionDenied = connect.NewError(connect.CodePermissionDenied, errors.New("access denied"))
)

type Authorizer struct{}

func NewAuthorizer() *Authorizer {
	return &Authorizer{}
}

// CanAccessUser checks if the current user can access the target user's data.
// Returns nil if access is allowed, error otherwise.
func (a *Authorizer) CanAccessUser(ctx context.Context, targetUserID string) error {
	currentUserID := pkgmw.GetUserID(ctx)
	if currentUserID == "" {
		return ErrUnauthenticated
	}

	// Admin can access any user
	if a.HasScope(ctx, ScopeAdmin) {
		return nil
	}

	// Users can only access their own data (BOLA prevention)
	if currentUserID != targetUserID {
		return ErrPermissionDenied
	}

	return nil
}

// HasScope checks if the current user has the specified scope.
func (a *Authorizer) HasScope(ctx context.Context, scope string) bool {
	scopes := pkgmw.GetScopes(ctx)
	if scopes == "" {
		return false
	}

	for _, s := range strings.Split(scopes, " ") {
		if s == scope {
			return true
		}
	}
	return false
}

// RequireAuthenticated checks if the user is authenticated.
func (a *Authorizer) RequireAuthenticated(ctx context.Context) error {
	if pkgmw.GetUserID(ctx) == "" {
		return ErrUnauthenticated
	}
	return nil
}
