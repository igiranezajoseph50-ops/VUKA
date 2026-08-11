package ledger

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// schemaSQL is the idempotent double-entry ledger schema, embedded into the
// binary so the engine self-heals on ANY PostgreSQL — including an empty
// managed database like Render's (which, unlike the local docker-compose,
// never runs deploy/init.sql automatically).
//
//go:embed schema.sql
var schemaSQL string

// EnsureSchema creates the ledger schema if it does not exist. It is safe to
// call on every boot against an empty or an already-provisioned database:
// every statement in schema.sql is guarded (CREATE ... IF NOT EXISTS, DO $$
// blocks for enum types, CREATE OR REPLACE VIEW). Returns the first error,
// which means startup should abort fast — a ledger without its schema cannot
// move money correctly.
func (s *Service) EnsureSchema(ctx context.Context) error {
	if s.pool == nil {
		return fmt.Errorf("ledger: EnsureSchema called with nil pool")
	}
	// pgx Exec runs a multi-statement script (including DO $$ ... $$ blocks)
	// when no parameters are bound.
	if _, err := s.pool.Exec(ctx, schemaSQL); err != nil {
		return fmt.Errorf("ledger: EnsureSchema: %w", err)
	}
	return nil
}

// ensureSchemaForPool runs the embedded schema against any live pool. Kept
// separate so tests / provisioning code can bootstrap before building a
// full Service.
func ensureSchemaForPool(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		return fmt.Errorf("ledger: ensure schema: %w", err)
	}
	return nil
}