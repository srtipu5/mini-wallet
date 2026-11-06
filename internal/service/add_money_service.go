package service

import (
	"context"
	"strings"

	"github.com/srtipu5/mini-wallet/internal/domain"
	"github.com/srtipu5/mini-wallet/internal/dto"
)

// AddMoney credits a wallet after a (simulated) payment-gateway confirmation.
//
// Validation runs first, outside the database. The credit runs inside ONE
// database transaction opened by runIdempotent (see addMoneyInTx).
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
		func(tx Repositories) (dto.AddMoneyResult, error) {
			return addMoneyInTx(ctx, tx, in)
		})
}

// addMoneyInTx credits the wallet. Every call on tx runs inside the same
// database transaction, so an error in any step rolls back all of them.
func addMoneyInTx(ctx context.Context, tx Repositories, in dto.AddMoneyInput) (dto.AddMoneyResult, error) {
	// Step 1: credit the wallet.
	// UPDATE wallets SET balance = balance + amount ... RETURNING balance
	// The UPDATE also locks the wallet row until COMMIT or ROLLBACK.
	balanceAfter, err := tx.Wallets.ChangeBalance(ctx, in.WalletID, in.Amount)
	if err != nil {
		return dto.AddMoneyResult{}, err
	}

	// Step 2: write the ledger entry.
	// INSERT INTO transactions (ADD_MONEY)
	// The unique index on gateway_ref rejects a second credit for the same
	// payment, even under a different Idempotency-Key. The error then rolls
	// back step 1 as well.
	entry := domain.Transaction{
		Type:         domain.TxAddMoney,
		Amount:       in.Amount,
		BalanceAfter: balanceAfter,
		ReferenceID:  in.GatewayRef,
	}
	if err := tx.Transactions.Insert(ctx, in.WalletID, &entry); err != nil {
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
