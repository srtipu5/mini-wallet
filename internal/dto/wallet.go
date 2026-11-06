// Package dto holds the data shapes that cross layer boundaries, one file per
// resource:
//
//   - *Request: HTTP request bodies, decoded by the handlers.
//   - *Input:   use-case inputs, passed from the handlers to the service.
//   - outputs:  use-case results that are not domain entities (e.g. HistoryPage).
//
// It depends only on the domain package.
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
// Amount is raw JSON so the handler can reject floats, exponents and strings
// (money is integer poisha only).
type AddMoneyRequest struct {
	Amount     json.RawMessage `json:"amount"`
	GatewayRef string          `json:"gateway_ref"`
}

// AddMoneyInput is the add-money use-case input.
//
// The JSON tags matter: the input is hashed to detect an Idempotency-Key
// reused with a different request. Changing them changes every hash.
type AddMoneyInput struct {
	WalletID   string `json:"wallet_id"`
	Amount     int64  `json:"amount"`
	GatewayRef string `json:"gateway_ref"`
}

// AddMoneyResult is the response of a successful add-money call.
// Keep the JSON tags stable: stored idempotent responses are decoded into it.
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
