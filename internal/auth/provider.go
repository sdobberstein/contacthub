package auth

import (
	"context"

	"github.com/sdobberstein/contacthub/internal/store"
)

// Provider verifies credentials and returns the matching user.
type Provider interface {
	// Authenticate returns the user if username+password are valid,
	// or ErrInvalidCredentials on failure.
	Authenticate(ctx context.Context, username, password string) (*store.User, error)
}

// ErrInvalidCredentials is returned when credentials are syntactically valid but wrong.
var ErrInvalidCredentials = &authErr{"invalid credentials"}

type authErr struct{ msg string }

func (e *authErr) Error() string { return e.msg }
