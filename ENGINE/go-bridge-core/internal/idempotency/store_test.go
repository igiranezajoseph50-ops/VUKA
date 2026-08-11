package idempotency_test

import (
	"context"
	"errors"
	"testing"

	"vuka/go-bridge-core/internal/idempotency"
	"vuka/go-bridge-core/internal/testutil"
)

func TestClaimAndLookup(t *testing.T) {
	pool := testutil.TestPool(t)
	s := idempotency.NewStore(pool)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	defer tx.Rollback(ctx)

	// Fresh key: claim succeeds, lookup returns the mapped tx id.
	if err := s.Claim(ctx, tx, "key-1", "11111111-1111-1111-1111-111111111111"); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	id, ok, err := s.Lookup(ctx, "key-1")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if !ok || id != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("Lookup = (%q, %v), want (11111111-1111-1111-1111-111111111111, true)", id, ok)
	}
}

func TestClaim_DuplicateKeyRejected(t *testing.T) {
	pool := testutil.TestPool(t)
	s := idempotency.NewStore(pool)
	ctx := context.Background()

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := s.Claim(ctx, tx, "key-dup", "22222222-2222-2222-2222-222222222222"); err != nil {
		t.Fatalf("first Claim: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Second claim in a fresh tx must fail with ErrKeyExists.
	tx2, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin2: %v", err)
	}
	defer tx2.Rollback(ctx)
	if err := s.Claim(ctx, tx2, "key-dup", "22222222-2222-2222-2222-222222222222"); !errors.Is(err, idempotency.ErrKeyExists) {
		t.Fatalf("expected ErrKeyExists, got %v", err)
	}
}

func TestLookup_MissingKey(t *testing.T) {
	pool := testutil.TestPool(t)
	s := idempotency.NewStore(pool)

	id, ok, err := s.Lookup(context.Background(), "does-not-exist")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if ok || id != "" {
		t.Errorf("Lookup = (%q, %v), want (\"\", false)", id, ok)
	}
}
