package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/sdobberstein/contacthub/internal/store"
)

// CreateAppPassword inserts a new app password record.
func (d *DB) CreateAppPassword(ctx context.Context, ap *store.AppPassword) error {
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO app_passwords (id, user_id, name, token_hash, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		ap.ID, ap.UserID, ap.Name, ap.TokenHash, ap.CreatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

// GetAppPasswordByTokenHash returns the app password matching the given SHA-256 token hash,
// or store.ErrNotFound.
func (d *DB) GetAppPasswordByTokenHash(ctx context.Context, hash string) (*store.AppPassword, error) {
	row := d.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, token_hash, last_used_at, created_at
		 FROM app_passwords WHERE token_hash = ?`, hash)
	ap, err := scanAppPassword(row)
	return ap, mapNoRows(err)
}

// ListAppPasswords returns all app passwords for the given user, ordered by creation time.
func (d *DB) ListAppPasswords(ctx context.Context, userID string) ([]*store.AppPassword, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT id, user_id, name, token_hash, last_used_at, created_at
		 FROM app_passwords WHERE user_id = ? ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // read-only rows, close error not actionable

	var aps []*store.AppPassword
	for rows.Next() {
		ap, err := scanAppPassword(rows)
		if err != nil {
			return nil, err
		}
		aps = append(aps, ap)
	}
	return aps, rows.Err()
}

// UpdateAppPasswordLastUsed sets the last_used_at timestamp for the given app password.
func (d *DB) UpdateAppPasswordLastUsed(ctx context.Context, id string, t time.Time) error {
	_, err := d.db.ExecContext(ctx,
		`UPDATE app_passwords SET last_used_at=? WHERE id=?`,
		t.UTC().Format(time.RFC3339), id)
	return err
}

// DeleteAppPassword removes the app password with the given ID, or store.ErrNotFound.
func (d *DB) DeleteAppPassword(ctx context.Context, id string) error {
	res, err := d.db.ExecContext(ctx, `DELETE FROM app_passwords WHERE id=?`, id)
	if err != nil {
		return err
	}
	return requireOneRow(res)
}

// --- helpers ---

func scanAppPassword(s rowScanner) (*store.AppPassword, error) {
	ap := &store.AppPassword{}
	var lastUsedAt sql.NullString
	var createdAt string
	if err := s.Scan(&ap.ID, &ap.UserID, &ap.Name, &ap.TokenHash, &lastUsedAt, &createdAt); err != nil {
		return nil, err
	}
	ap.CreatedAt = mustParseRFC3339(createdAt)
	if lastUsedAt.Valid {
		t := mustParseRFC3339(lastUsedAt.String)
		ap.LastUsedAt = &t
	}
	return ap, nil
}
