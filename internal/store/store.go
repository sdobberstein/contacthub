// Package store defines the storage interfaces and model types for contacthub.
package store

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// ErrNotImplemented is returned by stub store methods awaiting implementation.
var ErrNotImplemented = errors.New("not implemented")

// ErrConflict is returned when a uniqueness constraint would be violated.
var ErrConflict = errors.New("conflict")

// --- Model types ---

// User represents a contacthub account.
type User struct {
	ID           string
	Username     string
	DisplayName  string
	Email        string
	PasswordHash string
	IsAdmin      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Session represents an authenticated web session.
type Session struct {
	ID        string
	UserID    string
	IPAddress string
	UserAgent string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// AppPassword represents a per-device application password.
type AppPassword struct {
	ID         string
	UserID     string
	Name       string
	TokenHash  string
	LastUsedAt *time.Time
	CreatedAt  time.Time
}

// AddressBook represents a CardDAV address book collection.
type AddressBook struct {
	ID          string
	UserID      string
	Name        string // URL slug
	DisplayName string
	Description string
	Color       string
	SyncToken   int64
	CTag        string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Contact represents a single vCard resource within an address book.
type Contact struct {
	ID            string
	UID           string
	AddressBookID string
	Filename      string
	ETag          string
	VCard         string
	FN            string
	Kind          string
	Organization  string
	Birthday      string
	Anniversary   string
	PhotoSize     int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// SyncLogEntry records a single add/modify/delete event for delta sync.
type SyncLogEntry struct {
	ID            int64
	AddressBookID string
	Filename      string
	UID           string
	Operation     string // "added" | "modified" | "deleted"
	SyncToken     int64
	CreatedAt     time.Time
}

// ACLEntry grants a privilege on an address book to a principal.
type ACLEntry struct {
	AddressBookID string
	PrincipalID   string
	Privilege     string // "read" | "write" | "admin"
	GrantedBy     string
	CreatedAt     time.Time
}

// Lock represents a WebDAV lock on a resource.
type Lock struct {
	Token       string
	Resource    string
	Scope       string // "exclusive" | "shared"
	Depth       string // "0" | "infinity"
	OwnerXML    string
	PrincipalID string
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

// Property stores a dead WebDAV property for a resource.
type Property struct {
	Resource  string
	Namespace string
	Name      string
	Value     string
}

// AuditEntry records a security-relevant action.
type AuditEntry struct {
	ID         int64
	UserID     string
	Action     string
	Resource   string
	Detail     string // JSON
	IPAddress  string
	AuthMethod string // "session" | "app_password" | "proxy"
	CreatedAt  time.Time
}

// --- Store interfaces ---

// UserStore manages user accounts.
type UserStore interface {
	CreateUser(ctx context.Context, u *User) error
	GetUserByID(ctx context.Context, id string) (*User, error)
	GetUserByUsername(ctx context.Context, username string) (*User, error)
	UpdateUser(ctx context.Context, u *User) error
	DeleteUser(ctx context.Context, id string) error
	ListUsers(ctx context.Context) ([]*User, error)
	CountUsers(ctx context.Context) (int, error)
}

// SessionStore manages web sessions.
type SessionStore interface {
	CreateSession(ctx context.Context, s *Session) error
	GetSession(ctx context.Context, id string) (*Session, error)
	DeleteSession(ctx context.Context, id string) error
	DeleteUserSessions(ctx context.Context, userID string) error
	PurgeExpiredSessions(ctx context.Context) error
}

// AppPasswordStore manages application passwords.
type AppPasswordStore interface {
	CreateAppPassword(ctx context.Context, ap *AppPassword) error
	GetAppPasswordByTokenHash(ctx context.Context, hash string) (*AppPassword, error)
	ListAppPasswords(ctx context.Context, userID string) ([]*AppPassword, error)
	UpdateAppPasswordLastUsed(ctx context.Context, id string, t time.Time) error
	DeleteAppPassword(ctx context.Context, id string) error
}

// AddressBookStore manages address book collections.
type AddressBookStore interface {
	CreateAddressBook(ctx context.Context, ab *AddressBook) error
	GetAddressBook(ctx context.Context, id string) (*AddressBook, error)
	GetAddressBookByName(ctx context.Context, userID, name string) (*AddressBook, error)
	ListAddressBooks(ctx context.Context, userID string) ([]*AddressBook, error)
	UpdateAddressBook(ctx context.Context, ab *AddressBook) error
	DeleteAddressBook(ctx context.Context, id string) error
	BumpSyncToken(ctx context.Context, id string) (int64, error)
}

// ContactStore manages vCard resources.
type ContactStore interface {
	CreateContact(ctx context.Context, c *Contact) error
	GetContactByFilename(ctx context.Context, addressBookID, filename string) (*Contact, error)
	GetContactByUID(ctx context.Context, addressBookID, uid string) (*Contact, error)
	ListContacts(ctx context.Context, addressBookID string) ([]*Contact, error)
	UpdateContact(ctx context.Context, c *Contact) error
	DeleteContact(ctx context.Context, id string) error
}

// SyncStore manages delta-sync tokens and log entries.
type SyncStore interface {
	AppendSyncLog(ctx context.Context, entry *SyncLogEntry) error
	GetSyncLogSince(ctx context.Context, addressBookID string, syncToken int64) ([]*SyncLogEntry, error)
	PurgeSyncLog(ctx context.Context, addressBookID string, olderThan time.Time) error
}

// ACLStore manages address book access control.
type ACLStore interface {
	GrantACL(ctx context.Context, entry *ACLEntry) error
	RevokeACL(ctx context.Context, addressBookID, principalID string) error
	GetACL(ctx context.Context, addressBookID, principalID string) (*ACLEntry, error)
	ListACLByBook(ctx context.Context, addressBookID string) ([]*ACLEntry, error)
	ListACLByPrincipal(ctx context.Context, principalID string) ([]*ACLEntry, error)
}

// LockStore manages WebDAV locks.
type LockStore interface {
	CreateLock(ctx context.Context, l *Lock) error
	GetLock(ctx context.Context, token string) (*Lock, error)
	GetLocksByResource(ctx context.Context, resource string) ([]*Lock, error)
	RefreshLock(ctx context.Context, token string, expiresAt time.Time) error
	DeleteLock(ctx context.Context, token string) error
	PurgeExpiredLocks(ctx context.Context) error
}

// PropertyStore manages dead WebDAV properties.
type PropertyStore interface {
	SetProperty(ctx context.Context, p *Property) error
	GetProperty(ctx context.Context, resource, namespace, name string) (*Property, error)
	ListProperties(ctx context.Context, resource string) ([]*Property, error)
	DeleteProperty(ctx context.Context, resource, namespace, name string) error
	DeleteResourceProperties(ctx context.Context, resource string) error
}

// AuditStore records security-relevant events.
type AuditStore interface {
	AppendAudit(ctx context.Context, entry *AuditEntry) error
	PurgeAuditLog(ctx context.Context, olderThan time.Time) error
}

// Store is the aggregate of all store interfaces, satisfied by the SQLite implementation.
type Store interface {
	UserStore
	SessionStore
	AppPasswordStore
	AddressBookStore
	ContactStore
	SyncStore
	ACLStore
	LockStore
	PropertyStore
	AuditStore
	Ping(ctx context.Context) error
	Close() error
}
