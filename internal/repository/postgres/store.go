package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/srtipu5/mini-wallet/internal/service"
)

type Store struct {
	pool *pgxpool.Pool
}

var _ service.Store = (*Store)(nil)

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Repos() service.Repositories {
	return newRepositories(s.pool)
}

func (s *Store) BeginTx(ctx context.Context) (service.Tx, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &dbTx{tx: tx, repos: newRepositories(tx)}, nil
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

type dbTx struct {
	tx    pgx.Tx
	repos service.Repositories
}

func (t *dbTx) Repos() service.Repositories { return t.repos }

func (t *dbTx) Commit(ctx context.Context) error { return t.tx.Commit(ctx) }

// Rollback ignores an already-closed transaction so it is safe to defer.
func (t *dbTx) Rollback(ctx context.Context) error {
	if err := t.tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return err
	}
	return nil
}

func newRepositories(db DBTX) service.Repositories {
	return service.Repositories{
		Wallets:      &walletRepository{db: db},
		Transactions: &transactionRepository{db: db},
		Transfers:    &transferRepository{db: db},
		Idempotency:  &idempotencyRepository{db: db},
	}
}
