package service

import (
	"context"
	"strings"

	"github.com/srtipu5/mini-wallet/internal/domain"
	"github.com/srtipu5/mini-wallet/internal/dto"
)

// AddMoney credits a wallet after a (simulated) payment-gateway confirmation.
func (s *Service) AddMoney(ctx context.Context, idempotencyKey string, in dto.AddMoneyInput) (dto.AddMoneyResult, bool, error) {
	in.WalletID = strings.ToLower(in.WalletID)

	if !domain.IsValidWalletID(in.WalletID) {
		return dto.AddMoneyResult{}, false, domain.ErrWalletNotFound
	}
	if err := domain.ValidateAmount(in.Amount); err != nil {
		return dto.AddMoneyResult{}, false, err
	}
	if err := domain.ValidateText("gateway_ref", in.GatewayRef, domain.MaxGatewayRefLength); err != nil {
		return dto.AddMoneyResult{}, false, err
	}

	return runIdempotent(ctx, s.store, scopeAddMoney, idempotencyKey, in,
		func(r Repositories) (dto.AddMoneyResult, error) {
			// UPDATE locks the wallet row and returns the new balance.
			balance, err := r.Wallets.ChangeBalance(ctx, in.WalletID, in.Amount)
			if err != nil {
				return dto.AddMoneyResult{}, err
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
			if err := r.Transactions.Insert(ctx, in.WalletID, &entry); err != nil {
				return dto.AddMoneyResult{}, err
			}

			return dto.AddMoneyResult{
				TransactionID: entry.ID,
				WalletID:      in.WalletID,
				Amount:        in.Amount,
				GatewayRef:    in.GatewayRef,
				BalanceAfter:  balance,
				CreatedAt:     entry.CreatedAt,
			}, nil
		})
}
