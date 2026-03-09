package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sdobberstein/contacthub/internal/auth"
	"github.com/sdobberstein/contacthub/internal/store"
)

const sessionCookieName = "chub_session"

// RequireAuth is middleware that enforces authentication via session cookie or
// Basic Auth (app passwords). On success it stores the Principal in the context.
// On failure it returns 401 for /dav/ paths or 302 to /auth/login otherwise.
func RequireAuth(st store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try Basic Auth first (used by CardDAV clients with app passwords).
			if username, password, ok := r.BasicAuth(); ok {
				p := authenticateBasic(r.Context(), st, username, password)
				if p != nil {
					next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), p)))
					return
				}
				// Basic Auth presented but invalid → 401 with WWW-Authenticate.
				w.Header().Set("WWW-Authenticate", `Basic realm="contacthub"`)
				writeJSON401(w)
				return
			}

			// Try session cookie.
			if p := authenticateSession(r, st); p != nil {
				next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), p)))
				return
			}

			// No valid auth — redirect browsers, 401 for DAV clients.
			if isDavPath(r.URL.Path) {
				w.Header().Set("WWW-Authenticate", `Basic realm="contacthub"`)
				writeJSON401(w)
				return
			}
			http.Redirect(w, r, "/auth/login", http.StatusFound)
		})
	}
}

func authenticateSession(r *http.Request, st store.Store) *auth.Principal {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil
	}

	session, err := st.GetSession(r.Context(), cookie.Value)
	if err != nil || time.Now().After(session.ExpiresAt) {
		return nil
	}

	user, err := st.GetUserByID(r.Context(), session.UserID)
	if err != nil {
		return nil
	}

	return &auth.Principal{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		IsAdmin:     user.IsAdmin,
		AuthMethod:  auth.MethodSession,
	}
}

func authenticateBasic(ctx context.Context, st store.Store, _, password string) *auth.Principal {
	if !strings.HasPrefix(password, "chub_") {
		return nil
	}

	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(password)))
	ap, err := st.GetAppPasswordByTokenHash(ctx, hash)
	if err != nil {
		return nil
	}

	user, err := st.GetUserByID(ctx, ap.UserID)
	if err != nil {
		return nil
	}

	// Update last-used timestamp asynchronously — don't block the request.
	go func() {
		_ = st.UpdateAppPasswordLastUsed(context.WithoutCancel(ctx), ap.ID, time.Now()) //nolint:errcheck // best-effort timestamp update
	}()

	return &auth.Principal{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		IsAdmin:     user.IsAdmin,
		AuthMethod:  auth.MethodAppPassword,
	}
}

// SetupGuard is middleware that allows access only when no users exist yet.
// Returns 404 once users are present.
func SetupGuard(st store.UserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n, err := st.CountUsers(r.Context())
			if err != nil || n > 0 {
				http.NotFound(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isDavPath(path string) bool {
	return strings.HasPrefix(path, "/dav/") || strings.HasPrefix(path, "/.well-known/")
}

func writeJSON401(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"}) //nolint:errcheck // write to ResponseWriter, error unrecoverable
}

// SessionCookieName is exported for use in handlers.
const SessionCookieName = sessionCookieName
