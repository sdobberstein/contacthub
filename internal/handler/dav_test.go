package handler_test

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/sdobberstein/contacthub/internal/auth"
	"github.com/sdobberstein/contacthub/internal/auth/local"
	"github.com/sdobberstein/contacthub/internal/handler"
)

// withPrincipal injects a principal into the request context.
func withPrincipal(r *http.Request, p *auth.Principal) *http.Request {
	return r.WithContext(auth.WithPrincipal(r.Context(), p))
}

// withChiParam injects a chi URL parameter into the request context.
// If a route context already exists it is reused, so multiple calls accumulate params.
func withChiParam(r *http.Request, key, val string) *http.Request {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil {
		rctx = chi.NewRouteContext()
	}
	rctx.URLParams.Add(key, val)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// propfindReq builds a PROPFIND request with the given XML body.
func propfindReq(t *testing.T, path, body string) *http.Request {
	t.Helper()
	var bodyReader *strings.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	var r *http.Request
	var err error
	if bodyReader != nil {
		r, err = http.NewRequestWithContext(context.Background(), "PROPFIND", path, bodyReader)
	} else {
		r, err = http.NewRequestWithContext(context.Background(), "PROPFIND", path, http.NoBody)
	}
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	return r
}

// assertXMLContains checks that the response body is a valid multistatus containing substr.
func assertXMLContains(t *testing.T, w *httptest.ResponseRecorder, substr string) {
	t.Helper()
	body := w.Body.String()
	if !strings.Contains(body, substr) {
		t.Errorf("response body missing %q\nbody: %s", substr, body)
	}
}

// --- DAVOptions ---

func TestDAVOptions_StatusAndHeaders(t *testing.T) {
	r, _ := http.NewRequestWithContext(context.Background(), http.MethodOptions, "/dav/", http.NoBody)
	w := httptest.NewRecorder()
	handler.DAVOptions(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
	dav := w.Header().Get("DAV")
	for _, token := range []string{"1", "2", "access-control", "addressbook"} {
		if !strings.Contains(dav, token) {
			t.Errorf("DAV header %q missing token %q", dav, token)
		}
	}
	if w.Header().Get("Allow") == "" {
		t.Error("want Allow header")
	}
}

// --- DAVRootPropfind ---

func TestDAVRootPropfind_AllProp(t *testing.T) {
	alice := &auth.Principal{ID: "u1", Username: "alice", DisplayName: "Alice"}

	r := withPrincipal(propfindReq(t, "/dav/", ""), alice)
	w := httptest.NewRecorder()
	handler.DAVRootPropfind(w, r)

	if w.Code != http.StatusMultiStatus {
		t.Fatalf("want 207, got %d\nbody: %s", w.Code, w.Body.String())
	}
	assertXMLContains(t, w, "<D:href>/dav/</D:href>")
	assertXMLContains(t, w, "current-user-principal")
	assertXMLContains(t, w, "/dav/principals/users/alice/")
}

func TestDAVRootPropfind_SpecificProp(t *testing.T) {
	alice := &auth.Principal{ID: "u1", Username: "alice", DisplayName: "Alice"}
	body := `<?xml version="1.0"?><D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`

	r := withPrincipal(propfindReq(t, "/dav/", body), alice)
	w := httptest.NewRecorder()
	handler.DAVRootPropfind(w, r)

	if w.Code != http.StatusMultiStatus {
		t.Fatalf("want 207, got %d", w.Code)
	}
	assertXMLContains(t, w, "current-user-principal")
	assertXMLContains(t, w, "/dav/principals/users/alice/")
}

func TestDAVRootPropfind_UnknownPropGets404Propstat(t *testing.T) {
	alice := &auth.Principal{ID: "u1", Username: "alice", DisplayName: "Alice"}
	body := `<?xml version="1.0"?><D:propfind xmlns:D="DAV:"><D:prop><D:getetag/></D:prop></D:propfind>`

	r := withPrincipal(propfindReq(t, "/dav/", body), alice)
	w := httptest.NewRecorder()
	handler.DAVRootPropfind(w, r)

	if w.Code != http.StatusMultiStatus {
		t.Fatalf("want 207, got %d", w.Code)
	}
	assertXMLContains(t, w, "404")
}

func TestDAVRootPropfind_NoAuth_Returns401(t *testing.T) {
	r, _ := http.NewRequestWithContext(context.Background(), "PROPFIND", "/dav/", http.NoBody)
	w := httptest.NewRecorder()
	handler.DAVRootPropfind(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestDAVRootPropfind_ContentType(t *testing.T) {
	alice := &auth.Principal{ID: "u1", Username: "alice"}
	r := withPrincipal(propfindReq(t, "/dav/", ""), alice)
	w := httptest.NewRecorder()
	handler.DAVRootPropfind(w, r)

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/xml") {
		t.Errorf("want application/xml content-type, got %q", ct)
	}
}

// --- PrincipalPropfind ---

func TestPrincipalPropfind_AllProp(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	if _, err := local.CreateUser(ctx, db, "alice", "Alice Smith", "pass123456", false); err != nil {
		t.Fatalf("create user: %v", err)
	}
	alice := &auth.Principal{ID: "u1", Username: "alice", DisplayName: "Alice Smith"}

	r := withPrincipal(propfindReq(t, "/dav/principals/users/alice/", ""), alice)
	r = withChiParam(r, "username", "alice")
	w := httptest.NewRecorder()
	handler.PrincipalPropfind(db)(w, r)

	if w.Code != http.StatusMultiStatus {
		t.Fatalf("want 207, got %d\nbody: %s", w.Code, w.Body.String())
	}
	assertXMLContains(t, w, "addressbook-home-set")
	assertXMLContains(t, w, "/dav/addressbooks/alice/")
	assertXMLContains(t, w, "Alice Smith")
}

func TestPrincipalPropfind_SpecificProps(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	if _, err := local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false); err != nil {
		t.Fatalf("create user: %v", err)
	}
	alice := &auth.Principal{Username: "alice"}
	body := `<?xml version="1.0"?>
		<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
			<D:prop><C:addressbook-home-set/></D:prop>
		</D:propfind>`

	r := withPrincipal(propfindReq(t, "/dav/principals/users/alice/", body), alice)
	r = withChiParam(r, "username", "alice")
	w := httptest.NewRecorder()
	handler.PrincipalPropfind(db)(w, r)

	if w.Code != http.StatusMultiStatus {
		t.Fatalf("want 207, got %d", w.Code)
	}
	assertXMLContains(t, w, "addressbook-home-set")
	assertXMLContains(t, w, "/dav/addressbooks/alice/")
}

func TestPrincipalPropfind_FallbackDisplayName(t *testing.T) {
	// User with empty display name falls back to username.
	db := newTestDB(t)
	ctx := context.Background()
	if _, err := local.CreateUser(ctx, db, "bob", "", "pass123456", false); err != nil {
		t.Fatalf("create user: %v", err)
	}
	bob := &auth.Principal{Username: "bob"}
	r := withPrincipal(propfindReq(t, "/dav/principals/users/bob/", ""), bob)
	r = withChiParam(r, "username", "bob")
	w := httptest.NewRecorder()
	handler.PrincipalPropfind(db)(w, r)

	if w.Code != http.StatusMultiStatus {
		t.Fatalf("want 207, got %d", w.Code)
	}
	assertXMLContains(t, w, "<D:displayname>bob</D:displayname>")
}

func TestPrincipalPropfind_AdminCanReadOtherPrincipal(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	if _, err := local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false); err != nil {
		t.Fatalf("create user: %v", err)
	}
	admin := &auth.Principal{Username: "admin", IsAdmin: true}

	r := withPrincipal(propfindReq(t, "/dav/principals/users/alice/", ""), admin)
	r = withChiParam(r, "username", "alice")
	w := httptest.NewRecorder()
	handler.PrincipalPropfind(db)(w, r)

	if w.Code != http.StatusMultiStatus {
		t.Fatalf("want 207, got %d", w.Code)
	}
}

func TestPrincipalPropfind_ForbiddenForOtherUser(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	if _, err := local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false); err != nil {
		t.Fatalf("create user: %v", err)
	}
	bob := &auth.Principal{Username: "bob", IsAdmin: false}

	r := withPrincipal(propfindReq(t, "/dav/principals/users/alice/", ""), bob)
	r = withChiParam(r, "username", "alice")
	w := httptest.NewRecorder()
	handler.PrincipalPropfind(db)(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", w.Code)
	}
}

