package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/sdobberstein/contacthub/internal/store"
)

func makeAppPassword(id, userID, name, tokenHash string) *store.AppPassword {
	return &store.AppPassword{
		ID:        id,
		UserID:    userID,
		Name:      name,
		TokenHash: tokenHash,
		CreatedAt: time.Now(),
	}
}

func TestAppPasswordStore_CreateAndGetByHash(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	_ = db.CreateUser(ctx, makeUser("u1", "alice"))

	ap := makeAppPassword("ap1", "u1", "my phone", "hash1")
	if err := db.CreateAppPassword(ctx, ap); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := db.GetAppPasswordByTokenHash(ctx, "hash1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "my phone" {
		t.Errorf("name: want %q, got %q", "my phone", got.Name)
	}
	if got.LastUsedAt != nil {
		t.Error("last_used_at should be nil initially")
	}
}

func TestAppPasswordStore_GetNotFound(t *testing.T) {
	db := newTestDB(t)
	if _, err := db.GetAppPasswordByTokenHash(context.Background(), "nohash"); err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestAppPasswordStore_List(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	_ = db.CreateUser(ctx, makeUser("u1", "alice"))
	_ = db.CreateUser(ctx, makeUser("u2", "bob"))
	_ = db.CreateAppPassword(ctx, makeAppPassword("ap1", "u1", "phone", "h1"))
	_ = db.CreateAppPassword(ctx, makeAppPassword("ap2", "u1", "tablet", "h2"))
	_ = db.CreateAppPassword(ctx, makeAppPassword("ap3", "u2", "phone", "h3"))

	aps, err := db.ListAppPasswords(ctx, "u1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(aps) != 2 {
		t.Errorf("want 2, got %d", len(aps))
	}

	empty, _ := db.ListAppPasswords(ctx, "u99")
	if len(empty) != 0 {
		t.Errorf("want 0 for unknown user, got %d", len(empty))
	}
}

func TestAppPasswordStore_UpdateLastUsed(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	_ = db.CreateUser(ctx, makeUser("u1", "alice"))
	_ = db.CreateAppPassword(ctx, makeAppPassword("ap1", "u1", "phone", "h1"))

	now := time.Now().Truncate(time.Second)
	if err := db.UpdateAppPasswordLastUsed(ctx, "ap1", now); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := db.GetAppPasswordByTokenHash(ctx, "h1")
	if got.LastUsedAt == nil {
		t.Fatal("last_used_at should be set")
	}
	if got.LastUsedAt.Unix() != now.Unix() {
		t.Errorf("last_used_at: want %v, got %v", now.Unix(), got.LastUsedAt.Unix())
	}
}

func TestAppPasswordStore_Delete(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	_ = db.CreateUser(ctx, makeUser("u1", "alice"))
	_ = db.CreateAppPassword(ctx, makeAppPassword("ap1", "u1", "phone", "h1"))

	if err := db.DeleteAppPassword(ctx, "ap1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := db.GetAppPasswordByTokenHash(ctx, "h1"); err != store.ErrNotFound {
		t.Errorf("want ErrNotFound after delete, got %v", err)
	}
}

func TestAppPasswordStore_DeleteNotFound(t *testing.T) {
	db := newTestDB(t)
	if err := db.DeleteAppPassword(context.Background(), "ghost"); err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}
