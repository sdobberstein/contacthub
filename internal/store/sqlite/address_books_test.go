package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/sdobberstein/contacthub/internal/store"
)

func makeAddressBook(id, userID, name string) *store.AddressBook {
	now := time.Now()
	return &store.AddressBook{
		ID:          id,
		UserID:      userID,
		Name:        name,
		DisplayName: name,
		SyncToken:   0,
		CTag:        "0",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestAddressBookStore_CreateAndGet(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	_ = db.CreateUser(ctx, makeUser("u1", "alice"))
	ab := makeAddressBook("ab1", "u1", "personal")
	if err := db.CreateAddressBook(ctx, ab); err != nil {
		t.Fatalf("CreateAddressBook: %v", err)
	}

	got, err := db.GetAddressBook(ctx, "ab1")
	if err != nil {
		t.Fatalf("GetAddressBook: %v", err)
	}
	if got.Name != "personal" {
		t.Errorf("name: want personal, got %q", got.Name)
	}

	got2, err := db.GetAddressBookByName(ctx, "u1", "personal")
	if err != nil {
		t.Fatalf("GetAddressBookByName: %v", err)
	}
	if got2.ID != "ab1" {
		t.Errorf("id: want ab1, got %q", got2.ID)
	}
}

func TestAddressBookStore_CreateConflict(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	_ = db.CreateUser(ctx, makeUser("u1", "alice"))
	_ = db.CreateAddressBook(ctx, makeAddressBook("ab1", "u1", "personal"))
	err := db.CreateAddressBook(ctx, makeAddressBook("ab2", "u1", "personal"))
	if err != store.ErrConflict {
		t.Errorf("want ErrConflict, got %v", err)
	}
}

func TestAddressBookStore_GetNotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	if _, err := db.GetAddressBook(ctx, "nope"); err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
	if _, err := db.GetAddressBookByName(ctx, "u1", "nope"); err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestAddressBookStore_List(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	_ = db.CreateUser(ctx, makeUser("u1", "alice"))
	_ = db.CreateAddressBook(ctx, makeAddressBook("ab1", "u1", "work"))
	_ = db.CreateAddressBook(ctx, makeAddressBook("ab2", "u1", "personal"))

	books, err := db.ListAddressBooks(ctx, "u1")
	if err != nil {
		t.Fatalf("ListAddressBooks: %v", err)
	}
	if len(books) != 2 {
		t.Fatalf("want 2 books, got %d", len(books))
	}
	// Ordered by name.
	if books[0].Name != "personal" || books[1].Name != "work" {
		t.Errorf("wrong order: %q, %q", books[0].Name, books[1].Name)
	}
}

func TestAddressBookStore_Update(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	_ = db.CreateUser(ctx, makeUser("u1", "alice"))
	ab := makeAddressBook("ab1", "u1", "personal")
	_ = db.CreateAddressBook(ctx, ab)

	ab.DisplayName = "Personal Updated"
	ab.UpdatedAt = time.Now()
	if err := db.UpdateAddressBook(ctx, ab); err != nil {
		t.Fatalf("UpdateAddressBook: %v", err)
	}

	got, _ := db.GetAddressBook(ctx, "ab1")
	if got.DisplayName != "Personal Updated" {
		t.Errorf("DisplayName not updated, got %q", got.DisplayName)
	}
}

func TestAddressBookStore_Delete(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	_ = db.CreateUser(ctx, makeUser("u1", "alice"))
	_ = db.CreateAddressBook(ctx, makeAddressBook("ab1", "u1", "personal"))

	if err := db.DeleteAddressBook(ctx, "ab1"); err != nil {
		t.Fatalf("DeleteAddressBook: %v", err)
	}
	if _, err := db.GetAddressBook(ctx, "ab1"); err != store.ErrNotFound {
		t.Errorf("want ErrNotFound after delete, got %v", err)
	}
}

func TestAddressBookStore_BumpSyncToken(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	_ = db.CreateUser(ctx, makeUser("u1", "alice"))
	_ = db.CreateAddressBook(ctx, makeAddressBook("ab1", "u1", "personal"))

	tok1, err := db.BumpSyncToken(ctx, "ab1")
	if err != nil {
		t.Fatalf("BumpSyncToken: %v", err)
	}
	if tok1 != 1 {
		t.Errorf("first bump: want 1, got %d", tok1)
	}

	tok2, err := db.BumpSyncToken(ctx, "ab1")
	if err != nil {
		t.Fatalf("BumpSyncToken: %v", err)
	}
	if tok2 != 2 {
		t.Errorf("second bump: want 2, got %d", tok2)
	}
}
