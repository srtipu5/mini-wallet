package service

import (
	"context"
	"strings"

	"github.com/srtipu5/mini-wallet/internal/domain"
	"github.com/srtipu5/mini-wallet/internal/dto"
)

func (s *Service) CreateWallet(ctx context.Context, userID, phone string) (domain.Wallet, error) {
	if err := domain.ValidateText("user_id", userID, domain.MaxUserIDLength); err != nil {
		return domain.Wallet{}, err
	}
	if err := domain.ValidatePhone(phone); err != nil {
		return domain.Wallet{}, err
	}
	return s.store.Repos().Wallets.Create(ctx, userID, phone)
}

func (s *Service) GetWallet(ctx context.Context, walletID string) (domain.Wallet, error) {
	walletID = strings.ToLower(walletID)
	if !domain.IsValidWalletID(walletID) {
		return domain.Wallet{}, domain.ErrWalletNotFound
	}
	return s.store.Repos().Wallets.Get(ctx, walletID)
}

// ListWallets returns one page of all wallets, newest first. page and
// pageSize have already been validated by the HTTP layer.
func (s *Service) ListWallets(ctx context.Context, page, pageSize int) (dto.WalletPage, error) {
	wallets, total, err := s.store.Repos().Wallets.List(ctx, page, pageSize)
	if err != nil {
		return dto.WalletPage{}, err
	}
	if wallets == nil {
		wallets = []domain.Wallet{} // render [] instead of null
	}
	return dto.WalletPage{Data: wallets, Pagination: dto.NewPagination(page, pageSize, total)}, nil
}
