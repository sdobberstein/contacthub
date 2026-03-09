package handler_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sdobberstein/contacthub/internal/auth"
	"github.com/sdobberstein/contacthub/internal/auth/local"
	"github.com/sdobberstein/contacthub/internal/handler"
	"github.com/sdobberstein/contacthub/internal/store"
)

const testVCard = "BEGIN:VCARD\r\n" +
	"VERSION:4.0\r\n" +
	"UID:urn:uuid:aaaaaaaa-0000-0000-0000-000000000001\r\n" +
	"FN:Alice Test\r\n" +
	"END:VCARD\r\n"

// setupAliceWithBook creates alice + personal address book, returning the address book.
func setupAliceWithBook(t *testing.T, db interface {
	CreateUser(context.Context, *store.User) error
	CreateAddressBook(context.Context, *store.AddressBook) error
	GetUserByUsername(context.Context, string) (*store.User, error)
}) *store.AddressBook {
	t.Helper()
	ctx := context.Background()
	user, err := local.CreateUser(ctx, db.(interface {
		CreateUser(context.Context, *store.User) error
		CreateSession(context.Context, *store.Session) error
		CreateAppPassword(context.Context, *store.AppPassword) error
		GetUserByID(context.Context, string) (*store.User, error)
		GetUserByUsername(context.Context, string) (*store.User, error)
		UpdateUser(context.Context, *store.User) error
		DeleteUser(context.Context, string) error
		ListUsers(context.Context) ([]*store.User, error)
		CountUsers(context.Context) (int, error)
		GetSession(context.Context, string) (*store.Session, error)
		DeleteSession(context.Context, string) error
		DeleteUserSessions(context.Context, string) error
		PurgeExpiredSessions(context.Context) error
		GetAppPasswordByTokenHash(context.Context, string) (*store.AppPassword, error)
		ListAppPasswords(context.Context, string) ([]*store.AppPassword, error)
		UpdateAppPasswordLastUsed(context.Context, string, time.Time) error
		DeleteAppPassword(context.Context, string) error
	}), "alice", "Alice", "pass123456", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	ab := makeBook("ab1", user.ID, "personal")
	if err := db.CreateAddressBook(ctx, ab); err != nil {
		t.Fatalf("CreateAddressBook: %v", err)
	}
	return ab
}

func putContactRequest(t *testing.T, path, body string) *http.Request {
	t.Helper()
	r, err := http.NewRequestWithContext(context.Background(), http.MethodPut, path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new PUT request: %v", err)
	}
	r.Header.Set("Content-Type", "text/vcard; charset=utf-8")
	return r
}

// --- ContactPut ---

func TestContactPut_Creates201(t *testing.T) {
	db := newTestDB(t)
	setupAliceWithBook(t, db)
	alice := &auth.Principal{Username: "alice"}

	r := putContactRequest(t, "/dav/addressbooks/alice/personal/alice.vcf", testVCard)
	r = withPrincipal(r, alice)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "personal")
	r = withChiParam(r, "filename", "alice.vcf")
	w := httptest.NewRecorder()
	handler.ContactPut(db)(w, r)

	if w.Code != http.StatusCreated {
		t.Errorf("want 201, got %d\n%s", w.Code, w.Body.String())
	}
	if w.Header().Get("ETag") == "" {
		t.Error("want ETag header on PUT")
	}
}

func TestContactPut_Updates204(t *testing.T) {
	db := newTestDB(t)
	setupAliceWithBook(t, db)
	alice := &auth.Principal{Username: "alice"}

	// First PUT → 201.
	r := putContactRequest(t, "/dav/addressbooks/alice/personal/alice.vcf", testVCard)
	r = withPrincipal(r, alice)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "personal")
	r = withChiParam(r, "filename", "alice.vcf")
	w := httptest.NewRecorder()
	handler.ContactPut(db)(w, r)

	// Second PUT → 204.
	r2 := putContactRequest(t, "/dav/addressbooks/alice/personal/alice.vcf", testVCard)
	r2 = withPrincipal(r2, alice)
	r2 = withChiParam(r2, "username", "alice")
	r2 = withChiParam(r2, "book", "personal")
	r2 = withChiParam(r2, "filename", "alice.vcf")
	w2 := httptest.NewRecorder()
	handler.ContactPut(db)(w2, r2)

	if w2.Code != http.StatusNoContent {
		t.Errorf("want 204 on update, got %d", w2.Code)
	}
}

