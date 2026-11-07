package dto

import "encoding/json"

// TransferRequest is the body of POST /api/v1/transfers.
// Amount stays raw so non-integer values can be rejected.
type TransferRequest struct {
	FromWalletID string          `json:"from_wallet_id"`
	ToWalletID   string          `json:"to_wallet_id"`
	Amount       json.RawMessage `json:"amount"`
	Note         *string         `json:"note"`
}

// TransferInput is hashed for idempotency checks; keep JSON tags stable.
type TransferInput struct {
	FromWalletID string  `json:"from_wallet_id"`
	ToWalletID   string  `json:"to_wallet_id"`
	Amount       int64   `json:"amount"`
	Note         *string `json:"note"`
}
