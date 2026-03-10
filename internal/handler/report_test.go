package handler_test

import (
	"context"
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

const (
	aliceVCard = "BEGIN:VCARD\r\n" +
		"VERSION:4.0\r\n" +
		"UID:urn:uuid:aaaaaaaa-0000-0000-0000-000000000001\r\n" +
		"FN:Alice Test\r\n" +
		"END:VCARD\r\n"

	bobVCard = "BEGIN:VCARD\r\n" +
		"VERSION:4.0\r\n" +
		"UID:urn:uuid:bbbbbbbb-0000-0000-0000-000000000001\r\n" +
		"FN:Bob Test\r\n" +
		"END:VCARD\r\n"
)

func setupReportFixtures(t *testing.T, db interface {
	CreateUser(context.Context, *store.User) error
	CreateAddressBook(context.Context, *store.AddressBook) error
	CreateContact(context.Context, *store.Contact) error
	GetUserByUsername(context.Context, string) (*store.User, error)
	CreateSession(context.Context, *store.Session) error
	CreateAppPassword(context.Context, *store.AppPassword) error
	GetUserByID(context.Context, string) (*store.User, error)
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
}) (userID, abID string) {
	t.Helper()
	ctx := context.Background()
	user, err := local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	ab := makeBook("ab1", user.ID, "personal")
	if err := db.CreateAddressBook(ctx, ab); err != nil {
		t.Fatalf("create address book: %v", err)
	}
	now := time.Now().UTC()
	_ = db.CreateContact(ctx, &store.Contact{
		ID: "c1", UID: "uid-alice", AddressBookID: "ab1",
		Filename: "alice.vcf", ETag: "etag-alice",
		VCard: aliceVCard, FN: "Alice Test",
		CreatedAt: now, UpdatedAt: now,
	})
	_ = db.CreateContact(ctx, &store.Contact{
		ID: "c2", UID: "uid-bob", AddressBookID: "ab1",
		Filename: "bob.vcf", ETag: "etag-bob",
		VCard: bobVCard, FN: "Bob Test",
		CreatedAt: now, UpdatedAt: now,
	})
	return user.ID, "ab1"
}

func reportReq(t *testing.T, body string) *http.Request {
	t.Helper()
	r, err := http.NewRequestWithContext(context.Background(), "REPORT",
		"/dav/addressbooks/alice/personal/", strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	return r
}

// --- addressbook-query ---

func TestAddressBookReport_Query_NoFilter_Returns207WithAllContacts(t *testing.T) {
	db := newTestDB(t)
	setupReportFixtures(t, db)
	alice := &auth.Principal{Username: "alice"}

	body := `<?xml version="1.0" encoding="utf-8"?>
		<C:addressbook-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
			<D:prop><D:getetag/><C:address-data/></D:prop>
		</C:addressbook-query>`
	r := withPrincipal(reportReq(t, body), alice)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "personal")
	w := httptest.NewRecorder()
	handler.AddressBookReport(db)(w, r)

	if w.Code != http.StatusMultiStatus {
		t.Fatalf("want 207, got %d\n%s", w.Code, w.Body.String())
	}
	assertXMLContains(t, w, "alice.vcf")
	assertXMLContains(t, w, "bob.vcf")
	assertXMLContains(t, w, "getetag")
	assertXMLContains(t, w, "address-data")
}

func TestAddressBookReport_Query_PropFilter_ReturnsOnlyMatch(t *testing.T) {
	db := newTestDB(t)
	setupReportFixtures(t, db)
	alice := &auth.Principal{Username: "alice"}

	body := `<?xml version="1.0" encoding="utf-8"?>
		<C:addressbook-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
			<D:prop><D:getetag/></D:prop>
			<C:filter>
				<C:prop-filter name="FN">
					<C:text-match>Alice</C:text-match>
				</C:prop-filter>
			</C:filter>
		</C:addressbook-query>`
	r := withPrincipal(reportReq(t, body), alice)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "personal")
	w := httptest.NewRecorder()
	handler.AddressBookReport(db)(w, r)

	if w.Code != http.StatusMultiStatus {
		t.Fatalf("want 207, got %d\n%s", w.Code, w.Body.String())
	}
	assertXMLContains(t, w, "alice.vcf")
	if strings.Contains(w.Body.String(), "bob.vcf") {
		t.Errorf("bob.vcf should not appear in filtered results")
	}
}