func TestContactPut_IfMatchPrecondition(t *testing.T) {
	db := newTestDB(t)
	setupAliceWithBook(t, db)
	alice := &auth.Principal{Username: "alice"}

	// Create contact.
	r := putContactRequest(t, "/dav/addressbooks/alice/personal/alice.vcf", testVCard)
	r = withPrincipal(r, alice)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "personal")
	r = withChiParam(r, "filename", "alice.vcf")
	w := httptest.NewRecorder()
	handler.ContactPut(db)(w, r)
	etag := w.Header().Get("ETag")

	// Update with stale ETag → 412.
	r2 := putContactRequest(t, "/dav/addressbooks/alice/personal/alice.vcf", testVCard)
	r2 = withPrincipal(r2, alice)
	r2 = withChiParam(r2, "username", "alice")
	r2 = withChiParam(r2, "book", "personal")
	r2 = withChiParam(r2, "filename", "alice.vcf")
	r2.Header.Set("If-Match", `"stale-etag"`)
	w2 := httptest.NewRecorder()
	handler.ContactPut(db)(w2, r2)

	if w2.Code != http.StatusPreconditionFailed {
		t.Errorf("want 412 for stale If-Match, got %d", w2.Code)
	}

	// Update with correct ETag → 204.
	r3 := putContactRequest(t, "/dav/addressbooks/alice/personal/alice.vcf", testVCard)
	r3 = withPrincipal(r3, alice)
	r3 = withChiParam(r3, "username", "alice")
	r3 = withChiParam(r3, "book", "personal")
	r3 = withChiParam(r3, "filename", "alice.vcf")
	r3.Header.Set("If-Match", etag)
	w3 := httptest.NewRecorder()
	handler.ContactPut(db)(w3, r3)

	if w3.Code != http.StatusNoContent {
		t.Errorf("want 204 with correct If-Match, got %d", w3.Code)
	}
}

func TestContactPut_NoAuth_Returns401(t *testing.T) {
	db := newTestDB(t)
	r := putContactRequest(t, "/dav/addressbooks/alice/personal/alice.vcf", testVCard)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "personal")
	r = withChiParam(r, "filename", "alice.vcf")
	w := httptest.NewRecorder()
	handler.ContactPut(db)(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

// --- ContactGet ---

func TestContactGet_Returns200WithBody(t *testing.T) {
	db := newTestDB(t)
	setupAliceWithBook(t, db)
	alice := &auth.Principal{Username: "alice"}

	// PUT first.
	r := putContactRequest(t, "/dav/addressbooks/alice/personal/alice.vcf", testVCard)
	r = withPrincipal(r, alice)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "personal")
	r = withChiParam(r, "filename", "alice.vcf")
	handler.ContactPut(db)(httptest.NewRecorder(), r)

	// GET.
	rg, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/dav/addressbooks/alice/personal/alice.vcf", http.NoBody)
	rg = withPrincipal(rg, alice)
	rg = withChiParam(rg, "username", "alice")
	rg = withChiParam(rg, "book", "personal")
	rg = withChiParam(rg, "filename", "alice.vcf")
	wg := httptest.NewRecorder()
	handler.ContactGet(db)(wg, rg)

	if wg.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", wg.Code)
	}
	if wg.Header().Get("ETag") == "" {
		t.Error("want ETag header")
	}
	ct := wg.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/vcard") {
		t.Errorf("want text/vcard content-type, got %q", ct)
	}
	if !strings.Contains(wg.Body.String(), "BEGIN:VCARD") {
		t.Error("want vCard body")
	}
}

func TestContactGet_NotFound(t *testing.T) {
	db := newTestDB(t)
	setupAliceWithBook(t, db)
	alice := &auth.Principal{Username: "alice"}

	r, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/dav/addressbooks/alice/personal/nope.vcf", http.NoBody)
	r = withPrincipal(r, alice)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "personal")
	r = withChiParam(r, "filename", "nope.vcf")
	w := httptest.NewRecorder()
	handler.ContactGet(db)(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

// --- ContactDelete ---

func TestContactDelete_Returns204(t *testing.T) {
	db := newTestDB(t)
	setupAliceWithBook(t, db)
	alice := &auth.Principal{Username: "alice"}

	// Create.
	rp := putContactRequest(t, "/dav/addressbooks/alice/personal/alice.vcf", testVCard)
	rp = withPrincipal(rp, alice)
	rp = withChiParam(rp, "username", "alice")
	rp = withChiParam(rp, "book", "personal")
	rp = withChiParam(rp, "filename", "alice.vcf")
	wp := httptest.NewRecorder()
	handler.ContactPut(db)(wp, rp)
	etag := wp.Header().Get("ETag")

	// Delete with matching ETag.
	rd, _ := http.NewRequestWithContext(context.Background(), http.MethodDelete, "/dav/addressbooks/alice/personal/alice.vcf", http.NoBody)
	rd = withPrincipal(rd, alice)
	rd = withChiParam(rd, "username", "alice")
	rd = withChiParam(rd, "book", "personal")
	rd = withChiParam(rd, "filename", "alice.vcf")
	rd.Header.Set("If-Match", etag)
	wd := httptest.NewRecorder()
	handler.ContactDelete(db)(wd, rd)

	if wd.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d", wd.Code)
	}

	// Subsequent GET → 404.
	rg, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/dav/addressbooks/alice/personal/alice.vcf", http.NoBody)
	rg = withPrincipal(rg, alice)
	rg = withChiParam(rg, "username", "alice")
	rg = withChiParam(rg, "book", "personal")
	rg = withChiParam(rg, "filename", "alice.vcf")
	wg := httptest.NewRecorder()
	handler.ContactGet(db)(wg, rg)
	if wg.Code != http.StatusNotFound {
		t.Errorf("want 404 after delete, got %d", wg.Code)
	}
}

