package application

import (
	"context"
	"strings"

	"github.com/srtipu5/mini-wallet/internal/domain"
	"github.com/srtipu5/mini-wallet/internal/dto"
)

// Transfer moves money between two wallets. Debit and credit happen in one
// database transaction, so either both happen or neither does.
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
		func(r Repositories) (domain.Transfer, error) {
			return executeTransfer(ctx, r, in)
		})
}

func validateTransfer(in dto.TransferInput) error {
	if in.FromWalletID == "" || in.ToWalletID == "" {
		return domain.Validation("from_wallet_id and to_wallet_id are required")
	}
	if in.FromWalletID == in.ToWalletID {
		return domain.ErrSameWalletTransfer
	}
	if !domain.IsValidWalletID(in.FromWalletID) {
		return domain.NewError(domain.CodeWalletNotFound, "sender wallet not found")
	}
	if !domain.IsValidWalletID(in.ToWalletID) {
		return domain.NewError(domain.CodeWalletNotFound, "receiver wallet not found")
	}
	if err := domain.ValidateAmount(in.Amount); err != nil {
		return err
	}
	if in.Note != nil && len([]rune(*in.Note)) > domain.MaxNoteLength {
		return domain.Validation("note must be at most 255 characters")
	}
	return nil
}

// executeTransfer runs inside the transaction opened by runIdempotent.
func executeTransfer(ctx context.Context, r Repositories, in dto.TransferInput) (domain.Transfer, error) {
	// 1. Lock both wallets, ALWAYS in the same (sorted) order.
	//    If A->B and B->A run at the same time, both lock the smaller id first,
	//    so one simply waits for the other instead of deadlocking.
	first, second := in.FromWalletID, in.ToWalletID
	if second < first {
		first, second = second, first
	}
	balances := map[string]int64{}
	for _, id := range []string{first, second} {
		balance, err := r.Wallets.Lock(ctx, id)
		if err != nil {
			if id == in.FromWalletID {
				return domain.Transfer{}, notFound("sender", err)
			}
			return domain.Transfer{}, notFound("receiver", err)
		}
		balances[id] = balance
	}

	// 2. Check the balance. Safe because nobody else can change it while we hold the lock.
	if balances[in.FromWalletID] < in.Amount {
		return domain.Transfer{}, domain.ErrInsufficientBalance
	}

	// 3. Debit and credit.
	fromBalance, err := r.Wallets.ChangeBalance(ctx, in.FromWalletID, -in.Amount)
	if err != nil {
		return domain.Transfer{}, err
	}
	toBalance, err := r.Wallets.ChangeBalance(ctx, in.ToWalletID, in.Amount)
	if err != nil {
		return domain.Transfer{}, err
	}

	// 4. Record the transfer and one ledger entry per wallet.
	transfer, err := r.Transfers.Insert(ctx, in.FromWalletID, in.ToWalletID, in.Amount, in.Note)
	if err != nil {
		return domain.Transfer{}, err
	}

	out := domain.Transaction{
		Type: domain.TxTransferOut, Amount: in.Amount, BalanceAfter: fromBalance,
		CounterpartyWalletID: &in.ToWalletID, ReferenceID: transfer.ID,
	}
	if err := r.Transactions.Insert(ctx, in.FromWalletID, &out); err != nil {
		return domain.Transfer{}, err
	}

	inEntry := domain.Transaction{
		Type: domain.TxTransferIn, Amount: in.Amount, BalanceAfter: toBalance,
		CounterpartyWalletID: &in.FromWalletID, ReferenceID: transfer.ID,
	}
	if err := r.Transactions.Insert(ctx, in.ToWalletID, &inEntry); err != nil {
		return domain.Transfer{}, err
	}

	return transfer, nil
}

// notFound turns a generic "wallet not found" into "sender/receiver wallet not found".
func notFound(who string, err error) error {
	if e, ok := domain.AsError(err); ok && e.Code == domain.CodeWalletNotFound {
		return domain.NewError(domain.CodeWalletNotFound, who+" wallet not found")
	}
	return err
}