// --- addressbook-multiget ---

func TestAddressBookReport_Multiget_ReturnsBothContacts(t *testing.T) {
	db := newTestDB(t)
	setupReportFixtures(t, db)
	alice := &auth.Principal{Username: "alice"}

	body := `<?xml version="1.0" encoding="utf-8"?>
		<C:addressbook-multiget xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
			<D:prop><D:getetag/><C:address-data/></D:prop>
			<D:href>/dav/addressbooks/alice/personal/alice.vcf</D:href>
			<D:href>/dav/addressbooks/alice/personal/bob.vcf</D:href>
		</C:addressbook-multiget>`
	r := withPrincipal(reportReq(t, body), alice)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "personal")
	w := httptest.NewRecorder()
	handler.AddressBookReport(db)(w, r)

	if w.Code != http.StatusMultiStatus {
		t.Fatalf("want 207, got %d\n%s", w.Code, w.Body.String())
	}
	assertXMLContains(t, w, "alice.vcf")
	assertXMLContains(t, w, "bob.vcf")
	assertXMLContains(t, w, "FN:Alice Test")
	assertXMLContains(t, w, "FN:Bob Test")
}

func TestAddressBookReport_Multiget_NotFoundHrefGets404(t *testing.T) {
	db := newTestDB(t)
	setupReportFixtures(t, db)
	alice := &auth.Principal{Username: "alice"}

	body := `<?xml version="1.0" encoding="utf-8"?>
		<C:addressbook-multiget xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
			<D:prop><D:getetag/><C:address-data/></D:prop>
			<D:href>/dav/addressbooks/alice/personal/alice.vcf</D:href>
			<D:href>/dav/addressbooks/alice/personal/missing.vcf</D:href>
		</C:addressbook-multiget>`
	r := withPrincipal(reportReq(t, body), alice)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "personal")
	w := httptest.NewRecorder()
	handler.AddressBookReport(db)(w, r)

	if w.Code != http.StatusMultiStatus {
		t.Fatalf("want 207, got %d\n%s", w.Code, w.Body.String())
	}
	assertXMLContains(t, w, "alice.vcf")
	assertXMLContains(t, w, "missing.vcf")
	assertXMLContains(t, w, "404")
}

// --- auth/error cases ---

func TestAddressBookReport_NoAuth_Returns401(t *testing.T) {
	db := newTestDB(t)
	r, _ := http.NewRequestWithContext(context.Background(), "REPORT",
		"/dav/addressbooks/alice/personal/", http.NoBody)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "personal")
	w := httptest.NewRecorder()
	handler.AddressBookReport(db)(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

func TestAddressBookReport_EmptyBody_Returns400(t *testing.T) {
	db := newTestDB(t)
	setupReportFixtures(t, db)
	alice := &auth.Principal{Username: "alice"}

	r, _ := http.NewRequestWithContext(context.Background(), "REPORT",
		"/dav/addressbooks/alice/personal/", http.NoBody)
	r = withPrincipal(r, alice)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "personal")
	w := httptest.NewRecorder()
	handler.AddressBookReport(db)(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", w.Code)
	}
}

func TestAddressBookReport_ForbiddenForOtherUser(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	if _, err := local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false); err != nil {
		t.Fatalf("create user: %v", err)
	}
	bob := &auth.Principal{Username: "bob", IsAdmin: false}

	body := `<?xml version="1.0"?><C:addressbook-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav"><D:prop><D:getetag/></D:prop></C:addressbook-query>`
	r, _ := http.NewRequestWithContext(ctx, "REPORT",
		"/dav/addressbooks/alice/personal/", strings.NewReader(body))
	r = withPrincipal(r, bob)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "personal")
	w := httptest.NewRecorder()
	handler.AddressBookReport(db)(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", w.Code)
	}
}