func TestContactDelete_ETagMismatch_Returns412(t *testing.T) {
	db := newTestDB(t)
	setupAliceWithBook(t, db)
	alice := &auth.Principal{Username: "alice"}

	rp := putContactRequest(t, "/dav/addressbooks/alice/personal/alice.vcf", testVCard)
	rp = withPrincipal(rp, alice)
	rp = withChiParam(rp, "username", "alice")
	rp = withChiParam(rp, "book", "personal")
	rp = withChiParam(rp, "filename", "alice.vcf")
	handler.ContactPut(db)(httptest.NewRecorder(), rp)

	rd, _ := http.NewRequestWithContext(context.Background(), http.MethodDelete, "/dav/addressbooks/alice/personal/alice.vcf", http.NoBody)
	rd = withPrincipal(rd, alice)
	rd = withChiParam(rd, "username", "alice")
	rd = withChiParam(rd, "book", "personal")
	rd = withChiParam(rd, "filename", "alice.vcf")
	rd.Header.Set("If-Match", `"stale-etag"`)
	w := httptest.NewRecorder()
	handler.ContactDelete(db)(w, rd)

	if w.Code != http.StatusPreconditionFailed {
		t.Errorf("want 412, got %d", w.Code)
	}
}

// --- ContactPropfind ---

func TestContactPropfind_Depth0_ReturnsProps(t *testing.T) {
	db := newTestDB(t)
	setupAliceWithBook(t, db)
	alice := &auth.Principal{Username: "alice"}

	rp := putContactRequest(t, "/dav/addressbooks/alice/personal/alice.vcf", testVCard)
	rp = withPrincipal(rp, alice)
	rp = withChiParam(rp, "username", "alice")
	rp = withChiParam(rp, "book", "personal")
	rp = withChiParam(rp, "filename", "alice.vcf")
	handler.ContactPut(db)(httptest.NewRecorder(), rp)

	body := `<?xml version="1.0"?><D:propfind xmlns:D="DAV:"><D:allprop/></D:propfind>`
	r := propfindReq(t, "/dav/addressbooks/alice/personal/alice.vcf", body)
	r.Header.Set("Depth", "0")
	r = withPrincipal(r, alice)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "personal")
	r = withChiParam(r, "filename", "alice.vcf")
	w := httptest.NewRecorder()
	handler.ContactPropfind(db)(w, r)

	if w.Code != http.StatusMultiStatus {
		t.Fatalf("want 207, got %d\n%s", w.Code, w.Body.String())
	}
	for _, prop := range []string{"getetag", "getcontenttype", "getcontentlength", "getlastmodified"} {
		if !strings.Contains(w.Body.String(), prop) {
			t.Errorf("want property %q in response", prop)
		}
	}
}

func TestContactPropfind_InfinityForbidden(t *testing.T) {
	db := newTestDB(t)
	setupAliceWithBook(t, db)
	alice := &auth.Principal{Username: "alice"}

	rp := putContactRequest(t, "/dav/addressbooks/alice/personal/alice.vcf", testVCard)
	rp = withPrincipal(rp, alice)
	rp = withChiParam(rp, "username", "alice")
	rp = withChiParam(rp, "book", "personal")
	rp = withChiParam(rp, "filename", "alice.vcf")
	handler.ContactPut(db)(httptest.NewRecorder(), rp)

	r, _ := http.NewRequestWithContext(context.Background(), "PROPFIND", "/dav/addressbooks/alice/personal/alice.vcf", http.NoBody)
	r.Header.Set("Depth", "infinity")
	r = withPrincipal(r, alice)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "personal")
	r = withChiParam(r, "filename", "alice.vcf")
	w := httptest.NewRecorder()
	handler.ContactPropfind(db)(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", w.Code)
	}
}

