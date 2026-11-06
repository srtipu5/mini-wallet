package application

import (
	"context"

	"github.com/srtipu5/mini-wallet/internal/domain"
	"github.com/srtipu5/mini-wallet/internal/repository"
)

type HistoryPage struct {
	Data       []domain.Transaction `json:"data"`
	Pagination Pagination           `json:"pagination"`
}

type Pagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

// ListTransactions returns a page of a wallet's history, newest first.
// The filter has already been validated by the HTTP layer.
func (s *Service) ListTransactions(ctx context.Context, f repository.HistoryFilter) (HistoryPage, error) {
	// 404 for an unknown wallet, instead of an empty list.
	wallet, err := s.GetWallet(ctx, f.WalletID)
	if err != nil {
		return HistoryPage{}, err
	}
	f.WalletID = wallet.ID

	items, total, err := repository.ListTransactions(ctx, s.db, f)
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
