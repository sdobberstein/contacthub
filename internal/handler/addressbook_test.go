package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sdobberstein/contacthub/internal/auth"
	"github.com/sdobberstein/contacthub/internal/auth/local"
	"github.com/sdobberstein/contacthub/internal/handler"
	"github.com/sdobberstein/contacthub/internal/store"
)

func makeBook(id, userID, name string) *store.AddressBook {
	now := time.Now()
	return &store.AddressBook{
		ID: id, UserID: userID, Name: name, DisplayName: name,
		SyncToken: 0, CTag: "0", CreatedAt: now, UpdatedAt: now,
	}
}

// --- HomeSetPropfind ---

func TestHomeSetPropfind_ReturnsCollectionType(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	if _, err := local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false); err != nil {
		t.Fatalf("create user: %v", err)
	}
	alice := &auth.Principal{Username: "alice"}

	r, _ := http.NewRequestWithContext(ctx, "PROPFIND", "/dav/addressbooks/alice/", http.NoBody)
	r.Header.Set("Depth", "0")
	r = withPrincipal(r, alice)
	r = withChiParam(r, "username", "alice")
	w := httptest.NewRecorder()
	handler.HomeSetPropfind(db)(w, r)

	if w.Code != http.StatusMultiStatus {
		t.Fatalf("want 207, got %d\n%s", w.Code, w.Body.String())
	}
	assertXMLContains(t, w, "<D:collection/>")
}

func TestHomeSetPropfind_Depth1_IncludesAddressBooks(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	user, _ := local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false)
	_ = db.CreateAddressBook(ctx, makeBook("ab1", user.ID, "personal"))

	alice := &auth.Principal{Username: "alice"}
	r, _ := http.NewRequestWithContext(ctx, "PROPFIND", "/dav/addressbooks/alice/", http.NoBody)
	r.Header.Set("Depth", "1")
	r = withPrincipal(r, alice)
	r = withChiParam(r, "username", "alice")
	w := httptest.NewRecorder()
	handler.HomeSetPropfind(db)(w, r)

	if w.Code != http.StatusMultiStatus {
		t.Fatalf("want 207, got %d", w.Code)
	}
	assertXMLContains(t, w, "personal")
	assertXMLContains(t, w, "<C:addressbook/>")
}

func TestHomeSetPropfind_InfinityDepthForbidden(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false) //nolint:errcheck // test setup
	alice := &auth.Principal{Username: "alice"}

	r, _ := http.NewRequestWithContext(ctx, "PROPFIND", "/dav/addressbooks/alice/", http.NoBody)
	r.Header.Set("Depth", "infinity")
	r = withPrincipal(r, alice)
	r = withChiParam(r, "username", "alice")
	w := httptest.NewRecorder()
	handler.HomeSetPropfind(db)(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", w.Code)
	}
}

func TestHomeSetPropfind_ForbiddenForOtherUser(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false) //nolint:errcheck // test setup
	bob := &auth.Principal{Username: "bob", IsAdmin: false}

	r, _ := http.NewRequestWithContext(ctx, "PROPFIND", "/dav/addressbooks/alice/", http.NoBody)
	r.Header.Set("Depth", "0")
	r = withPrincipal(r, bob)
	r = withChiParam(r, "username", "alice")
	w := httptest.NewRecorder()
	handler.HomeSetPropfind(db)(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", w.Code)
	}
}

func TestHomeSetPropfind_NoAuth_Returns401(t *testing.T) {
	db := newTestDB(t)
	r, _ := http.NewRequestWithContext(context.Background(), "PROPFIND", "/dav/addressbooks/alice/", http.NoBody)
	r = withChiParam(r, "username", "alice")
	w := httptest.NewRecorder()
	handler.HomeSetPropfind(db)(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

// --- AddressBookMkcol ---

func TestAddressBookMkcol_Creates201(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false) //nolint:errcheck // test setup
	alice := &auth.Principal{Username: "alice"}

	r, _ := http.NewRequestWithContext(ctx, "MKCOL", "/dav/addressbooks/alice/personal/", http.NoBody)
	r = withPrincipal(r, alice)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "personal")
	w := httptest.NewRecorder()
	handler.AddressBookMkcol(db)(w, r)

	if w.Code != http.StatusCreated {
		t.Errorf("want 201, got %d\n%s", w.Code, w.Body.String())
	}
}

func TestAddressBookMkcol_DuplicateReturns405(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	user, _ := local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false)
	_ = db.CreateAddressBook(ctx, makeBook("ab1", user.ID, "personal"))
	alice := &auth.Principal{Username: "alice"}

	r, _ := http.NewRequestWithContext(ctx, "MKCOL", "/dav/addressbooks/alice/personal/", http.NoBody)
	r = withPrincipal(r, alice)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "personal")
	w := httptest.NewRecorder()
	handler.AddressBookMkcol(db)(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("want 405, got %d", w.Code)
	}
}

