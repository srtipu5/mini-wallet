package dto

import "encoding/json"

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
