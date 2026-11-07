package service

import (
	"context"

	"github.com/srtipu5/mini-wallet/internal/domain"
)

type WalletRepository interface {
	Create(ctx context.Context, userID, phone string) (domain.Wallet, error)
	Get(ctx context.Context, id string) (domain.Wallet, error)
	List(ctx context.Context, page, pageSize int) (wallets []domain.Wallet, total int, err error)
	// LockBalances takes row locks (FOR UPDATE) in id order; unknown ids are omitted.
	LockBalances(ctx context.Context, ids ...string) (map[string]int64, error)
	// AdjustBalance adds delta (may be negative) and returns the new balance.
	AdjustBalance(ctx context.Context, id string, delta int64) (int64, error)
}

type TransactionRepository interface {
	Insert(ctx context.Context, walletID string, t *domain.Transaction) error
	List(ctx context.Context, f domain.TransactionFilter) (items []domain.Transaction, total int, err error)
}

type TransferRepository interface {
	Insert(ctx context.Context, fromID, toID string, amount int64, note *string) (domain.Transfer, error)
}

type IdempotencyRepository interface {
	// Claim returns nil if the key is new, or the existing record if already used.
	Claim(ctx context.Context, scope, key, requestHash string) (*domain.IdempotencyRecord, error)
	SaveResponse(ctx context.Context, scope, key string, body []byte) error
}

type Repositories struct {
	Wallets      WalletRepository
	Transactions TransactionRepository
	Transfers    TransferRepository
	Idempotency  IdempotencyRepository
}

// Tx is a database transaction. Rollback is a no-op after Commit.
type Tx interface {
	Repos() Repositories
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type Store interface {
	Repos() Repositories
	BeginTx(ctx context.Context) (Tx, error)
	Ping(ctx context.Context) error
}
