package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/srtipu5/mini-wallet/internal/domain"
)

// InsertTransaction writes one ledger entry and fills in its ID and CreatedAt.
func InsertTransaction(ctx context.Context, tx pgx.Tx, walletID string, t *domain.Transaction) error {
	err := tx.QueryRow(ctx,
		`INSERT INTO transactions (wallet_id, type, amount, balance_after, counterparty_wallet_id, reference_id)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, created_at`,
		walletID, t.Type, t.Amount, t.BalanceAfter, t.CounterpartyWalletID, t.ReferenceID,
	).Scan(&t.ID, &t.CreatedAt)
	t.CreatedAt = t.CreatedAt.UTC()

	if pgErrorIs(err, pgUniqueViolation, "transactions_gateway_ref_key") {
		return domain.ErrDuplicateGatewayRef
	}
	return err
}

// HistoryFilter selects a page of one wallet's history.
// From is inclusive, To is exclusive: created_at in [From, To).
type HistoryFilter struct {
	WalletID string
	Type     string     // "" = all types
	From     *time.Time // nil = no lower bound
	To       *time.Time // nil = no upper bound
	Page     int        // 1-based
	PageSize int
}

// ListTransactions returns one page (newest first) and the total match count.
func ListTransactions(ctx context.Context, db DBTX, f HistoryFilter) ([]domain.Transaction, int, error) {
	// NULL parameters switch a condition off, so one query covers every filter combination.
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
	if err := db.QueryRow(ctx, `SELECT count(*) FROM transactions`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Sorting by id (not created_at) gives the exact order in which the
	// balance changed, because ids are assigned while the wallet is locked.
	rows, err := db.Query(ctx,
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
