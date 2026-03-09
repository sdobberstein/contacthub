//nolint:revive // stub implementations pending Phase 4+ — doc comments omitted intentionally
package sqlite

import (
	"context"
	"time"

	"github.com/sdobberstein/contacthub/internal/store"
)

// --- AddressBookStore ---

func (d *DB) CreateAddressBook(_ context.Context, _ *store.AddressBook) error {
	return store.ErrNotImplemented
}
func (d *DB) GetAddressBook(_ context.Context, _ string) (*store.AddressBook, error) {
	return nil, store.ErrNotImplemented
}
func (d *DB) GetAddressBookByName(_ context.Context, _, _ string) (*store.AddressBook, error) {
	return nil, store.ErrNotImplemented
}
func (d *DB) ListAddressBooks(_ context.Context, _ string) ([]*store.AddressBook, error) {
	return nil, store.ErrNotImplemented
}
func (d *DB) UpdateAddressBook(_ context.Context, _ *store.AddressBook) error {
	return store.ErrNotImplemented
}
func (d *DB) DeleteAddressBook(_ context.Context, _ string) error {
	return store.ErrNotImplemented
}
func (d *DB) BumpSyncToken(_ context.Context, _ string) (int64, error) {
	return 0, store.ErrNotImplemented
}

// --- ContactStore ---

func (d *DB) CreateContact(_ context.Context, _ *store.Contact) error {
	return store.ErrNotImplemented
}
func (d *DB) GetContactByFilename(_ context.Context, _, _ string) (*store.Contact, error) {
	return nil, store.ErrNotImplemented
}
func (d *DB) GetContactByUID(_ context.Context, _, _ string) (*store.Contact, error) {
	return nil, store.ErrNotImplemented
}
func (d *DB) ListContacts(_ context.Context, _ string) ([]*store.Contact, error) {
	return nil, store.ErrNotImplemented
}
func (d *DB) UpdateContact(_ context.Context, _ *store.Contact) error {
	return store.ErrNotImplemented
}
func (d *DB) DeleteContact(_ context.Context, _ string) error {
	return store.ErrNotImplemented
}

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

// --- PropertyStore ---

func (d *DB) SetProperty(_ context.Context, _ *store.Property) error {
	return store.ErrNotImplemented
}
func (d *DB) GetProperty(_ context.Context, _, _, _ string) (*store.Property, error) {
	return nil, store.ErrNotImplemented
}
func (d *DB) ListProperties(_ context.Context, _ string) ([]*store.Property, error) {
	return nil, store.ErrNotImplemented
}
func (d *DB) DeleteProperty(_ context.Context, _, _, _ string) error {
	return store.ErrNotImplemented
}
func (d *DB) DeleteResourceProperties(_ context.Context, _ string) error {
	return store.ErrNotImplemented
}

