package sqlite

import (
	"context"
	"time"

	"github.com/sdobberstein/contacthub/internal/store"
)

// AppendAudit records a security-relevant event in the audit log.
func (d *DB) AppendAudit(ctx context.Context, entry *store.AuditEntry) error {
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO audit_log (user_id, action, resource, detail, ip_address, auth_method, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		nullableString(entry.UserID), entry.Action, nullableString(entry.Resource),
		nullableString(entry.Detail), nullableString(entry.IPAddress),
		nullableString(entry.AuthMethod), entry.CreatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

// PurgeAuditLog deletes audit entries older than olderThan.
func (d *DB) PurgeAuditLog(ctx context.Context, olderThan time.Time) error {
	_, err := d.db.ExecContext(ctx,
		`DELETE FROM audit_log WHERE created_at < ?`, olderThan.UTC().Format(time.RFC3339))
	return err
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
