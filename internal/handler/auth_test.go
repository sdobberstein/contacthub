package handler_test

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/sdobberstein/contacthub/internal/auth/local"
	"github.com/sdobberstein/contacthub/internal/config"
	"github.com/sdobberstein/contacthub/internal/handler"
	"github.com/sdobberstein/contacthub/internal/migrations"
	"github.com/sdobberstein/contacthub/internal/store"
	"github.com/sdobberstein/contacthub/internal/store/sqlite"
)

// testTmpl is a minimal template set used in handler tests.
var testTmpl = template.Must(template.New("login.html").Parse(
	`{{define "login.html"}}ERROR:{{.Error}}{{end}}`,
)).Lookup("login.html")

func init() {
	template.Must(testTmpl.New("setup.html").Parse(
		`{{define "setup.html"}}ERROR:{{.Error}}{{end}}`,
	))
}

func newTestDB(t *testing.T) *sqlite.DB {
	t.Helper()
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := migrations.Run(context.Background(), db.DB()); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() }) //nolint:errcheck // best-effort cleanup in test
	return db
}

func defaultAuthCfg() config.AuthConfig {
	return config.AuthConfig{
		Session: config.SessionConfig{MaxAge: 3600},
	}
}

func postForm(t *testing.T, h http.Handler, path string, vals url.Values) *httptest.ResponseRecorder {
	t.Helper()
	r, err := http.NewRequestWithContext(context.Background(), "POST", path, strings.NewReader(vals.Encode()))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// --- LoginHandler ---

func TestLoginHandler_GET(t *testing.T) {
	db := newTestDB(t)
	h := handler.LoginHandler(db, local.New(db), defaultAuthCfg(), testTmpl)

	r, _ := http.NewRequestWithContext(context.Background(), "GET", "/auth/login", http.NoBody)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestLoginHandler_POST_ValidCredentials(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	if _, err := local.CreateUser(ctx, db, "alice", "Alice", "password123", false); err != nil {
		t.Fatalf("create user: %v", err)
	}

	h := handler.LoginHandler(db, local.New(db), defaultAuthCfg(), testTmpl)
	w := postForm(t, h, "/auth/login", url.Values{"username": {"alice"}, "password": {"password123"}})

	if w.Code != http.StatusSeeOther {
		t.Errorf("want 303, got %d", w.Code)
	}

	var sessionCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "chub_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("want chub_session cookie, got none")
	}
	if !sessionCookie.HttpOnly {
		t.Error("session cookie must be HttpOnly")
	}
}

func TestLoginHandler_POST_BadCredentials(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	if _, err := local.CreateUser(ctx, db, "alice", "Alice", "rightpassword", false); err != nil {
		t.Fatalf("create user: %v", err)
	}

	h := handler.LoginHandler(db, local.New(db), defaultAuthCfg(), testTmpl)
	w := postForm(t, h, "/auth/login", url.Values{"username": {"alice"}, "password": {"wrongpassword"}})

	if w.Code != http.StatusOK {
		t.Errorf("want 200 with error, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ERROR:") {
		t.Error("want error message in body")
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == "chub_session" {
			t.Error("must not set session cookie on bad login")
		}
	}
}

func TestLoginHandler_POST_UnknownUser(t *testing.T) {
	db := newTestDB(t)
	h := handler.LoginHandler(db, local.New(db), defaultAuthCfg(), testTmpl)
	w := postForm(t, h, "/auth/login", url.Values{"username": {"nobody"}, "password": {"pass"}})

	if w.Code != http.StatusOK {
		t.Errorf("want 200 with error, got %d", w.Code)
	}
}

// --- LogoutHandler ---

func TestLogoutHandler_ClearsCookieAndRedirects(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	if _, err := local.CreateUser(ctx, db, "alice", "Alice", "pass", false); err != nil {
		t.Fatalf("create user: %v", err)
	}
	u, _ := db.GetUserByUsername(ctx, "alice")
	session := &store.Session{
		ID: "sess1", UserID: u.ID, IPAddress: "127.0.0.1",
		CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
	}
	_ = db.CreateSession(ctx, session)

	h := handler.LogoutHandler(db)
	r, _ := http.NewRequestWithContext(ctx, "POST", "/auth/logout", http.NoBody)
	r.AddCookie(&http.Cookie{Name: "chub_session", Value: "sess1"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Errorf("want 303, got %d", w.Code)
	}

	var cleared bool
	for _, c := range w.Result().Cookies() {
		if c.Name == "chub_session" && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("want chub_session cleared (MaxAge < 0)")
	}

	if _, err := db.GetSession(ctx, "sess1"); err != store.ErrNotFound {
		t.Errorf("session should be deleted, got %v", err)
	}
}

// --- SetupHandler ---

func TestSetupHandler_GET(t *testing.T) {
	db := newTestDB(t)
	h := handler.SetupHandler(db, testTmpl)

	r, _ := http.NewRequestWithContext(context.Background(), "GET", "/setup", http.NoBody)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestSetupHandler_POST_CreatesAdminAndRedirects(t *testing.T) {
	db := newTestDB(t)
	h := handler.SetupHandler(db, testTmpl)

	w := postForm(t, h, "/setup", url.Values{
		"username": {"admin"}, "password": {"securepass"}, "display_name": {"Admin"},
	})

	if w.Code != http.StatusSeeOther {
		t.Errorf("want 303, got %d", w.Code)
	}

	n, _ := db.CountUsers(context.Background())
	if n != 1 {
		t.Errorf("want 1 user, got %d", n)
	}
}

func TestSetupHandler_POST_ShortPassword(t *testing.T) {
	db := newTestDB(t)
	h := handler.SetupHandler(db, testTmpl)

	w := postForm(t, h, "/setup", url.Values{"username": {"admin"}, "password": {"short"}})

	if w.Code != http.StatusOK {
		t.Errorf("want 200 with error, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ERROR:") {
		t.Error("want error in body")
	}
}

func TestSetupHandler_POST_EmptyFields(t *testing.T) {
	db := newTestDB(t)
	h := handler.SetupHandler(db, testTmpl)

	w := postForm(t, h, "/setup", url.Values{"username": {""}, "password": {""}})

	if w.Code != http.StatusOK {
		t.Errorf("want 200 with error, got %d", w.Code)
	}
}

func TestSetupHandler_POST_DefaultDisplayName(t *testing.T) {
	db := newTestDB(t)
	h := handler.SetupHandler(db, testTmpl)

	postForm(t, h, "/setup", url.Values{"username": {"admin"}, "password": {"securepass"}})

	u, err := db.GetUserByUsername(context.Background(), "admin")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if u.DisplayName != "admin" {
		t.Errorf("want display_name=admin, got %q", u.DisplayName)
	}
}
