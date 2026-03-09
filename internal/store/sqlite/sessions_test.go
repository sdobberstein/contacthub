package sqlite_test

import (
	"context"
	"testing"
	"time"

	"github.com/sdobberstein/contacthub/internal/store"
)

func makeSession(id, userID string, expiresAt time.Time) *store.Session {
	return &store.Session{
		ID:        id,
		UserID:    userID,
		IPAddress: "127.0.0.1",
		UserAgent: "test",
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}
}

func TestSessionStore_CreateAndGet(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	_ = db.CreateUser(ctx, makeUser("u1", "alice"))

	exp := time.Now().Add(time.Hour)
	s := makeSession("s1", "u1", exp)
	if err := db.CreateSession(ctx, s); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	got, err := db.GetSession(ctx, "s1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.UserID != "u1" {
		t.Errorf("user_id: want u1, got %q", got.UserID)
	}
	// ExpiresAt round-trips through RFC3339 (second precision).
	if got.ExpiresAt.Unix() != exp.Unix() {
		t.Errorf("expires_at mismatch: want %v, got %v", exp.Unix(), got.ExpiresAt.Unix())
	}
}

func TestSessionStore_GetNotFound(t *testing.T) {
	db := newTestDB(t)
	if _, err := db.GetSession(context.Background(), "nope"); err != store.ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestSessionStore_Delete(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	_ = db.CreateUser(ctx, makeUser("u1", "alice"))
	_ = db.CreateSession(ctx, makeSession("s1", "u1", time.Now().Add(time.Hour)))

	if err := db.DeleteSession(ctx, "s1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := db.GetSession(ctx, "s1"); err != store.ErrNotFound {
		t.Errorf("want ErrNotFound after delete, got %v", err)
	}
}

func TestSessionStore_DeleteUserSessions(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	_ = db.CreateUser(ctx, makeUser("u1", "alice"))
	_ = db.CreateUser(ctx, makeUser("u2", "bob"))
	_ = db.CreateSession(ctx, makeSession("s1", "u1", time.Now().Add(time.Hour)))
	_ = db.CreateSession(ctx, makeSession("s2", "u1", time.Now().Add(time.Hour)))
	_ = db.CreateSession(ctx, makeSession("s3", "u2", time.Now().Add(time.Hour)))

	if err := db.DeleteUserSessions(ctx, "u1"); err != nil {
		t.Fatalf("delete user sessions: %v", err)
	}

	if _, err := db.GetSession(ctx, "s1"); err != store.ErrNotFound {
		t.Error("s1 should be gone")
	}
	if _, err := db.GetSession(ctx, "s2"); err != store.ErrNotFound {
		t.Error("s2 should be gone")
	}
	// u2's session must survive.
	if _, err := db.GetSession(ctx, "s3"); err != nil {
		t.Errorf("s3 should survive, got %v", err)
	}
}

func TestSessionStore_PurgeExpired(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	_ = db.CreateUser(ctx, makeUser("u1", "alice"))
	_ = db.CreateSession(ctx, makeSession("valid", "u1", time.Now().Add(time.Hour)))
	_ = db.CreateSession(ctx, makeSession("expired", "u1", time.Now().Add(-time.Hour)))

	if err := db.PurgeExpiredSessions(ctx); err != nil {
		t.Fatalf("purge: %v", err)
	}

	if _, err := db.GetSession(ctx, "expired"); err != store.ErrNotFound {
		t.Error("expired session should be purged")
	}
	if _, err := db.GetSession(ctx, "valid"); err != nil {
		t.Errorf("valid session should survive, got %v", err)
	}
}
