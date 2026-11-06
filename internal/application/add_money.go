package application

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/srtipu5/mini-wallet/internal/domain"
	"github.com/srtipu5/mini-wallet/internal/repository"
)

type AddMoneyInput struct {
	WalletID   string `json:"wallet_id"`
	Amount     int64  `json:"amount"`
	GatewayRef string `json:"gateway_ref"`
}

// AddMoney credits a wallet after a (simulated) payment-gateway confirmation.
func (s *Service) AddMoney(ctx context.Context, idempotencyKey string, in AddMoneyInput) (domain.AddMoneyResult, bool, error) {
	in.WalletID = strings.ToLower(in.WalletID)

	if !domain.IsValidWalletID(in.WalletID) {
		return domain.AddMoneyResult{}, false, domain.ErrWalletNotFound
	}
	if err := domain.ValidateAmount(in.Amount); err != nil {
		return domain.AddMoneyResult{}, false, err
	}
	if err := domain.ValidateText("gateway_ref", in.GatewayRef, domain.MaxGatewayRefLength); err != nil {
		return domain.AddMoneyResult{}, false, err
	}

	return runIdempotent(ctx, s.db, scopeAddMoney, idempotencyKey, in,
		func(tx pgx.Tx) (domain.AddMoneyResult, error) {
			// UPDATE locks the wallet row and returns the new balance.
			balance, err := repository.ChangeBalance(ctx, tx, in.WalletID, in.Amount)
			if err != nil {
				return domain.AddMoneyResult{}, err
			}

			// The unique index on gateway_ref rejects a second credit for the
			// same payment, even under a different Idempotency-Key. If it does,
			// the whole transaction (including the balance change) is rolled back.
			entry := domain.Transaction{
				Type:         domain.TxAddMoney,
				Amount:       in.Amount,
				BalanceAfter: balance,
				ReferenceID:  in.GatewayRef,
			}
			if err := repository.InsertTransaction(ctx, tx, in.WalletID, &entry); err != nil {
				return domain.AddMoneyResult{}, err
			}

			return domain.AddMoneyResult{
				TransactionID: entry.ID,
				WalletID:      in.WalletID,
				Amount:        in.Amount,
				GatewayRef:    in.GatewayRef,
				BalanceAfter:  balance,
				CreatedAt:     entry.CreatedAt,
			}, nil
		})
}
