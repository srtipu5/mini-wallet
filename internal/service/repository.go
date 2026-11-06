package service

import (
	"context"

	"github.com/srtipu5/mini-wallet/internal/domain"
)

// The repository interfaces the service needs.
//
// They are defined here, next to the code that uses them, and implemented in
// internal/repository/postgres. This is why the service never imports a
// database driver, and why another database could be added without changing
// the business logic.

type WalletRepository interface {
	Create(ctx context.Context, userID, phone string) (domain.Wallet, error)
	Get(ctx context.Context, id string) (domain.Wallet, error)
	// List returns one page of wallets (newest first) and the total count.
	List(ctx context.Context, page, pageSize int) ([]domain.Wallet, int, error)
	// LockBalances locks the given wallets with SELECT ... FOR UPDATE, always
	// in wallet id order, and returns their balances by id. Wallets that do
	// not exist are missing from the map. Only meaningful inside
	// Store.WithinTx.
	LockBalances(ctx context.Context, ids ...string) (map[string]int64, error)
	// ChangeBalance adds delta (may be negative) and returns the new balance.
	ChangeBalance(ctx context.Context, id string, delta int64) (int64, error)
}

type TransactionRepository interface {
	// Insert writes one ledger entry and fills in its ID and CreatedAt.
	Insert(ctx context.Context, walletID string, t *domain.Transaction) error
	// List returns one page (newest first) and the total number of matches.
	List(ctx context.Context, f domain.TransactionFilter) ([]domain.Transaction, int, error)
}

type TransferRepository interface {
	Insert(ctx context.Context, fromID, toID string, amount int64, note *string) (domain.Transfer, error)
}

type IdempotencyRepository interface {
	// Claim reserves (scope, key). It returns nil if the key is new (the
	// caller now owns it), or the stored record if the key was already used.
	Claim(ctx context.Context, scope, key, requestHash string) (*domain.IdempotencyRecord, error)
	// SaveResponse stores the success response for a claimed key.
	SaveResponse(ctx context.Context, scope, key string, body []byte) error
}

// Repositories groups every repository. Depending on where it comes from,
// they all run on the connection pool or all inside one transaction.
type Repositories struct {
	Wallets      WalletRepository
	Transactions TransactionRepository
	Transfers    TransferRepository
	Idempotency  IdempotencyRepository
}

// Store gives access to the repositories and to database transactions.
type Store interface {
	// Repos returns repositories that run outside a transaction (simple reads/writes).
	Repos() Repositories
	// WithinTx runs fn inside one database transaction. If fn returns an
	// error, everything is rolled back; otherwise it is committed.
	WithinTx(ctx context.Context, fn func(tx Repositories) error) error
	// Ping checks that the database is reachable.
	Ping(ctx context.Context) error
}
