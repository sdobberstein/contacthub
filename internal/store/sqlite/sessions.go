package sqlite

import (
	"context"
	"time"

	"github.com/sdobberstein/contacthub/internal/store"
)

// CreateSession inserts a new web session.
func (d *DB) CreateSession(ctx context.Context, s *store.Session) error {
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO sessions (id, user_id, ip_address, user_agent, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		s.ID, s.UserID, s.IPAddress, s.UserAgent,
		s.CreatedAt.UTC().Format(time.RFC3339), s.ExpiresAt.UTC().Format(time.RFC3339),
	)
	return err
}

// GetSession returns the session with the given ID, or store.ErrNotFound.
func (d *DB) GetSession(ctx context.Context, id string) (*store.Session, error) {
	row := d.db.QueryRowContext(ctx,
		`SELECT id, user_id, ip_address, user_agent, created_at, expires_at
		 FROM sessions WHERE id = ?`, id)

	s := &store.Session{}
	var createdAt, expiresAt string
	err := row.Scan(&s.ID, &s.UserID, &s.IPAddress, &s.UserAgent, &createdAt, &expiresAt)
	if err != nil {
		return nil, mapNoRows(err)
	}
	s.CreatedAt = mustParseRFC3339(createdAt)
	s.ExpiresAt = mustParseRFC3339(expiresAt)
	return s, nil
}

// DeleteSession removes the session with the given ID.
func (d *DB) DeleteSession(ctx context.Context, id string) error {
	_, err := d.db.ExecContext(ctx, `DELETE FROM sessions WHERE id=?`, id)
	return err
}

// DeleteUserSessions removes all sessions belonging to the given user.
func (d *DB) DeleteUserSessions(ctx context.Context, userID string) error {
	_, err := d.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id=?`, userID)
	return err
}

// PurgeExpiredSessions deletes all sessions whose expiry time is in the past.
func (d *DB) PurgeExpiredSessions(ctx context.Context) error {
	_, err := d.db.ExecContext(ctx,
		`DELETE FROM sessions WHERE expires_at < ?`, time.Now().UTC().Format(time.RFC3339))
	return err
}

