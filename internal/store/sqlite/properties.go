package sqlite

import (
	"context"

	"github.com/sdobberstein/contacthub/internal/store"
)

// SetProperty inserts or replaces a dead WebDAV property for a resource.
func (d *DB) SetProperty(ctx context.Context, p *store.Property) error {
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO properties (resource, namespace, name, value)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(resource, namespace, name) DO UPDATE SET value = excluded.value`,
		p.Resource, p.Namespace, p.Name, p.Value,
	)
	return err
}

// GetProperty returns the property identified by resource, namespace, and name.
func (d *DB) GetProperty(ctx context.Context, resource, namespace, name string) (*store.Property, error) {
	row := d.db.QueryRowContext(ctx,
		`SELECT resource, namespace, name, value FROM properties
		 WHERE resource = ? AND namespace = ? AND name = ?`,
		resource, namespace, name)
	p := &store.Property{}
	var value *string
	if err := row.Scan(&p.Resource, &p.Namespace, &p.Name, &value); err != nil {
		return nil, mapNoRows(err)
	}
	if value != nil {
		p.Value = *value
	}
	return p, nil
}

// ListProperties returns all dead properties stored for a resource.
func (d *DB) ListProperties(ctx context.Context, resource string) ([]*store.Property, error) {
	rows, err := d.db.QueryContext(ctx,
		`SELECT resource, namespace, name, value FROM properties
		 WHERE resource = ? ORDER BY namespace, name`,
		resource)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // read-only rows, close error not actionable

	var props []*store.Property
	for rows.Next() {
		p := &store.Property{}
		var value *string
		if err := rows.Scan(&p.Resource, &p.Namespace, &p.Name, &value); err != nil {
			return nil, err
		}
		if value != nil {
			p.Value = *value
		}
		props = append(props, p)
	}
	return props, rows.Err()
}

// DeleteProperty removes a single dead property.
func (d *DB) DeleteProperty(ctx context.Context, resource, namespace, name string) error {
	_, err := d.db.ExecContext(ctx,
		`DELETE FROM properties WHERE resource = ? AND namespace = ? AND name = ?`,
		resource, namespace, name)
	return err
}

// DeleteResourceProperties removes all dead properties for a resource.
func (d *DB) DeleteResourceProperties(ctx context.Context, resource string) error {
	_, err := d.db.ExecContext(ctx, `DELETE FROM properties WHERE resource = ?`, resource)
	return err
}