// --- ContactCopy ---

func TestContactCopy_Creates201_BothExist(t *testing.T) {
	db := newTestDB(t)
	setupAliceWithBook(t, db)
	alice := &auth.Principal{Username: "alice"}

	rp := putContactRequest(t, "/dav/addressbooks/alice/personal/src.vcf", testVCard)
	rp = withPrincipal(rp, alice)
	rp = withChiParam(rp, "username", "alice")
	rp = withChiParam(rp, "book", "personal")
	rp = withChiParam(rp, "filename", "src.vcf")
	handler.ContactPut(db)(httptest.NewRecorder(), rp)

	rc, _ := http.NewRequestWithContext(context.Background(), "COPY", "/dav/addressbooks/alice/personal/src.vcf", http.NoBody)
	rc.Header.Set("Destination", "http://localhost/dav/addressbooks/alice/personal/dst.vcf")
	rc.Header.Set("Overwrite", "F")
	rc = withPrincipal(rc, alice)
	rc = withChiParam(rc, "username", "alice")
	rc = withChiParam(rc, "book", "personal")
	rc = withChiParam(rc, "filename", "src.vcf")
	wc := httptest.NewRecorder()
	handler.ContactCopy(db)(wc, rc)

	if wc.Code != http.StatusCreated {
		t.Errorf("want 201, got %d\n%s", wc.Code, wc.Body.String())
	}

	// Both src and dst must exist.
	for _, f := range []string{"src.vcf", "dst.vcf"} {
		rg, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, fmt.Sprintf("/dav/addressbooks/alice/personal/%s", f), http.NoBody)
		rg = withPrincipal(rg, alice)
		rg = withChiParam(rg, "username", "alice")
		rg = withChiParam(rg, "book", "personal")
		rg = withChiParam(rg, "filename", f)
		wg := httptest.NewRecorder()
		handler.ContactGet(db)(wg, rg)
		if wg.Code != http.StatusOK {
			t.Errorf("GET %s after COPY: want 200, got %d", f, wg.Code)
		}
	}
}

// --- ContactMove ---

func TestContactMove_Relocates_SrcGone_DstExists(t *testing.T) {
	db := newTestDB(t)
	setupAliceWithBook(t, db)
	alice := &auth.Principal{Username: "alice"}

	rp := putContactRequest(t, "/dav/addressbooks/alice/personal/src.vcf", testVCard)
	rp = withPrincipal(rp, alice)
	rp = withChiParam(rp, "username", "alice")
	rp = withChiParam(rp, "book", "personal")
	rp = withChiParam(rp, "filename", "src.vcf")
	handler.ContactPut(db)(httptest.NewRecorder(), rp)

	rm, _ := http.NewRequestWithContext(context.Background(), "MOVE", "/dav/addressbooks/alice/personal/src.vcf", http.NoBody)
	rm.Header.Set("Destination", "http://localhost/dav/addressbooks/alice/personal/dst.vcf")
	rm.Header.Set("Overwrite", "F")
	rm = withPrincipal(rm, alice)
	rm = withChiParam(rm, "username", "alice")
	rm = withChiParam(rm, "book", "personal")
	rm = withChiParam(rm, "filename", "src.vcf")
	wm := httptest.NewRecorder()
	handler.ContactMove(db)(wm, rm)

	if wm.Code != http.StatusCreated {
		t.Errorf("want 201, got %d\n%s", wm.Code, wm.Body.String())
	}

	// src → 404.
	rsrc, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/dav/addressbooks/alice/personal/src.vcf", http.NoBody)
	rsrc = withPrincipal(rsrc, alice)
	rsrc = withChiParam(rsrc, "username", "alice")
	rsrc = withChiParam(rsrc, "book", "personal")
	rsrc = withChiParam(rsrc, "filename", "src.vcf")
	wsrc := httptest.NewRecorder()
	handler.ContactGet(db)(wsrc, rsrc)
	if wsrc.Code != http.StatusNotFound {
		t.Errorf("src after MOVE: want 404, got %d", wsrc.Code)
	}

	// dst → 200.
	rdst, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/dav/addressbooks/alice/personal/dst.vcf", http.NoBody)
	rdst = withPrincipal(rdst, alice)
	rdst = withChiParam(rdst, "username", "alice")
	rdst = withChiParam(rdst, "book", "personal")
	rdst = withChiParam(rdst, "filename", "dst.vcf")
	wdst := httptest.NewRecorder()
	handler.ContactGet(db)(wdst, rdst)
	if wdst.Code != http.StatusOK {
		t.Errorf("dst after MOVE: want 200, got %d", wdst.Code)
	}
}
