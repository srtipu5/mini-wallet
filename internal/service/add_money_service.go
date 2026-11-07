package service

import (
	"context"
	"strings"

	"github.com/srtipu5/mini-wallet/internal/domain"
	"github.com/srtipu5/mini-wallet/internal/dto"
)

// AddMoney credits a wallet after a payment-gateway confirmation.
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

	tx, err := s.store.BeginTx(ctx)
	if err != nil {
		return dto.AddMoneyResult{}, false, err
	}
	defer tx.Rollback(ctx)

	result, replayed, err := withIdempotency(ctx, tx.Repos(), scopeAddMoney, idempotencyKey, in,
		func() (dto.AddMoneyResult, error) {
			return creditWallet(ctx, tx.Repos(), in)
		})
	if err != nil {
		return dto.AddMoneyResult{}, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return dto.AddMoneyResult{}, false, err
	}
	return result, replayed, nil
}

// creditWallet must run inside a transaction.
func creditWallet(ctx context.Context, repos Repositories, in dto.AddMoneyInput) (dto.AddMoneyResult, error) {
	balanceAfter, err := repos.Wallets.AdjustBalance(ctx, in.WalletID, in.Amount)
	if err != nil {
		return dto.AddMoneyResult{}, err
	}

	// A reused gateway_ref fails on the unique index and rolls back the credit.
	entry := domain.Transaction{
		Type:         domain.TxAddMoney,
		Amount:       in.Amount,
		BalanceAfter: balanceAfter,
		ReferenceID:  in.GatewayRef,
	}
	if err := repos.Transactions.Insert(ctx, in.WalletID, &entry); err != nil {
		return dto.AddMoneyResult{}, err
	}

	return dto.AddMoneyResult{
		TransactionID: entry.ID,
		WalletID:      in.WalletID,
		Amount:        in.Amount,
		GatewayRef:    in.GatewayRef,
		BalanceAfter:  balanceAfter,
		CreatedAt:     entry.CreatedAt,
	}, nil
}
