package postgres

import "context"

// CreateSchema applies all pending migrations. Delegates to Migrate for migration-based schema management.
func (s *PGStore) CreateSchema(ctx context.Context) error {
	return s.Migrate(ctx)
}
