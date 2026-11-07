package service

import (
	"context"

	"github.com/srtipu5/mini-wallet/internal/domain"
	"github.com/srtipu5/mini-wallet/internal/dto"
)

func (s *Service) ListTransactions(ctx context.Context, f domain.TransactionFilter) (dto.HistoryPage, error) {
	// Return 404 for an unknown wallet rather than an empty list.
	wallet, err := s.GetWallet(ctx, f.WalletID)
	if err != nil {
		return dto.HistoryPage{}, err
	}
	f.WalletID = wallet.ID

	items, total, err := s.store.Repos().Transactions.List(ctx, f)
	if err != nil {
		return dto.HistoryPage{}, err
	}
	if items == nil {
		items = []domain.Transaction{}
	}

	return dto.HistoryPage{
		Data:       items,
		Pagination: dto.NewPagination(f.Page, f.PageSize, total),
	}, nil
}
