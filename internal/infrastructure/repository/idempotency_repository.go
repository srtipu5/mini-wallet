package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/srtipu5/mini-wallet/internal/application"
	"github.com/srtipu5/mini-wallet/internal/domain"
)

type idempotencyRepository struct {
	db DBTX
}

var _ application.IdempotencyRepository = (*idempotencyRepository)(nil)

// Claim tries to reserve (scope, key). It must run inside a transaction.
//
//   - Key is new: the row is inserted, and nil is returned. The caller now
//     owns the key and must do the work in the same transaction.
//   - Key already committed: the stored record is returned.
//   - Another transaction holds the key but has not committed yet: the INSERT
//     waits on the primary-key index until that transaction finishes. If it
//     commits, we see its row and return it. If it rolls back, our INSERT
//     succeeds and we become the owner.
//
// This is what makes concurrent requests with the same key execute exactly once.
func (r *idempotencyRepository) Claim(ctx context.Context, scope, key, requestHash string) (*domain.IdempotencyRecord, error) {
	tag, err := r.db.Exec(ctx,
		`INSERT INTO idempotency_keys (scope, key, request_hash) VALUES ($1, $2, $3)
		 ON CONFLICT (scope, key) DO NOTHING`,
		scope, key, requestHash)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 1 {
		return nil, nil // claimed
	}

	var rec domain.IdempotencyRecord
	err = r.db.QueryRow(ctx,
		`SELECT request_hash, response_body FROM idempotency_keys WHERE scope = $1 AND key = $2`,
		scope, key).Scan(&rec.RequestHash, &rec.ResponseBody)
	if errors.Is(err, pgx.ErrNoRows) {
		// Not reachable in practice: a conflicting row exists and rows are never deleted.
		return nil, errors.New("idempotency key vanished during claim")
	}
	return &rec, err
}

// SaveResponse stores the success response for a claimed key.
func (r *idempotencyRepository) SaveResponse(ctx context.Context, scope, key string, body []byte) error {
	_, err := r.db.Exec(ctx,
		`UPDATE idempotency_keys SET response_body = $3 WHERE scope = $1 AND key = $2`,
		scope, key, body)
	return err
}
