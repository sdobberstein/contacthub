package sqlite_test

import (
	"context"
	"testing"

	"github.com/sdobberstein/contacthub/internal/migrations"
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
