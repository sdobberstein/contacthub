package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sdobberstein/contacthub/internal/auth"
	"github.com/sdobberstein/contacthub/internal/auth/local"
	"github.com/sdobberstein/contacthub/internal/handler"
)

const proppatchSetBody = `<?xml version="1.0" encoding="utf-8"?>
<D:propertyupdate xmlns:D="DAV:" xmlns:X="http://example.com/ns/">
  <D:set>
    <D:prop>
      <X:color>blue</X:color>
    </D:prop>
  </D:set>
</D:propertyupdate>`

const proppatchRemoveBody = `<?xml version="1.0" encoding="utf-8"?>
<D:propertyupdate xmlns:D="DAV:" xmlns:X="http://example.com/ns/">
  <D:remove>
    <D:prop>
      <X:color/>
    </D:prop>
  </D:remove>
</D:propertyupdate>`

const proppatchLivePropBody = `<?xml version="1.0" encoding="utf-8"?>
<D:propertyupdate xmlns:D="DAV:">
  <D:set>
    <D:prop>
      <D:displayname>Forbidden</D:displayname>
    </D:prop>
  </D:set>
</D:propertyupdate>`

func TestPropPatchHandler_SetDeadProperty_Returns207(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false) //nolint:errcheck // test setup
	alice := &auth.Principal{Username: "alice"}

	resource := "/dav/addressbooks/alice/personal/alice.vcf"
	r, _ := http.NewRequestWithContext(ctx, "PROPPATCH", resource, strings.NewReader(proppatchSetBody))
	r = withPrincipal(r, alice)
	w := httptest.NewRecorder()
	handler.PropPatchHandler(db)(w, r)

	if w.Code != http.StatusMultiStatus {
		t.Fatalf("want 207, got %d\n%s", w.Code, w.Body.String())
	}
	assertXMLContains(t, w, "HTTP/1.1 200 OK")
}

func TestPropPatchHandler_PropertyPersists(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false) //nolint:errcheck // test setup
	alice := &auth.Principal{Username: "alice"}

	resource := "/dav/addressbooks/alice/personal/alice.vcf"

	// Set the property.
	r, _ := http.NewRequestWithContext(ctx, "PROPPATCH", resource, strings.NewReader(proppatchSetBody))
	r = withPrincipal(r, alice)
	handler.PropPatchHandler(db)(httptest.NewRecorder(), r)

	// Verify it was stored.
	prop, err := db.GetProperty(ctx, resource, "http://example.com/ns/", "color")
	if err != nil {
		t.Fatalf("GetProperty: %v", err)
	}
	if prop.Value != "blue" {
		t.Errorf("want value=blue, got %q", prop.Value)
	}
}

func TestPropPatchHandler_RemoveDeadProperty_Returns207(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false) //nolint:errcheck // test setup
	alice := &auth.Principal{Username: "alice"}

	resource := "/dav/addressbooks/alice/personal/alice.vcf"

	// Set first.
	r, _ := http.NewRequestWithContext(ctx, "PROPPATCH", resource, strings.NewReader(proppatchSetBody))
	r = withPrincipal(r, alice)
	handler.PropPatchHandler(db)(httptest.NewRecorder(), r)

	// Now remove.
	r2, _ := http.NewRequestWithContext(ctx, "PROPPATCH", resource, strings.NewReader(proppatchRemoveBody))
	r2 = withPrincipal(r2, alice)
	w := httptest.NewRecorder()
	handler.PropPatchHandler(db)(w, r2)

	if w.Code != http.StatusMultiStatus {
		t.Fatalf("want 207, got %d\n%s", w.Code, w.Body.String())
	}
}

func TestPropPatchHandler_RemoveNonExistentProperty_Returns207(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false) //nolint:errcheck // test setup
	alice := &auth.Principal{Username: "alice"}

	r, _ := http.NewRequestWithContext(ctx, "PROPPATCH", "/r", strings.NewReader(proppatchRemoveBody))
	r = withPrincipal(r, alice)
	w := httptest.NewRecorder()
	handler.PropPatchHandler(db)(w, r)

	if w.Code != http.StatusMultiStatus {
		t.Errorf("want 207, got %d", w.Code)
	}
}

func TestPropPatchHandler_LivePropertyForbidden(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false) //nolint:errcheck // test setup
	alice := &auth.Principal{Username: "alice"}

	r, _ := http.NewRequestWithContext(ctx, "PROPPATCH", "/dav/addressbooks/alice/", strings.NewReader(proppatchLivePropBody))
	r = withPrincipal(r, alice)
	w := httptest.NewRecorder()
	handler.PropPatchHandler(db)(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", w.Code)
	}
}

func TestPropPatchHandler_NoAuth_Returns401(t *testing.T) {
	db := newTestDB(t)
	r, _ := http.NewRequestWithContext(context.Background(), "PROPPATCH", "/r", strings.NewReader(proppatchSetBody))
	w := httptest.NewRecorder()
	handler.PropPatchHandler(db)(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestPropPatchHandler_EmptyBody_Returns400(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false) //nolint:errcheck // test setup
	alice := &auth.Principal{Username: "alice"}

	r, _ := http.NewRequestWithContext(ctx, "PROPPATCH", "/r", http.NoBody)
	r = withPrincipal(r, alice)
	w := httptest.NewRecorder()
	handler.PropPatchHandler(db)(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}
