package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/sdobberstein/contacthub/internal/store"
)

func makeContact(id, uid, abID, filename string) *store.Contact {
	now := time.Now()
	return &store.Contact{
		ID:            id,
		UID:           uid,
		AddressBookID: abID,
		Filename:      filename,
		ETag:          "abcdef1234567890abcdef1234567890",
		VCard:         "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:" + uid + "\r\nFN:Test\r\nEND:VCARD\r\n",
		FN:            "Test",
		Kind:          "individual",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// setupBook creates a user and address book, returning the address book ID.
func setupBook(t *testing.T, db interface {
	CreateUser(context.Context, *store.User) error
	CreateAddressBook(context.Context, *store.AddressBook) error
}, userID, username, bookID, bookName string) {
	t.Helper()
	ctx := context.Background()
	_ = db.CreateUser(ctx, makeUser(userID, username))
	_ = db.CreateAddressBook(ctx, makeAddressBook(bookID, userID, bookName))
}

func TestContactStore_CreateAndGet(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	setupBook(t, db, "u1", "alice", "ab1", "personal")

	c := makeContact("c1", "urn:uuid:aaa", "ab1", "alice.vcf")
	if err := db.CreateContact(ctx, c); err != nil {
		t.Fatalf("CreateContact: %v", err)
	}

	got, err := db.GetContactByFilename(ctx, "ab1", "alice.vcf")
	if err != nil {
		t.Fatalf("GetContactByFilename: %v", err)
	}
	if got.UID != "urn:uuid:aaa" {
		t.Errorf("uid: got %q", got.UID)
	}

	got2, err := db.GetContactByUID(ctx, "ab1", "urn:uuid:aaa")
	if err != nil {
		t.Fatalf("GetContactByUID: %v", err)
	}
	if got2.Filename != "alice.vcf" {
		t.Errorf("filename: got %q", got2.Filename)
	}
}

func TestContactStore_CreateConflict_DuplicateFilename(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	setupBook(t, db, "u1", "alice", "ab1", "personal")

	_ = db.CreateContact(ctx, makeContact("c1", "urn:uuid:aaa", "ab1", "alice.vcf"))
	err := db.CreateContact(ctx, makeContact("c2", "urn:uuid:bbb", "ab1", "alice.vcf"))
	if err != store.ErrConflict {
		t.Errorf("want ErrConflict for duplicate filename, got %v", err)
	}
}

func TestContactStore_CreateConflict_DuplicateUID(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	setupBook(t, db, "u1", "alice", "ab1", "personal")

	_ = db.CreateContact(ctx, makeContact("c1", "urn:uuid:aaa", "ab1", "alice.vcf"))
	err := db.CreateContact(ctx, makeContact("c2", "urn:uuid:aaa", "ab1", "bob.vcf"))
	if err != store.ErrConflict {
		t.Errorf("want ErrConflict for duplicate UID, got %v", err)
	}
}

func TestContactStore_GetNotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	setupBook(t, db, "u1", "alice", "ab1", "personal")

	if _, err := db.GetContactByFilename(ctx, "ab1", "nope.vcf"); err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
	if _, err := db.GetContactByUID(ctx, "ab1", "urn:uuid:nope"); err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestContactStore_List(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	setupBook(t, db, "u1", "alice", "ab1", "personal")

	_ = db.CreateContact(ctx, makeContact("c1", "urn:uuid:bbb", "ab1", "bob.vcf"))
	_ = db.CreateContact(ctx, makeContact("c2", "urn:uuid:aaa", "ab1", "alice.vcf"))

	contacts, err := db.ListContacts(ctx, "ab1")
	if err != nil {
		t.Fatalf("ListContacts: %v", err)
	}
	if len(contacts) != 2 {
		t.Fatalf("want 2 contacts, got %d", len(contacts))
	}
	// Ordered by filename.
	if contacts[0].Filename != "alice.vcf" || contacts[1].Filename != "bob.vcf" {
		t.Errorf("wrong order: %q, %q", contacts[0].Filename, contacts[1].Filename)
	}
}

func TestContactStore_Update(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	setupBook(t, db, "u1", "alice", "ab1", "personal")

	c := makeContact("c1", "urn:uuid:aaa", "ab1", "alice.vcf")
	_ = db.CreateContact(ctx, c)

	c.FN = "Alice Updated"
	c.ETag = "newetaghex12345678901234567890ab"
	c.UpdatedAt = time.Now()
	if err := db.UpdateContact(ctx, c); err != nil {
		t.Fatalf("UpdateContact: %v", err)
	}

	got, _ := db.GetContactByFilename(ctx, "ab1", "alice.vcf")
	if got.FN != "Alice Updated" {
		t.Errorf("FN not updated, got %q", got.FN)
	}
	if got.ETag != "newetaghex12345678901234567890ab" {
		t.Errorf("ETag not updated, got %q", got.ETag)
	}
}

func TestContactStore_Update_RenamesFilename(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	setupBook(t, db, "u1", "alice", "ab1", "personal")

	c := makeContact("c1", "urn:uuid:aaa", "ab1", "alice.vcf")
	_ = db.CreateContact(ctx, c)

	c.Filename = "alice-renamed.vcf"
	c.UpdatedAt = time.Now()
	_ = db.UpdateContact(ctx, c)

	if _, err := db.GetContactByFilename(ctx, "ab1", "alice.vcf"); err != store.ErrNotFound {
		t.Error("old filename should not exist after rename")
	}
	if got, err := db.GetContactByFilename(ctx, "ab1", "alice-renamed.vcf"); err != nil || got == nil {
		t.Errorf("new filename should exist: %v", err)
	}
}

func TestContactStore_UpdateNotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	setupBook(t, db, "u1", "alice", "ab1", "personal")

	c := makeContact("ghost", "urn:uuid:ghost", "ab1", "ghost.vcf")
	if err := db.UpdateContact(ctx, c); err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestContactStore_Delete(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	setupBook(t, db, "u1", "alice", "ab1", "personal")

	_ = db.CreateContact(ctx, makeContact("c1", "urn:uuid:aaa", "ab1", "alice.vcf"))

	if err := db.DeleteContact(ctx, "c1"); err != nil {
		t.Fatalf("DeleteContact: %v", err)
	}
	if _, err := db.GetContactByFilename(ctx, "ab1", "alice.vcf"); err != store.ErrNotFound {
		t.Errorf("want ErrNotFound after delete, got %v", err)
	}
}

func TestContactStore_DeleteNotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	if err := db.DeleteContact(ctx, "ghost"); err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestContactStore_DeleteCascadesWithAddressBook(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	setupBook(t, db, "u1", "alice", "ab1", "personal")

	_ = db.CreateContact(ctx, makeContact("c1", "urn:uuid:aaa", "ab1", "alice.vcf"))
	_ = db.DeleteAddressBook(ctx, "ab1")

	// Contact should be gone (CASCADE).
	if _, err := db.GetContactByFilename(ctx, "ab1", "alice.vcf"); err != store.ErrNotFound {
		t.Errorf("want ErrNotFound after cascade delete, got %v", err)
	}
}