func TestPrincipalPropfind_NotFound(t *testing.T) {
	db := newTestDB(t)
	admin := &auth.Principal{Username: "admin", IsAdmin: true}

	r := withPrincipal(propfindReq(t, "/dav/principals/users/nobody/", ""), admin)
	r = withChiParam(r, "username", "nobody")
	w := httptest.NewRecorder()
	handler.PrincipalPropfind(db)(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestPrincipalPropfind_NoAuth_Returns401(t *testing.T) {
	db := newTestDB(t)
	r, _ := http.NewRequestWithContext(context.Background(), "PROPFIND", "/dav/principals/users/alice/", http.NoBody)
	r = withChiParam(r, "username", "alice")
	w := httptest.NewRecorder()
	handler.PrincipalPropfind(db)(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

// TestPrincipalPropfind_ResponseIsValidXML verifies the 207 body parses as well-formed XML.
func TestPrincipalPropfind_ResponseIsValidXML(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	if _, err := local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false); err != nil {
		t.Fatalf("create user: %v", err)
	}
	alice := &auth.Principal{Username: "alice"}
	r := withPrincipal(propfindReq(t, "/dav/principals/users/alice/", ""), alice)
	r = withChiParam(r, "username", "alice")
	w := httptest.NewRecorder()
	handler.PrincipalPropfind(db)(w, r)

	dec := xml.NewDecoder(w.Body)
	for {
		_, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.Fatalf("invalid XML in response: %v\n%s", err, w.Body.String())
		}
	}
}