func TestAddressBookMkcol_NoAuth_Returns401(t *testing.T) {
	db := newTestDB(t)
	r, _ := http.NewRequestWithContext(context.Background(), "MKCOL", "/dav/addressbooks/alice/personal/", http.NoBody)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "personal")
	w := httptest.NewRecorder()
	handler.AddressBookMkcol(db)(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("want 401, got %d", w.Code)
	}
}

// --- AddressBookPropfind ---

func TestAddressBookPropfind_Depth0_ReturnsBook(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	user, _ := local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false)
	_ = db.CreateAddressBook(ctx, makeBook("ab1", user.ID, "personal"))
	alice := &auth.Principal{Username: "alice"}

	r, _ := http.NewRequestWithContext(ctx, "PROPFIND", "/dav/addressbooks/alice/personal/", http.NoBody)
	r.Header.Set("Depth", "0")
	r = withPrincipal(r, alice)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "personal")
	w := httptest.NewRecorder()
	handler.AddressBookPropfind(db)(w, r)

	if w.Code != http.StatusMultiStatus {
		t.Fatalf("want 207, got %d\n%s", w.Code, w.Body.String())
	}
	assertXMLContains(t, w, "<C:addressbook/>")
	assertXMLContains(t, w, "personal")
}

func TestAddressBookPropfind_Depth1_IncludesContacts(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	user, _ := local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false)
	_ = db.CreateAddressBook(ctx, makeBook("ab1", user.ID, "personal"))
	_ = db.CreateContact(ctx, &store.Contact{
		ID: "c1", UID: "urn:uuid:aaa", AddressBookID: "ab1",
		Filename: "alice.vcf", ETag: "abc123",
		VCard:     "BEGIN:VCARD\r\nVERSION:4.0\r\nFN:Alice\r\nEND:VCARD\r\n",
		FN:        "Alice",
		Kind:      "individual",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	alice := &auth.Principal{Username: "alice"}

	r, _ := http.NewRequestWithContext(ctx, "PROPFIND", "/dav/addressbooks/alice/personal/", http.NoBody)
	r.Header.Set("Depth", "1")
	r = withPrincipal(r, alice)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "personal")
	w := httptest.NewRecorder()
	handler.AddressBookPropfind(db)(w, r)

	if w.Code != http.StatusMultiStatus {
		t.Fatalf("want 207, got %d\n%s", w.Code, w.Body.String())
	}
	assertXMLContains(t, w, "alice.vcf")
}

func TestAddressBookPropfind_NotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false) //nolint:errcheck // test setup
	alice := &auth.Principal{Username: "alice"}

	r, _ := http.NewRequestWithContext(ctx, "PROPFIND", "/dav/addressbooks/alice/nope/", http.NoBody)
	r.Header.Set("Depth", "0")
	r = withPrincipal(r, alice)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "nope")
	w := httptest.NewRecorder()
	handler.AddressBookPropfind(db)(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestAddressBookPropfind_InfinityForbidden(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	user, _ := local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false)
	_ = db.CreateAddressBook(ctx, makeBook("ab1", user.ID, "personal"))
	alice := &auth.Principal{Username: "alice"}

	r, _ := http.NewRequestWithContext(ctx, "PROPFIND", "/dav/addressbooks/alice/personal/", http.NoBody)
	r.Header.Set("Depth", "infinity")
	r = withPrincipal(r, alice)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "personal")
	w := httptest.NewRecorder()
	handler.AddressBookPropfind(db)(w, r)

	if w.Code != http.StatusForbidden {
		t.Errorf("want 403, got %d", w.Code)
	}
}

// --- AddressBookDelete ---

func TestAddressBookDelete_Returns204(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	user, _ := local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false)
	_ = db.CreateAddressBook(ctx, makeBook("ab1", user.ID, "personal"))
	alice := &auth.Principal{Username: "alice"}

	r, _ := http.NewRequestWithContext(ctx, http.MethodDelete, "/dav/addressbooks/alice/personal/", http.NoBody)
	r = withPrincipal(r, alice)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "personal")
	w := httptest.NewRecorder()
	handler.AddressBookDelete(db)(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d", w.Code)
	}

	// Verify the book is gone.
	if _, err := db.GetAddressBook(ctx, "ab1"); err == nil {
		t.Error("address book should be deleted")
	}
}

func TestAddressBookDelete_NotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	local.CreateUser(ctx, db, "alice", "Alice", "pass123456", false) //nolint:errcheck // test setup
	alice := &auth.Principal{Username: "alice"}

	r, _ := http.NewRequestWithContext(ctx, http.MethodDelete, "/dav/addressbooks/alice/nope/", http.NoBody)
	r = withPrincipal(r, alice)
	r = withChiParam(r, "username", "alice")
	r = withChiParam(r, "book", "nope")
	w := httptest.NewRecorder()
	handler.AddressBookDelete(db)(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

