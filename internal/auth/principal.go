// Package auth provides authentication types and provider interfaces.
package auth

import "context"

// Method identifies how a principal authenticated.
type Method string

const (
	// MethodSession indicates authentication via a web session cookie.
	MethodSession Method = "session"
	// MethodAppPassword indicates authentication via a per-device app password.
	MethodAppPassword Method = "app_password"
	// MethodProxy indicates authentication via a trusted reverse proxy header.
	MethodProxy Method = "proxy"
)

// Principal represents the authenticated user for a request.
type Principal struct {
	ID          string
	Username    string
	DisplayName string
	IsAdmin     bool
	AuthMethod  Method
}

type contextKey struct{}

// WithPrincipal returns a new context carrying p.
func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, p)
}

// CurrentPrincipal returns the principal stored in ctx, or nil if unauthenticated.
func CurrentPrincipal(ctx context.Context) *Principal {
	p, _ := ctx.Value(contextKey{}).(*Principal) //nolint:errcheck // type assertion ok form; false = unauthenticated
	return p
}
