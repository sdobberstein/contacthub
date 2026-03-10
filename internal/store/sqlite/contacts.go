package sqlite

import (
	"context"
	"strings"
	"time"

	"github.com/sdobberstein/contacthub/internal/store"
)

// CreateContact inserts a new contact and updates the FTS index.
func (d *DB) CreateContact(ctx context.Context, c *store.Contact) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // rollback on error path; commit error checked below

	_, err = tx.ExecContext(ctx,
		`INSERT INTO contacts
		 (id, uid, address_book_id, filename, etag, vcard, fn, kind, organization,
		  birthday, anniversary, photo_size, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.UID, c.AddressBookID, c.Filename, c.ETag, c.VCard,
		c.FN, c.Kind, c.Organization, c.Birthday, c.Anniversary, c.PhotoSize,
		c.CreatedAt.UTC().Format(time.RFC3339),
		c.UpdatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		if isUniqueConstraint(err) {
			return store.ErrConflict
		}
		return err
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO contacts_fts (contact_id, fn, emails, phones, organization, notes)
		 VALUES (?, ?, '', '', ?, '')`,
		c.ID, c.FN, c.Organization,
	); err != nil {
		return err
	}

	return tx.Commit()
}

// GetContactByFilename returns the contact with the given filename in the address book.
func (d *DB) GetContactByFilename(ctx context.Context, addressBookID, filename string) (*store.Contact, error) {
	row := d.db.QueryRowContext(ctx,
		`SELECT id, uid, address_book_id, filename, etag, vcard, fn, kind, organization,
		        birthday, anniversary, photo_size, created_at, updated_at
		 FROM contacts WHERE address_book_id = ? AND filename = ?`,
		addressBookID, filename)
	c, err := scanContact(row)
	return c, mapNoRows(err)
}

// GetContactByUID returns the contact with the given UID in the address book.
func (d *DB) GetContactByUID(ctx context.Context, addressBookID, uid string) (*store.Contact, error) {
	row := d.db.QueryRowContext(ctx,
		`SELECT id, uid, address_book_id, filename, etag, vcard, fn, kind, organization,
		        birthday, anniversary, photo_size, created_at, updated_at
		 FROM contacts WHERE address_book_id = ? AND uid = ?`,
		addressBookID, uid)
	c, err := scanContact(row)
	return c, mapNoRows(err)
}

// ListContacts returns all contacts in the address book, ordered by filename.
func (d *DB) ListContacts(ctx context.Context, addressBookID string) ([]*store.Contact, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT id, uid, address_book_id, filename, etag, vcard, fn, kind, organization,
		        birthday, anniversary, photo_size, created_at, updated_at
		 FROM contacts WHERE address_book_id = ? ORDER BY filename`,
		addressBookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // read-only rows, close error not actionable

	var contacts []*store.Contact
	for rows.Next() {
		c, err := scanContact(rows)
		if err != nil {
			return nil, err
		}
		contacts = append(contacts, c)
	}
	return contacts, rows.Err()
}

// UpdateContact updates all mutable fields of an existing contact and refreshes the FTS index.
func (d *DB) UpdateContact(ctx context.Context, c *store.Contact) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // rollback on error path; commit error checked below

	res, err := tx.ExecContext(ctx,
		`UPDATE contacts
		 SET uid=?, filename=?, etag=?, vcard=?, fn=?, kind=?, organization=?,
		     birthday=?, anniversary=?, photo_size=?, updated_at=?
		 WHERE id=?`,
		c.UID, c.Filename, c.ETag, c.VCard, c.FN, c.Kind, c.Organization,
		c.Birthday, c.Anniversary, c.PhotoSize,
		c.UpdatedAt.UTC().Format(time.RFC3339),
		c.ID,
	)
	if err != nil {
		if isUniqueConstraint(err) {
			return store.ErrConflict
		}
		return err
	}
	if err := requireOneRow(res); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM contacts_fts WHERE contact_id = ?`, c.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO contacts_fts (contact_id, fn, emails, phones, organization, notes)
		 VALUES (?, ?, '', '', ?, '')`,
		c.ID, c.FN, c.Organization,
	); err != nil {
		return err
	}

	return tx.Commit()
}

// DeleteContact removes the contact with the given ID and its FTS entry.
func (d *DB) DeleteContact(ctx context.Context, id string) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // rollback on error path; commit error checked below

	if _, err := tx.ExecContext(ctx, `DELETE FROM contacts_fts WHERE contact_id = ?`, id); err != nil {
		return err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM contacts WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if err := requireOneRow(res); err != nil {
		return err
	}
	return tx.Commit()
}

// SearchContacts returns contacts in addressBookID matching the optional filter.
// A nil filter or empty filter fields is equivalent to ListContacts.
// PropName "FN" and "ORG"/"ORGANIZATION" match against indexed columns;
// any other PropName falls back to a substring search on the full vCard text.
func (d *DB) SearchContacts(ctx context.Context, addressBookID string, filter *store.ContactFilter) ([]*store.Contact, error) {
	if filter == nil || filter.PropName == "" || filter.TextMatch == "" {
		return d.ListContacts(ctx, addressBookID)
	}

	// LIKE pattern for case-insensitive contains match (RFC 6352 §8.6.4 default).
	pattern := "%" + filter.TextMatch + "%"

	const (
		queryFN   = `SELECT id, uid, address_book_id, filename, etag, vcard, fn, kind, organization, birthday, anniversary, photo_size, created_at, updated_at FROM contacts WHERE address_book_id = ? AND fn LIKE ? ORDER BY filename`
		queryOrg  = `SELECT id, uid, address_book_id, filename, etag, vcard, fn, kind, organization, birthday, anniversary, photo_size, created_at, updated_at FROM contacts WHERE address_book_id = ? AND organization LIKE ? ORDER BY filename`
		queryVCard = `SELECT id, uid, address_book_id, filename, etag, vcard, fn, kind, organization, birthday, anniversary, photo_size, created_at, updated_at FROM contacts WHERE address_book_id = ? AND vcard LIKE ? ORDER BY filename`
	)

	var query string
	switch strings.ToUpper(filter.PropName) {
	case "FN":
		query = queryFN
	case "ORG", "ORGANIZATION":
		query = queryOrg
	default:
		query = queryVCard
	}

	rows, err := d.db.QueryContext(ctx, query, addressBookID, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // read-only rows, close error not actionable

	var contacts []*store.Contact
	for rows.Next() {
		c, err := scanContact(rows)
		if err != nil {
			return nil, err
		}
		contacts = append(contacts, c)
	}
	return contacts, rows.Err()
}

// --- helpers ---

func scanContact(s rowScanner) (*store.Contact, error) {
	c := &store.Contact{}
	var createdAt, updatedAt string
	var fn, kind, org, bday, anniversary *string
	var photoSize *int
	if err := s.Scan(
		&c.ID, &c.UID, &c.AddressBookID, &c.Filename, &c.ETag, &c.VCard,
		&fn, &kind, &org, &bday, &anniversary, &photoSize,
		&createdAt, &updatedAt,
	); err != nil {
		return nil, err
	}
	if fn != nil {
		c.FN = *fn
	}
	if kind != nil {
		c.Kind = *kind
	}
	if org != nil {
		c.Organization = *org
	}
	if bday != nil {
		c.Birthday = *bday
	}
	if anniversary != nil {
		c.Anniversary = *anniversary
	}
	if photoSize != nil {
		c.PhotoSize = *photoSize
	}
	c.CreatedAt = mustParseRFC3339(createdAt)
	c.UpdatedAt = mustParseRFC3339(updatedAt)
	return c, nil
}
