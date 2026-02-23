package postgres

import "context"

// CreateSchema applies all pending migrations. Delegates to Migrate for migration-based schema management.
func (s *PGStore) CreateSchema(ctx context.Context) error {
	return s.Migrate(ctx)
}

// DropSchema drops all ai tables and the migrations tracking table.
func (s *PGStore) DropSchema(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `
		DROP TABLE IF EXISTS ai_migrations CASCADE;
		DROP TABLE IF EXISTS ai_messages CASCADE;
		DROP TABLE IF EXISTS ai_sessions CASCADE;
	`)
	return err
}
