//nolint:revive // stub implementations pending future phases — doc comments omitted intentionally
package sqlite

import (
	"context"
	"time"

	"github.com/sdobberstein/contacthub/internal/store"
)

// --- SyncStore ---

func (d *DB) AppendSyncLog(_ context.Context, _ *store.SyncLogEntry) error {
	return store.ErrNotImplemented
}
func (d *DB) GetSyncLogSince(_ context.Context, _ string, _ int64) ([]*store.SyncLogEntry, error) {
	return nil, store.ErrNotImplemented
}
func (d *DB) PurgeSyncLog(_ context.Context, _ string, _ time.Time) error {
	return store.ErrNotImplemented
}

// --- ACLStore ---

func (d *DB) GrantACL(_ context.Context, _ *store.ACLEntry) error {
	return store.ErrNotImplemented
}
func (d *DB) RevokeACL(_ context.Context, _, _ string) error {
	return store.ErrNotImplemented
}
func (d *DB) GetACL(_ context.Context, _, _ string) (*store.ACLEntry, error) {
	return nil, store.ErrNotImplemented
}
func (d *DB) ListACLByBook(_ context.Context, _ string) ([]*store.ACLEntry, error) {
	return nil, store.ErrNotImplemented
}
func (d *DB) ListACLByPrincipal(_ context.Context, _ string) ([]*store.ACLEntry, error) {
	return nil, store.ErrNotImplemented
}

// --- LockStore ---

func (d *DB) CreateLock(_ context.Context, _ *store.Lock) error {
	return store.ErrNotImplemented
}
func (d *DB) GetLock(_ context.Context, _ string) (*store.Lock, error) {
	return nil, store.ErrNotImplemented
}
func (d *DB) GetLocksByResource(_ context.Context, _ string) ([]*store.Lock, error) {
	return nil, store.ErrNotImplemented
}
func (d *DB) RefreshLock(_ context.Context, _ string, _ time.Time) error {
	return store.ErrNotImplemented
}
func (d *DB) DeleteLock(_ context.Context, _ string) error {
	return store.ErrNotImplemented
}
func (d *DB) PurgeExpiredLocks(_ context.Context) error {
	return store.ErrNotImplemented
}
