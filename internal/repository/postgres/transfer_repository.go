package postgres

import (
	"context"

	"github.com/srtipu5/mini-wallet/internal/domain"
	"github.com/srtipu5/mini-wallet/internal/service"
)

type transferRepository struct {
	db DBTX
}

var _ service.TransferRepository = (*transferRepository)(nil)

func (r *transferRepository) Insert(ctx context.Context, fromID, toID string, amount int64, note *string) (domain.Transfer, error) {
	t := domain.Transfer{
		FromWalletID: fromID,
		ToWalletID:   toID,
		Amount:       amount,
		Note:         note,
		Status:       domain.TransferCompleted,
	}
	err := r.db.QueryRow(ctx,
		`INSERT INTO transfers (from_wallet_id, to_wallet_id, amount, note, status)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id::text, created_at`,
		fromID, toID, amount, note, t.Status).Scan(&t.ID, &t.CreatedAt)
	t.CreatedAt = t.CreatedAt.UTC()
	return t, err
}
