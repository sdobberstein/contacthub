package local_test

import (
	"context"
	"testing"

	"github.com/sdobberstein/contacthub/internal/auth"
	"github.com/sdobberstein/contacthub/internal/auth/local"
	"github.com/sdobberstein/contacthub/internal/migrations"
	"github.com/sdobberstein/contacthub/internal/store"
	"github.com/sdobberstein/contacthub/internal/store/sqlite"
)

func newTestDB(t *testing.T) *sqlite.DB {
	t.Helper()
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := migrations.Run(context.Background(), db.DB()); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() }) //nolint:errcheck // best-effort cleanup in test
	return db
}

func TestHashPassword_Format(t *testing.T) {
	hash, err := local.HashPassword("hunter2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if hash[:10] != "$argon2id$" {
		t.Errorf("unexpected hash prefix: %q", hash[:10])
	}
}

func TestHashPassword_Unique(t *testing.T) {
	h1, _ := local.HashPassword("samepassword")
	h2, _ := local.HashPassword("samepassword")
	if h1 == h2 {
		t.Error("expected different hashes (random salt), got identical")
	}
}

func TestAuthenticate_ValidCredentials(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	if _, err := local.CreateUser(ctx, db, "alice", "Alice", "correcthorse", false); err != nil {
		t.Fatalf("create user: %v", err)
	}

	p := local.New(db)
	user, err := p.Authenticate(ctx, "alice", "correcthorse")
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if user.Username != "alice" {
		t.Errorf("expected username alice, got %q", user.Username)
	}
}

func TestAuthenticate_WrongPassword(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	if _, err := local.CreateUser(ctx, db, "bob", "Bob", "rightpassword", false); err != nil {
		t.Fatalf("create user: %v", err)
	}

	p := local.New(db)
	_, err := p.Authenticate(ctx, "bob", "wrongpassword")
	if err != auth.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got: %v", err)
	}
}

func TestAuthenticate_UnknownUser(t *testing.T) {
	db := newTestDB(t)
	p := local.New(db)

	_, err := p.Authenticate(context.Background(), "nobody", "password")
	// Must not reveal whether user exists — same error as wrong password.
	if err != auth.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got: %v", err)
	}
}

func TestCreateUser_ConflictOnDuplicateUsername(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	if _, err := local.CreateUser(ctx, db, "carol", "Carol", "password1", false); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := local.CreateUser(ctx, db, "carol", "Carol 2", "password2", false)
	if err != store.ErrConflict {
		t.Errorf("expected ErrConflict, got: %v", err)
	}
}

func TestCreateUser_DefaultDisplayName(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	u, err := local.CreateUser(ctx, db, "dave", "", "password", false)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.DisplayName != "dave" {
		t.Errorf("expected display_name=dave, got %q", u.DisplayName)
	}
}
