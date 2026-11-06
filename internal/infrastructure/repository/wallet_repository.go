package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/srtipu5/mini-wallet/internal/application"
	"github.com/srtipu5/mini-wallet/internal/domain"
)

type walletRepository struct {
	db DBTX
}

var _ application.WalletRepository = (*walletRepository)(nil)

const walletColumns = `id::text, user_id, phone, currency, balance, created_at`

func scanWallet(row pgx.Row) (domain.Wallet, error) {
	var w domain.Wallet
	err := row.Scan(&w.ID, &w.UserID, &w.Phone, &w.Currency, &w.Balance, &w.CreatedAt)
	w.CreatedAt = w.CreatedAt.UTC()
	return w, err
}

// Create inserts a wallet. The UNIQUE constraints on user_id and phone
// make duplicates impossible, even when two requests race.
func (r *walletRepository) Create(ctx context.Context, userID, phone string) (domain.Wallet, error) {
	w, err := scanWallet(r.db.QueryRow(ctx,
		`INSERT INTO wallets (user_id, phone) VALUES ($1, $2) RETURNING `+walletColumns,
		userID, phone))

	switch {
	case pgErrorIs(err, pgUniqueViolation, "wallets_user_id_key"):
		return w, domain.NewError(domain.CodeDuplicateWallet, "this user already has a wallet")
	case pgErrorIs(err, pgUniqueViolation, "wallets_phone_key"):
		return w, domain.NewError(domain.CodeDuplicateWallet, "a wallet with this phone already exists")
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

// Lock takes a row lock on the wallet (SELECT ... FOR UPDATE) and returns its
// current balance. Other transactions that want to change this wallet wait
// until our transaction commits or rolls back.
func (r *walletRepository) Lock(ctx context.Context, id string) (int64, error) {
	var balance int64
	err := r.db.QueryRow(ctx, `SELECT balance FROM wallets WHERE id = $1 FOR UPDATE`, id).Scan(&balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ErrWalletNotFound
	}
	return balance, err
}

// ChangeBalance adds delta (positive or negative) to the balance and returns
// the new balance. The UPDATE itself locks the row.
func (r *walletRepository) ChangeBalance(ctx context.Context, id string, delta int64) (int64, error) {
	var balance int64
	err := r.db.QueryRow(ctx,
		`UPDATE wallets SET balance = balance + $2, updated_at = now() WHERE id = $1 RETURNING balance`,
		id, delta).Scan(&balance)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return 0, domain.ErrWalletNotFound
	case pgErrorIs(err, pgCheckViolation, "wallets_balance_non_negative"):
		// Should never happen because the service checks first; the DB is the safety net.
		return 0, domain.ErrInsufficientBalance
	}
	return balance, err
}
