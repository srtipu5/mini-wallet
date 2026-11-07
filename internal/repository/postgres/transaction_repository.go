package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/srtipu5/mini-wallet/internal/domain"
	"github.com/srtipu5/mini-wallet/internal/service"
)

type transactionRepository struct {
	db DBTX
}

var _ service.TransactionRepository = (*transactionRepository)(nil)

// Insert sets t.ID and t.CreatedAt.
func (r *transactionRepository) Insert(ctx context.Context, walletID string, t *domain.Transaction) error {
	err := r.db.QueryRow(ctx,
		`INSERT INTO transactions (wallet_id, type, amount, balance_after, counterparty_wallet_id, reference_id)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at`,
		walletID, t.Type, t.Amount, t.BalanceAfter, t.CounterpartyWalletID, t.ReferenceID,
	).Scan(&t.ID, &t.CreatedAt)
	t.CreatedAt = t.CreatedAt.UTC()

	if isPgError(err, pgUniqueViolation, "transactions_gateway_ref_key") {
		return domain.ErrDuplicateGatewayRef
	}
	return err
}

func (r *transactionRepository) List(ctx context.Context, f domain.TransactionFilter) ([]domain.Transaction, int, error) {
	// A NULL parameter disables its condition.
	const where = `
		WHERE wallet_id = $1
		  AND ($2::text        IS NULL OR type = $2)
		  AND ($3::timestamptz IS NULL OR created_at >= $3)
		  AND ($4::timestamptz IS NULL OR created_at <  $4)`

	var txType *string
	if f.Type != "" {
		txType = &f.Type
	}
	args := []any{f.WalletID, txType, f.From, f.To}

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM transactions`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// id order matches the order of balance changes; created_at can tie.
	rows, err := r.db.Query(ctx,
		`SELECT id, type, amount, balance_after, counterparty_wallet_id::text, reference_id, created_at
		 FROM transactions`+where+`
		 ORDER BY id DESC
		 LIMIT $5 OFFSET $6`,
		append(args, f.PageSize, (f.Page-1)*f.PageSize)...)
	if err != nil {
		return nil, 0, err
	}

	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Transaction, error) {
		var t domain.Transaction
		err := row.Scan(&t.ID, &t.Type, &t.Amount, &t.BalanceAfter, &t.CounterpartyWalletID, &t.ReferenceID, &t.CreatedAt)
		t.CreatedAt = t.CreatedAt.UTC()
		return t, err
	})
	return list, total, err
}
