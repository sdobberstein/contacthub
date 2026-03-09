CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    display_name  TEXT NOT NULL,
    email         TEXT,
    password_hash TEXT NOT NULL,
    is_admin      INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);

CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ip_address TEXT,
    user_agent TEXT,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);
CREATE INDEX sessions_user_idx    ON sessions(user_id);
CREATE INDEX sessions_expires_idx ON sessions(expires_at);

CREATE TABLE app_passwords (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    token_hash   TEXT NOT NULL,
    last_used_at TEXT,
    created_at   TEXT NOT NULL
);
CREATE INDEX app_passwords_user_idx ON app_passwords(user_id);

CREATE TABLE address_books (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    display_name TEXT NOT NULL,
    description  TEXT,
    color        TEXT,
    sync_token   INTEGER NOT NULL DEFAULT 0,
    ctag         TEXT NOT NULL,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL,
    UNIQUE(user_id, name)
);
CREATE INDEX address_books_user_idx ON address_books(user_id);

CREATE TABLE contacts (
    id              TEXT PRIMARY KEY,
    uid             TEXT NOT NULL,
    address_book_id TEXT NOT NULL REFERENCES address_books(id) ON DELETE CASCADE,
    filename        TEXT NOT NULL,
    etag            TEXT NOT NULL,
    vcard           TEXT NOT NULL,
    fn              TEXT,
    kind            TEXT DEFAULT 'individual',
    organization    TEXT,
    birthday        TEXT,
    anniversary     TEXT,
    photo_size      INTEGER,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    UNIQUE(address_book_id, filename),
    UNIQUE(address_book_id, uid)
);
CREATE INDEX contacts_address_book_idx ON contacts(address_book_id);
CREATE INDEX contacts_uid_idx          ON contacts(uid);
CREATE INDEX contacts_fn_idx           ON contacts(fn COLLATE NOCASE);
CREATE INDEX contacts_kind_idx         ON contacts(kind);

CREATE TABLE contact_emails (
    id         TEXT PRIMARY KEY,
    contact_id TEXT NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    email      TEXT NOT NULL,
    type       TEXT,
    preferred  INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX contact_emails_contact_idx ON contact_emails(contact_id);
CREATE INDEX contact_emails_email_idx   ON contact_emails(email COLLATE NOCASE);

CREATE TABLE contact_phones (
    id         TEXT PRIMARY KEY,
    contact_id TEXT NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
    phone      TEXT NOT NULL,
    type       TEXT,
    preferred  INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX contact_phones_contact_idx ON contact_phones(contact_id);

CREATE VIRTUAL TABLE contacts_fts USING fts5(
    contact_id UNINDEXED,
    fn,
    emails,
    phones,
    organization,
    notes,
    tokenize = 'unicode61 remove_diacritics 1'
);

CREATE TABLE sync_log (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    address_book_id TEXT NOT NULL REFERENCES address_books(id) ON DELETE CASCADE,
    filename        TEXT NOT NULL,
    uid             TEXT NOT NULL,
    operation       TEXT NOT NULL,
    sync_token      INTEGER NOT NULL,
    created_at      TEXT NOT NULL
);
CREATE INDEX sync_log_book_token_idx ON sync_log(address_book_id, sync_token);

CREATE TABLE address_book_acl (
    address_book_id TEXT NOT NULL REFERENCES address_books(id) ON DELETE CASCADE,
    principal_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    privilege       TEXT NOT NULL CHECK(privilege IN ('read', 'write', 'admin')),
    granted_by      TEXT NOT NULL REFERENCES users(id),
    created_at      TEXT NOT NULL,
    PRIMARY KEY (address_book_id, principal_id)
);
CREATE INDEX acl_principal_idx ON address_book_acl(principal_id);

CREATE TABLE locks (
    token        TEXT PRIMARY KEY,
    resource     TEXT NOT NULL,
    scope        TEXT NOT NULL CHECK(scope IN ('exclusive', 'shared')),
    depth        TEXT NOT NULL CHECK(depth IN ('0', 'infinity')),
    owner_xml    TEXT,
    principal_id TEXT REFERENCES users(id),
    expires_at   TEXT NOT NULL,
    created_at   TEXT NOT NULL
);
CREATE INDEX locks_resource_idx ON locks(resource);
CREATE INDEX locks_expires_idx  ON locks(expires_at);

CREATE TABLE properties (
    resource  TEXT NOT NULL,
    namespace TEXT NOT NULL,
    name      TEXT NOT NULL,
    value     TEXT,
    PRIMARY KEY (resource, namespace, name)
);
CREATE INDEX properties_resource_idx ON properties(resource);

CREATE TABLE audit_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id     TEXT REFERENCES users(id),
    action      TEXT NOT NULL,
    resource    TEXT,
    detail      TEXT,
    ip_address  TEXT,
    auth_method TEXT,
    created_at  TEXT NOT NULL
);
CREATE INDEX audit_log_user_idx    ON audit_log(user_id);
CREATE INDEX audit_log_created_idx ON audit_log(created_at);
