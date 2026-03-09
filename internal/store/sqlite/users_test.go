package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/sdobberstein/contacthub/internal/store"
)

func makeUser(id, username string) *store.User {
	now := time.Now()
	return &store.User{ //nolint:gosec // PasswordHash is a test fixture, not a real credential
		ID:          id,
		Username:    username,
		DisplayName: username,
		PasswordHash: "$argon2id$v=19$m=65536,t=1,p=4$fake$fake",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestUserStore_CreateAndGet(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	u := makeUser("u1", "alice")
	if err := db.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := db.GetUserByID(ctx, "u1")
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if got.Username != "alice" {
		t.Errorf("username: want alice, got %q", got.Username)
	}

	got2, err := db.GetUserByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if got2.ID != "u1" {
		t.Errorf("id: want u1, got %q", got2.ID)
	}
}

func TestUserStore_CreateConflict(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	if err := db.CreateUser(ctx, makeUser("u1", "alice")); err != nil {
		t.Fatalf("first create: %v", err)
	}
	err := db.CreateUser(ctx, makeUser("u2", "alice"))
	if err != store.ErrConflict {
		t.Errorf("want ErrConflict, got %v", err)
	}
}

func TestUserStore_GetNotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	if _, err := db.GetUserByID(ctx, "nope"); err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
	if _, err := db.GetUserByUsername(ctx, "nope"); err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestUserStore_Update(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	u := makeUser("u1", "alice")
	if err := db.CreateUser(ctx, u); err != nil {
		t.Fatalf("create: %v", err)
	}

	u.DisplayName = "Alice Updated"
	u.UpdatedAt = time.Now()
	if err := db.UpdateUser(ctx, u); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := db.GetUserByID(ctx, "u1")
	if got.DisplayName != "Alice Updated" {
		t.Errorf("display_name not updated, got %q", got.DisplayName)
	}
}

func TestUserStore_UpdateNotFound(t *testing.T) {
	db := newTestDB(t)
	u := makeUser("ghost", "ghost")
	if err := db.UpdateUser(context.Background(), u); err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestUserStore_UpdateConflict(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	_ = db.CreateUser(ctx, makeUser("u1", "alice"))
	_ = db.CreateUser(ctx, makeUser("u2", "bob"))

	u2, _ := db.GetUserByID(ctx, "u2")
	u2.Username = "alice" // collides
	if err := db.UpdateUser(ctx, u2); err != store.ErrConflict {
		t.Errorf("want ErrConflict, got %v", err)
	}
}

func TestUserStore_Delete(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	_ = db.CreateUser(ctx, makeUser("u1", "alice"))

	if err := db.DeleteUser(ctx, "u1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := db.GetUserByID(ctx, "u1"); err != store.ErrNotFound {
		t.Errorf("want ErrNotFound after delete, got %v", err)
	}
}

func TestUserStore_DeleteNotFound(t *testing.T) {
	db := newTestDB(t)
	if err := db.DeleteUser(context.Background(), "ghost"); err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestUserStore_ListAndCount(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	n, _ := db.CountUsers(ctx)
	if n != 0 {
		t.Errorf("want 0, got %d", n)
	}

	_ = db.CreateUser(ctx, makeUser("u1", "bob"))
	_ = db.CreateUser(ctx, makeUser("u2", "alice"))

	n, _ = db.CountUsers(ctx)
	if n != 2 {
		t.Errorf("want 2, got %d", n)
	}

	users, err := db.ListUsers(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("want 2 users, got %d", len(users))
	}
	// ListUsers orders by username.
	if users[0].Username != "alice" || users[1].Username != "bob" {
		t.Errorf("wrong order: %q, %q", users[0].Username, users[1].Username)
	}
}
