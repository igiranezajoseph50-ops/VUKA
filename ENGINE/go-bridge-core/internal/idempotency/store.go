// Package idempotency provides a database-backed idempotency key store.
//
// Every transfer request must carry a client-generated Idempotency-Key
// (UUIDv4 per spec §3.2). The key is claimed inside the same database
// transaction as the ledger mutation, so a duplicate charge is impossible
// even under concurrent retries:
//
//   - transactions.idempotency_key is UNIQUE, so a second header insert with
//     the same key fails with a unique violation;
//   - idempotency_keys.key is the PRIMARY KEY, so a second claim fails with
//     ErrKeyExists.
//
// Both guards live inside the transfer's transaction; exactly one request
// wins and the losers replay the winner's transfer instead of charging twice.
package idempotency

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrKeyExists is returned when an idempotency key has already been claimed
// by a prior transfer attempt.
var ErrKeyExists = errors.New("idempotency: key already exists")

// Store persists idempotency key claims in PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore creates an idempotency store around the shared pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Claim atomically records that key maps to transactionID. It MUST be called
// inside the transfer's database transaction (tx), making the claim atomic
// with the ledger mutation. Returns ErrKeyExists if the key was already
// claimed; the caller should roll back and replay the original transfer.
func (s *Store) Claim(ctx context.Context, tx pgx.Tx, key, transactionID string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO idempotency_keys (key, transaction_id)
		VALUES ($1, $2)`,
		key, transactionID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrKeyExists
		}
		return fmt.Errorf("idempotency: claim %q: %w", key, err)
	}
	return nil
}

// Lookup returns the transaction id previously associated with key, or
// (id, false) when the key is unknown.
func (s *Store) Lookup(ctx context.Context, key string) (string, bool, error) {
	var transactionID string
	err := s.pool.QueryRow(ctx, `
		SELECT transaction_id FROM idempotency_keys WHERE key = $1`, key,
	).Scan(&transactionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("idempotency: lookup %q: %w", key, err)
	}
	return transactionID, true, nil
}
