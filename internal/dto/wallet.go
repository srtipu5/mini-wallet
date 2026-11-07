// Package dto holds request bodies, service inputs and response shapes.
package dto

import (
	"encoding/json"
	"time"

	"github.com/srtipu5/mini-wallet/internal/domain"
)

// CreateWalletRequest is the body of POST /api/v1/wallets.
type CreateWalletRequest struct {
	UserID string `json:"user_id"`
	Phone  string `json:"phone"`
}

// AddMoneyRequest is the body of POST /api/v1/wallets/{wallet_id}/add-money.
// Amount stays raw so non-integer values can be rejected.
type AddMoneyRequest struct {
	Amount     json.RawMessage `json:"amount"`
	GatewayRef string          `json:"gateway_ref"`
}

// AddMoneyInput is hashed for idempotency checks; keep JSON tags stable.
type AddMoneyInput struct {
	WalletID   string `json:"wallet_id"`
	Amount     int64  `json:"amount"`
	GatewayRef string `json:"gateway_ref"`
}

// AddMoneyResult is stored for idempotent replays; keep JSON tags stable.
type AddMoneyResult struct {
	TransactionID int64     `json:"transaction_id"`
	WalletID      string    `json:"wallet_id"`
	Amount        int64     `json:"amount"`
	GatewayRef    string    `json:"gateway_ref"`
	BalanceAfter  int64     `json:"balance_after"`
	CreatedAt     time.Time `json:"created_at"`
}

// WalletPage is the response of GET /api/v1/wallets.
type WalletPage struct {
	Data       []domain.Wallet `json:"data"`
	Pagination Pagination      `json:"pagination"`
}
