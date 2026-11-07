package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/srtipu5/mini-wallet/internal/domain"
	"github.com/srtipu5/mini-wallet/internal/service"
)

type idempotencyRepository struct {
	db DBTX
}

var _ service.IdempotencyRepository = (*idempotencyRepository)(nil)

// Claim must run inside a transaction. If another transaction holds the same
// key, the INSERT waits for it to commit or roll back.
func (r *idempotencyRepository) Claim(ctx context.Context, scope, key, requestHash string) (*domain.IdempotencyRecord, error) {
	tag, err := r.db.Exec(ctx,
		`INSERT INTO idempotency_keys (scope, key, request_hash) VALUES ($1, $2, $3)
		 ON CONFLICT (scope, key) DO NOTHING`,
		scope, key, requestHash)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 1 {
		return nil, nil
	}

	var rec domain.IdempotencyRecord
	err = r.db.QueryRow(ctx,
		`SELECT request_hash, response_body FROM idempotency_keys WHERE scope = $1 AND key = $2`,
		scope, key).Scan(&rec.RequestHash, &rec.ResponseBody)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("idempotency key disappeared during claim")
	}
	return &rec, err
}

func (r *idempotencyRepository) SaveResponse(ctx context.Context, scope, key string, body []byte) error {
	_, err := r.db.Exec(ctx,
		`UPDATE idempotency_keys SET response_body = $3 WHERE scope = $1 AND key = $2`,
		scope, key, body)
	return err
}
