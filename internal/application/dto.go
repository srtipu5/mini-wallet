package application

import "github.com/srtipu5/mini-wallet/internal/domain"

// Inputs and outputs of the use cases.
//
// The JSON tags on the inputs matter: the inputs are hashed to detect an
// Idempotency-Key reused with a different request.

type AddMoneyInput struct {
	WalletID   string `json:"wallet_id"`
	Amount     int64  `json:"amount"`
	GatewayRef string `json:"gateway_ref"`
}

type TransferInput struct {
	FromWalletID string  `json:"from_wallet_id"`
	ToWalletID   string  `json:"to_wallet_id"`
	Amount       int64   `json:"amount"`
	Note         *string `json:"note"`
}

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
