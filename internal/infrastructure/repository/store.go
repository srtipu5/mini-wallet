package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srtipu5/mini-wallet/internal/application"
)

// Store implements application.Store on a pgx connection pool.
type Store struct {
	pool *pgxpool.Pool
}

var _ application.Store = (*Store)(nil)

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Repos() application.Repositories {
	return newRepositories(s.pool)
}

// WithinTx commits if fn returns nil and rolls back otherwise.
func (s *Store) WithinTx(ctx context.Context, fn func(r application.Repositories) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(newRepositories(tx))
	})
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// newRepositories builds every repository on the same connection or transaction.
func newRepositories(db DBTX) application.Repositories {
	return application.Repositories{
		Wallets:      &walletRepository{db: db},
		Transactions: &transactionRepository{db: db},
		Transfers:    &transferRepository{db: db},
		Idempotency:  &idempotencyRepository{db: db},
	}
}
