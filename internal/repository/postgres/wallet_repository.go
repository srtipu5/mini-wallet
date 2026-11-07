package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/srtipu5/mini-wallet/internal/domain"
	"github.com/srtipu5/mini-wallet/internal/service"
)

type walletRepository struct {
	db DBTX
}

var _ service.WalletRepository = (*walletRepository)(nil)

const walletColumns = `id::text, user_id, phone, currency, balance, created_at`

func scanWallet(row pgx.Row) (domain.Wallet, error) {
	var w domain.Wallet
	err := row.Scan(&w.ID, &w.UserID, &w.Phone, &w.Currency, &w.Balance, &w.CreatedAt)
	w.CreatedAt = w.CreatedAt.UTC()
	return w, err
}

func (r *walletRepository) Create(ctx context.Context, userID, phone string) (domain.Wallet, error) {
	w, err := scanWallet(r.db.QueryRow(ctx,
		`INSERT INTO wallets (user_id, phone) VALUES ($1, $2) RETURNING `+walletColumns,
		userID, phone))

	switch {
	case isPgError(err, pgUniqueViolation, "wallets_user_id_key"):
		return w, domain.NewError(domain.CodeDuplicateWallet, "This user already has a wallet")
	case isPgError(err, pgUniqueViolation, "wallets_phone_key"):
		return w, domain.NewError(domain.CodeDuplicateWallet, "A wallet with this phone already exists")
	}
	return w, err
}

func (r *walletRepository) Get(ctx context.Context, id string) (domain.Wallet, error) {
	w, err := scanWallet(r.db.QueryRow(ctx,
		`SELECT `+walletColumns+` FROM wallets WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return w, domain.ErrWalletNotFound
	}
	return w, err
}

func (r *walletRepository) List(ctx context.Context, page, pageSize int) ([]domain.Wallet, int, error) {
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM wallets`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx,
		`SELECT `+walletColumns+` FROM wallets
		 ORDER BY created_at DESC, id DESC
		 LIMIT $1 OFFSET $2`,
		pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	wallets, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Wallet, error) {
		return scanWallet(row)
	})
	return wallets, total, err
}

// LockBalances locks rows in id order so concurrent transfers cannot deadlock.
func (r *walletRepository) LockBalances(ctx context.Context, ids ...string) (map[string]int64, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id::text, balance FROM wallets
		 WHERE id = ANY($1::uuid[])
		 ORDER BY id
		 FOR UPDATE`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	balances := make(map[string]int64, len(ids))
	for rows.Next() {
		var id string
		var balance int64
		if err := rows.Scan(&id, &balance); err != nil {
			return nil, err
		}
		balances[id] = balance
	}
	return balances, rows.Err()
}

func (r *walletRepository) AdjustBalance(ctx context.Context, id string, delta int64) (int64, error) {
	var balance int64
	err := r.db.QueryRow(ctx,
		`UPDATE wallets SET balance = balance + $2, updated_at = now() WHERE id = $1 RETURNING balance`,
		id, delta).Scan(&balance)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return 0, domain.ErrWalletNotFound
	case isPgError(err, pgCheckViolation, "wallets_balance_non_negative"):
		// Safety net: the service checks the balance first.
		return 0, domain.ErrInsufficientBalance
	}
	return balance, err
}
