package service

import (
	"context"
	"strings"

	"github.com/srtipu5/mini-wallet/internal/domain"
	"github.com/srtipu5/mini-wallet/internal/dto"
)

func (s *Service) Transfer(ctx context.Context, idempotencyKey string, in dto.TransferInput) (domain.Transfer, bool, error) {
	in.FromWalletID = strings.ToLower(in.FromWalletID)
	in.ToWalletID = strings.ToLower(in.ToWalletID)
	if in.Note != nil && strings.TrimSpace(*in.Note) == "" {
		in.Note = nil
	}

	if err := validateTransfer(in); err != nil {
		return domain.Transfer{}, false, err
	}

	tx, err := s.store.BeginTx(ctx)
	if err != nil {
		return domain.Transfer{}, false, err
	}
	defer tx.Rollback(ctx)
	repos := tx.Repos()

	transfer, replayed, err := claimIdempotency[domain.Transfer](ctx, repos, scopeTransfer, idempotencyKey, in)
	if err != nil {
		return domain.Transfer{}, false, err
	}
	if !replayed {
		transfer, err = executeTransfer(ctx, repos, in)
		if err != nil {
			return domain.Transfer{}, false, err
		}
		if err := saveIdempotencyResponse(ctx, repos, scopeTransfer, idempotencyKey, transfer); err != nil {
			return domain.Transfer{}, false, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Transfer{}, false, err
	}
	return transfer, replayed, nil
}

func validateTransfer(in dto.TransferInput) error {
	if in.FromWalletID == "" || in.ToWalletID == "" {
		return domain.Validation("The from_wallet_id and to_wallet_id fields are required")
	}
	if in.FromWalletID == in.ToWalletID {
		return domain.ErrSameWalletTransfer
	}
	if !domain.IsValidWalletID(in.FromWalletID) {
		return domain.NewError(domain.CodeWalletNotFound, "Sender wallet not found")
	}
	if !domain.IsValidWalletID(in.ToWalletID) {
		return domain.NewError(domain.CodeWalletNotFound, "Receiver wallet not found")
	}
	if err := domain.ValidateAmount(in.Amount); err != nil {
		return err
	}
	if in.Note != nil && len([]rune(*in.Note)) > domain.MaxNoteLength {
		return domain.Validation("Note must be at most 255 characters")
	}
	return nil
}

// executeTransfer must run inside a transaction.
func executeTransfer(ctx context.Context, repos Repositories, in dto.TransferInput) (domain.Transfer, error) {
	// Locking both wallets in id order prevents deadlocks between opposite transfers.
	balances, err := repos.Wallets.LockBalances(ctx, in.FromWalletID, in.ToWalletID)
	if err != nil {
		return domain.Transfer{}, err
	}
	senderBalance, ok := balances[in.FromWalletID]
	if !ok {
		return domain.Transfer{}, domain.NewError(domain.CodeWalletNotFound, "Sender wallet not found")
	}
	if _, ok := balances[in.ToWalletID]; !ok {
		return domain.Transfer{}, domain.NewError(domain.CodeWalletNotFound, "Receiver wallet not found")
	}
	if senderBalance < in.Amount {
		return domain.Transfer{}, domain.ErrInsufficientBalance
	}

	senderBalanceAfter, err := repos.Wallets.AdjustBalance(ctx, in.FromWalletID, -in.Amount)
	if err != nil {
		return domain.Transfer{}, err
	}
	receiverBalanceAfter, err := repos.Wallets.AdjustBalance(ctx, in.ToWalletID, in.Amount)
	if err != nil {
		return domain.Transfer{}, err
	}

	transfer, err := repos.Transfers.Insert(ctx, in.FromWalletID, in.ToWalletID, in.Amount, in.Note)
	if err != nil {
		return domain.Transfer{}, err
	}

	debit := domain.Transaction{
		Type:                 domain.TxTransferOut,
		Amount:               in.Amount,
		BalanceAfter:         senderBalanceAfter,
		CounterpartyWalletID: &in.ToWalletID,
		ReferenceID:          transfer.ID,
	}
	if err := repos.Transactions.Insert(ctx, in.FromWalletID, &debit); err != nil {
		return domain.Transfer{}, err
	}

	credit := domain.Transaction{
		Type:                 domain.TxTransferIn,
		Amount:               in.Amount,
		BalanceAfter:         receiverBalanceAfter,
		CounterpartyWalletID: &in.FromWalletID,
		ReferenceID:          transfer.ID,
	}
	if err := repos.Transactions.Insert(ctx, in.ToWalletID, &credit); err != nil {
		return domain.Transfer{}, err
	}

	return transfer, nil
}
