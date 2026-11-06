package service

import (
	"context"
	"strings"

	"github.com/srtipu5/mini-wallet/internal/domain"
	"github.com/srtipu5/mini-wallet/internal/dto"
)

// Transfer moves money from one wallet to another.
//
// Validation runs first, outside the database. Everything else runs inside
// ONE database transaction (opened by runIdempotent):
//
//	BEGIN
//	  claim the Idempotency-Key            (runIdempotent)
//	  lock both wallets, check, move money (transferInTx)
//	  save the response for the key        (runIdempotent)
//	COMMIT    -- or ROLLBACK if any step returns an error
func (s *Service) Transfer(ctx context.Context, idempotencyKey string, in dto.TransferInput) (domain.Transfer, bool, error) {
	in.FromWalletID = strings.ToLower(in.FromWalletID)
	in.ToWalletID = strings.ToLower(in.ToWalletID)
	if in.Note != nil && strings.TrimSpace(*in.Note) == "" {
		in.Note = nil // treat "" the same as "no note"
	}

	if err := validateTransfer(in); err != nil {
		return domain.Transfer{}, false, err
	}

	return runIdempotent(ctx, s.store, scopeTransfer, idempotencyKey, in,
		func(tx Repositories) (domain.Transfer, error) {
			return transferInTx(ctx, tx, in)
		})
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

// transferInTx does the money movement. Every call on tx runs inside the same
// database transaction, so returning an error from any step rolls back all
// the steps before it: a debit is never saved without its credit.
func transferInTx(ctx context.Context, tx Repositories, in dto.TransferInput) (domain.Transfer, error) {
	// Step 1: lock both wallets.
	// SELECT ... WHERE id = ANY(...) ORDER BY id FOR UPDATE
	// Locking in wallet id order prevents deadlocks. Other transfers touching
	// these wallets now wait until we COMMIT or ROLLBACK.
	balances, err := tx.Wallets.LockBalances(ctx, in.FromWalletID, in.ToWalletID)
	if err != nil {
		return domain.Transfer{}, err
	}
	senderBalance, senderFound := balances[in.FromWalletID]
	if !senderFound {
		return domain.Transfer{}, domain.NewError(domain.CodeWalletNotFound, "Sender wallet not found")
	}
	if _, receiverFound := balances[in.ToWalletID]; !receiverFound {
		return domain.Transfer{}, domain.NewError(domain.CodeWalletNotFound, "Receiver wallet not found")
	}

	// Step 2: check the balance. Safe, because the lock stops anyone else
	// from changing it until we finish.
	if senderBalance < in.Amount {
		return domain.Transfer{}, domain.ErrInsufficientBalance
	}

	// Step 3: debit the sender and credit the receiver.
	// UPDATE wallets SET balance = balance - amount / + amount
	senderBalanceAfter, err := tx.Wallets.ChangeBalance(ctx, in.FromWalletID, -in.Amount)
	if err != nil {
		return domain.Transfer{}, err
	}
	receiverBalanceAfter, err := tx.Wallets.ChangeBalance(ctx, in.ToWalletID, in.Amount)
	if err != nil {
		return domain.Transfer{}, err
	}

	// Step 4: record the transfer.
	// INSERT INTO transfers
	transfer, err := tx.Transfers.Insert(ctx, in.FromWalletID, in.ToWalletID, in.Amount, in.Note)
	if err != nil {
		return domain.Transfer{}, err
	}

	// Step 5: one ledger entry per wallet, shown in each wallet's history.
	// INSERT INTO transactions (TRANSFER_OUT for the sender, TRANSFER_IN for the receiver)
	sent := domain.Transaction{
		Type:                 domain.TxTransferOut,
		Amount:               in.Amount,
		BalanceAfter:         senderBalanceAfter,
		CounterpartyWalletID: &in.ToWalletID,
		ReferenceID:          transfer.ID,
	}
	if err := tx.Transactions.Insert(ctx, in.FromWalletID, &sent); err != nil {
		return domain.Transfer{}, err
	}

	received := domain.Transaction{
		Type:                 domain.TxTransferIn,
		Amount:               in.Amount,
		BalanceAfter:         receiverBalanceAfter,
		CounterpartyWalletID: &in.FromWalletID,
		ReferenceID:          transfer.ID,
	}
	if err := tx.Transactions.Insert(ctx, in.ToWalletID, &received); err != nil {
		return domain.Transfer{}, err
	}

	return transfer, nil
}
