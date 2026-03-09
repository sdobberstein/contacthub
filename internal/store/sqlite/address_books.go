package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/sdobberstein/contacthub/internal/store"
)

// CreateAddressBook inserts a new address book.
func (d *DB) CreateAddressBook(ctx context.Context, ab *store.AddressBook) error {
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO address_books
		 (id, user_id, name, display_name, description, color, sync_token, ctag, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ab.ID, ab.UserID, ab.Name, ab.DisplayName, ab.Description, ab.Color,
		ab.SyncToken, ab.CTag,
		ab.CreatedAt.UTC().Format(time.RFC3339),
		ab.UpdatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		if isUniqueConstraint(err) {
			return store.ErrConflict
		}
		return err
	}
	return nil
}

// GetAddressBook returns the address book with the given ID, or store.ErrNotFound.
func (d *DB) GetAddressBook(ctx context.Context, id string) (*store.AddressBook, error) {
	row := d.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, display_name, description, color, sync_token, ctag, created_at, updated_at
		 FROM address_books WHERE id = ?`, id)
	ab, err := scanAddressBook(row)
	return ab, mapNoRows(err)
}

// GetAddressBookByName returns the address book for userID with the given name slug.
func (d *DB) GetAddressBookByName(ctx context.Context, userID, name string) (*store.AddressBook, error) {
	row := d.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, display_name, description, color, sync_token, ctag, created_at, updated_at
		 FROM address_books WHERE user_id = ? AND name = ?`, userID, name)
	ab, err := scanAddressBook(row)
	return ab, mapNoRows(err)
}

// ListAddressBooks returns all address books for the given user, ordered by name.
func (d *DB) ListAddressBooks(ctx context.Context, userID string) ([]*store.AddressBook, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT id, user_id, name, display_name, description, color, sync_token, ctag, created_at, updated_at
		 FROM address_books WHERE user_id = ? ORDER BY name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // read-only rows, close error not actionable

	var books []*store.AddressBook
	for rows.Next() {
		ab, err := scanAddressBook(rows)
		if err != nil {
			return nil, err
		}
		books = append(books, ab)
	}
	return books, rows.Err()
}

// UpdateAddressBook updates all mutable fields of an address book.
func (d *DB) UpdateAddressBook(ctx context.Context, ab *store.AddressBook) error {
	res, err := d.db.ExecContext(ctx,
		`UPDATE address_books
		 SET display_name=?, description=?, color=?, updated_at=?
		 WHERE id=?`,
		ab.DisplayName, ab.Description, ab.Color,
		ab.UpdatedAt.UTC().Format(time.RFC3339),
		ab.ID,
	)
	if err != nil {
		return err
	}
	return requireOneRow(res)
}

// DeleteAddressBook removes the address book with the given ID.
func (d *DB) DeleteAddressBook(ctx context.Context, id string) error {
	res, err := d.db.ExecContext(ctx, `DELETE FROM address_books WHERE id=?`, id)
	if err != nil {
		return err
	}
	return requireOneRow(res)
}

// BumpSyncToken increments the address book's sync_token by 1, updates ctag,
// and returns the new sync_token value.
func (d *DB) BumpSyncToken(ctx context.Context, id string) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := d.db.ExecContext(ctx,
		`UPDATE address_books
		 SET sync_token = sync_token + 1,
		     ctag = CAST(sync_token + 1 AS TEXT),
		     updated_at = ?
		 WHERE id = ?`,
		now, id,
	)
	if err != nil {
		return 0, fmt.Errorf("bump sync token: %w", err)
	}
	var token int64
	err = d.db.QueryRowContext(ctx, `SELECT sync_token FROM address_books WHERE id = ?`, id).Scan(&token)
	if err != nil {
		return 0, mapNoRows(err)
	}
	return token, nil
}

// --- helpers ---

func scanAddressBook(s rowScanner) (*store.AddressBook, error) {
	ab := &store.AddressBook{}
	var createdAt, updatedAt string
	var description, color *string
	if err := s.Scan(
		&ab.ID, &ab.UserID, &ab.Name, &ab.DisplayName,
		&description, &color,
		&ab.SyncToken, &ab.CTag,
		&createdAt, &updatedAt,
	); err != nil {
		return nil, err
	}
	if description != nil {
		ab.Description = *description
	}
	if color != nil {
		ab.Color = *color
	}
	ab.CreatedAt = mustParseRFC3339(createdAt)
	ab.UpdatedAt = mustParseRFC3339(updatedAt)
	return ab, nil
}
