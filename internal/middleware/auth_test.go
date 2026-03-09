package middleware_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sdobberstein/contacthub/internal/auth"
	"github.com/sdobberstein/contacthub/internal/middleware"
	"github.com/sdobberstein/contacthub/internal/migrations"
	"github.com/sdobberstein/contacthub/internal/store"
	"github.com/sdobberstein/contacthub/internal/store/sqlite"
)

func newTestDB(t *testing.T) *sqlite.DB {
	t.Helper()
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := migrations.Run(context.Background(), db.DB()); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() }) //nolint:errcheck // best-effort cleanup in test
	return db
}

func makeUser(t *testing.T, db *sqlite.DB, id, username string) *store.User {
	t.Helper()
	now := time.Now()
	u := &store.User{ //nolint:gosec // PasswordHash is a test fixture, not a real credential
		ID: id, Username: username, DisplayName: username,
		PasswordHash: "$argon2id$v=19$m=65536,t=1,p=4$fake$fake",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.CreateUser(context.Background(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func makeSession(t *testing.T, db *sqlite.DB, id, userID string, expiresAt time.Time) {
	t.Helper()
	s := &store.Session{
		ID: id, UserID: userID, IPAddress: "127.0.0.1",
		CreatedAt: time.Now(), ExpiresAt: expiresAt,
	}
	if err := db.CreateSession(context.Background(), s); err != nil {
		t.Fatalf("create session: %v", err)
	}
}

func okHandler(w http.ResponseWriter, r *http.Request) {
	p := auth.CurrentPrincipal(r.Context())
	if p != nil {
		fmt.Fprintf(w, "user:%s", p.Username)
	} else {
		w.WriteHeader(http.StatusOK)
	}
}

func newRequest(t *testing.T, method, path string) *http.Request {
	t.Helper()
	r, err := http.NewRequestWithContext(context.Background(), method, path, http.NoBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	return r
}

func TestRequireAuth_ValidSessionCookie(t *testing.T) {
	db := newTestDB(t)
	makeUser(t, db, "u1", "alice")
	makeSession(t, db, "sess1", "u1", time.Now().Add(time.Hour))

	handler := middleware.RequireAuth(db)(http.HandlerFunc(okHandler))

	r := newRequest(t, "GET", "/")
	r.AddCookie(&http.Cookie{Name: "chub_session", Value: "sess1"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
	if body := w.Body.String(); body != "user:alice" {
		t.Errorf("want user:alice in body, got %q", body)
	}
}

func TestRequireAuth_ExpiredSession(t *testing.T) {
	db := newTestDB(t)
	makeUser(t, db, "u1", "alice")
	makeSession(t, db, "old", "u1", time.Now().Add(-time.Hour))

	handler := middleware.RequireAuth(db)(http.HandlerFunc(okHandler))

	r := newRequest(t, "GET", "/")
	r.AddCookie(&http.Cookie{Name: "chub_session", Value: "old"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	// Expired session on a browser path → redirect to login.
	if w.Code != http.StatusFound {
		t.Errorf("want 302, got %d", w.Code)
	}
}

func TestRequireAuth_NoCookie_BrowserPath(t *testing.T) {
	db := newTestDB(t)
	handler := middleware.RequireAuth(db)(http.HandlerFunc(okHandler))

	r := newRequest(t, "GET", "/contacts")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusFound {
		t.Errorf("want 302 redirect, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/auth/login" {
		t.Errorf("want redirect to /auth/login, got %q", loc)
	}
}

func TestRequireAuth_NoCookie_DavPath(t *testing.T) {
	db := newTestDB(t)
	handler := middleware.RequireAuth(db)(http.HandlerFunc(okHandler))

	r := newRequest(t, "PROPFIND", "/dav/alice/default/")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
	if v := w.Header().Get("WWW-Authenticate"); v == "" {
		t.Error("want WWW-Authenticate header")
	}
}

func TestRequireAuth_ValidBasicAuth_AppPassword(t *testing.T) {
	db := newTestDB(t)
	makeUser(t, db, "u1", "alice")

	rawToken := "chub_AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(rawToken)))
	ap := &store.AppPassword{
		ID: "ap1", UserID: "u1", Name: "phone", TokenHash: hash, CreatedAt: time.Now(),
	}
	_ = db.CreateAppPassword(context.Background(), ap)

	handler := middleware.RequireAuth(db)(http.HandlerFunc(okHandler))

	r := newRequest(t, "GET", "/dav/")
	r.SetBasicAuth("alice", rawToken)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
	if body := w.Body.String(); body != "user:alice" {
		t.Errorf("want user:alice, got %q", body)
	}
}

func TestRequireAuth_InvalidBasicAuth(t *testing.T) {
	db := newTestDB(t)
	handler := middleware.RequireAuth(db)(http.HandlerFunc(okHandler))

	r := newRequest(t, "GET", "/dav/")
	r.SetBasicAuth("alice", "chub_wrongtoken")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestRequireAuth_BasicAuth_NonChubPassword(t *testing.T) {
	db := newTestDB(t)
	handler := middleware.RequireAuth(db)(http.HandlerFunc(okHandler))

	r := newRequest(t, "GET", "/dav/")
	r.SetBasicAuth("alice", "plaintextpassword")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestSetupGuard_AllowsWhenNoUsers(t *testing.T) {
	db := newTestDB(t)
	handler := middleware.SetupGuard(db)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := newRequest(t, "GET", "/setup")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestSetupGuard_BlocksWhenUsersExist(t *testing.T) {
	db := newTestDB(t)
	makeUser(t, db, "u1", "alice")

	handler := middleware.SetupGuard(db)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := newRequest(t, "GET", "/setup")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}
