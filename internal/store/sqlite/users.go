package sqlite

import (
	"context"
	"time"

	"github.com/sdobberstein/contacthub/internal/store"
)

// CreateUser inserts a new user record.
func (d *DB) CreateUser(ctx context.Context, u *store.User) error {
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO users (id, username, display_name, email, password_hash, is_admin, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Username, u.DisplayName, u.Email, u.PasswordHash,
		boolToInt(u.IsAdmin), u.CreatedAt.UTC().Format(time.RFC3339), u.UpdatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		if isUniqueConstraint(err) {
			return store.ErrConflict
		}
		return err
	}
	return nil
}

// GetUserByID returns the user with the given ID, or store.ErrNotFound.
func (d *DB) GetUserByID(ctx context.Context, id string) (*store.User, error) {
	row := d.db.QueryRowContext(ctx,
		`SELECT id, username, display_name, email, password_hash, is_admin, created_at, updated_at
		 FROM users WHERE id = ?`, id)
	u, err := scanUser(row)
	return u, mapNoRows(err)
}

// GetUserByUsername returns the user with the given username, or store.ErrNotFound.
func (d *DB) GetUserByUsername(ctx context.Context, username string) (*store.User, error) {
	row := d.db.QueryRowContext(ctx,
		`SELECT id, username, display_name, email, password_hash, is_admin, created_at, updated_at
		 FROM users WHERE username = ?`, username)
	u, err := scanUser(row)
	return u, mapNoRows(err)
}

// UpdateUser updates all mutable fields of an existing user.
func (d *DB) UpdateUser(ctx context.Context, u *store.User) error {
	res, err := d.db.ExecContext(ctx,
		`UPDATE users SET username=?, display_name=?, email=?, password_hash=?, is_admin=?, updated_at=?
		 WHERE id=?`,
		u.Username, u.DisplayName, u.Email, u.PasswordHash,
		boolToInt(u.IsAdmin), u.UpdatedAt.UTC().Format(time.RFC3339), u.ID,
	)
	if err != nil {
		if isUniqueConstraint(err) {
			return store.ErrConflict
		}
		return err
	}
	return requireOneRow(res)
}

// DeleteUser removes the user with the given ID, or store.ErrNotFound.
func (d *DB) DeleteUser(ctx context.Context, id string) error {
	res, err := d.db.ExecContext(ctx, `DELETE FROM users WHERE id=?`, id)
	if err != nil {
		return err
	}
	return requireOneRow(res)
}

// ListUsers returns all users ordered by username.
func (d *DB) ListUsers(ctx context.Context) ([]*store.User, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT id, username, display_name, email, password_hash, is_admin, created_at, updated_at
		 FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // read-only rows, close error not actionable

	var users []*store.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// CountUsers returns the total number of user accounts.
func (d *DB) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := d.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// --- helpers ---

func scanUser(s rowScanner) (*store.User, error) {
	u := &store.User{}
	var isAdmin int
	var createdAt, updatedAt string
	if err := s.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.PasswordHash,
		&isAdmin, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	u.IsAdmin = isAdmin != 0
	u.CreatedAt = mustParseRFC3339(createdAt)
	u.UpdatedAt = mustParseRFC3339(updatedAt)
	return u, nil
}
