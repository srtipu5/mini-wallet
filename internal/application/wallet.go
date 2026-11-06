package application

import (
	"context"
	"strings"

	"github.com/srtipu5/mini-wallet/internal/domain"
	"github.com/srtipu5/mini-wallet/internal/repository"
)

func (s *Service) CreateWallet(ctx context.Context, userID, phone string) (domain.Wallet, error) {
	if err := domain.ValidateText("user_id", userID, domain.MaxUserIDLength); err != nil {
		return domain.Wallet{}, err
	}
	if err := domain.ValidatePhone(phone); err != nil {
		return domain.Wallet{}, err
	}
	return repository.CreateWallet(ctx, s.db, userID, phone)
}

func (s *Service) GetWallet(ctx context.Context, walletID string) (domain.Wallet, error) {
	walletID = strings.ToLower(walletID)
	if !domain.IsValidWalletID(walletID) {
		return domain.Wallet{}, domain.ErrWalletNotFound
	}
	return repository.GetWallet(ctx, s.db, walletID)
}
