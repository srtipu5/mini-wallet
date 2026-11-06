package application

import (
	"context"

	"github.com/srtipu5/mini-wallet/internal/domain"
)

// ListTransactions returns a page of a wallet's history, newest first.
// The filter has already been validated by the HTTP layer.
func (s *Service) ListTransactions(ctx context.Context, f domain.TransactionFilter) (HistoryPage, error) {
	// 404 for an unknown wallet, instead of an empty list.
	wallet, err := s.GetWallet(ctx, f.WalletID)
	if err != nil {
		return HistoryPage{}, err
	}
	f.WalletID = wallet.ID

	items, total, err := s.store.Repos().Transactions.List(ctx, f)
	if err != nil {
		return HistoryPage{}, err
	}
	if items == nil {
		items = []domain.Transaction{} // render [] instead of null
	}

	return HistoryPage{
		Data: items,
		Pagination: Pagination{
			Page:       f.Page,
			PageSize:   f.PageSize,
			TotalItems: total,
			TotalPages: (total + f.PageSize - 1) / f.PageSize,
		},
	}, nil
}
